package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"flag"
	"fmt"
	"net/mail"
	"os"
	"regexp"
	"strings"
	"time"

	sharedauth "github.com/duclamdev/application-chat/backend/internal/shared/auth"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var usernamePattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]{2,39}$`)

type accountOptions struct {
	displayName string
	email       string
	username    string
}

type accountResult struct {
	created  bool
	email    string
	password string
	username string
}

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := openDatabase(ctx)
	if err != nil {
		fail(err)
	}
	defer pool.Close()

	switch os.Args[1] {
	case "list":
		if len(os.Args) != 2 {
			usage()
			os.Exit(2)
		}
		if err := listAdmins(ctx, pool); err != nil {
			fail(err)
		}
	case "ensure":
		options, err := parseAccountFlags("ensure", os.Args[2:])
		if err != nil {
			fail(err)
		}
		result, err := ensureAdmin(ctx, pool, options)
		if err != nil {
			fail(err)
		}
		printAccountResult(result)
	case "reset":
		flags := flag.NewFlagSet("reset", flag.ContinueOnError)
		username := flags.String("username", "admin", "admin username")
		if err := flags.Parse(os.Args[2:]); err != nil || flags.NArg() != 0 {
			usage()
			os.Exit(2)
		}
		password, email, err := resetPassword(ctx, pool, strings.ToLower(strings.TrimSpace(*username)))
		if err != nil {
			fail(err)
		}
		printAccountResult(accountResult{email: email, password: password, username: *username})
	default:
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, `Cách dùng:
  webtui-admin-account list
  webtui-admin-account ensure [--username admin] [--email admin@example.com] [--display-name "Quản trị viên"]
  webtui-admin-account reset [--username admin]`)
}

func openDatabase(ctx context.Context) (*pgxpool.Pool, error) {
	databaseURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if databaseURL == "" {
		return nil, errors.New("DATABASE_URL chưa được cấu hình")
	}
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("không thể mở kết nối dữ liệu: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("không thể kết nối dữ liệu: %w", err)
	}
	return pool, nil
}

func parseAccountFlags(command string, arguments []string) (accountOptions, error) {
	flags := flag.NewFlagSet(command, flag.ContinueOnError)
	defaultDomain := strings.TrimSpace(os.Getenv("INSTANCE_DOMAIN"))
	if defaultDomain == "" {
		defaultDomain = "localhost"
	}
	username := flags.String("username", "admin", "admin username")
	email := flags.String("email", "admin@"+defaultDomain, "admin email")
	displayName := flags.String("display-name", "Quản trị viên", "admin display name")
	if err := flags.Parse(arguments); err != nil {
		return accountOptions{}, err
	}
	if flags.NArg() != 0 {
		return accountOptions{}, errors.New("tham số tài khoản quản trị không hợp lệ")
	}

	options := accountOptions{
		displayName: strings.TrimSpace(*displayName),
		email:       strings.ToLower(strings.TrimSpace(*email)),
		username:    strings.ToLower(strings.TrimSpace(*username)),
	}
	if !usernamePattern.MatchString(options.username) {
		return accountOptions{}, errors.New("tên đăng nhập phải có 3-40 ký tự và chỉ gồm chữ, số, dấu chấm, gạch ngang hoặc gạch dưới")
	}
	parsedEmail, err := mail.ParseAddress(options.email)
	if err != nil || !strings.EqualFold(parsedEmail.Address, options.email) {
		return accountOptions{}, errors.New("email quản trị không hợp lệ")
	}
	if options.displayName == "" || len([]rune(options.displayName)) > 120 {
		return accountOptions{}, errors.New("tên hiển thị phải có từ 1 đến 120 ký tự")
	}
	return options, nil
}

func selfHostedWorkspaceID(ctx context.Context, tx pgx.Tx) (string, error) {
	var workspaceID string
	err := tx.QueryRow(ctx, `
SELECT workspace.id::text
FROM zones zone
JOIN workspaces workspace
  ON workspace.id = zone.primary_workspace_id
 AND workspace.zone_id = zone.id
 AND workspace.deleted_at IS NULL
WHERE zone.deleted_at IS NULL
  AND zone.metadata->>'deployment_model' = 'self_hosted'
ORDER BY zone.created_at
LIMIT 1
`).Scan(&workspaceID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", errors.New("chưa tìm thấy workspace self-host; hãy chạy migration và khởi động API trước")
	}
	return workspaceID, err
}

func ensureAdmin(ctx context.Context, pool *pgxpool.Pool, options accountOptions) (accountResult, error) {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return accountResult{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	workspaceID, err := selfHostedWorkspaceID(ctx, tx)
	if err != nil {
		return accountResult{}, err
	}
	rows, err := tx.Query(ctx, `
SELECT id::text, username::text, email::text
FROM users
WHERE deleted_at IS NULL
  AND (username = $1 OR email = $2)
FOR UPDATE
`, options.username, options.email)
	if err != nil {
		return accountResult{}, err
	}
	type existingUser struct{ id, username, email string }
	users := make([]existingUser, 0, 2)
	for rows.Next() {
		var user existingUser
		if err := rows.Scan(&user.id, &user.username, &user.email); err != nil {
			rows.Close()
			return accountResult{}, err
		}
		users = append(users, user)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return accountResult{}, err
	}
	if len(users) > 1 {
		return accountResult{}, errors.New("username và email đang thuộc hai tài khoản khác nhau; hãy chọn thông tin khác")
	}

	result := accountResult{email: options.email, username: options.username}
	var userID string
	if len(users) == 1 {
		userID = users[0].id
		result.email = users[0].email
		result.username = users[0].username
		if _, err := tx.Exec(ctx, `UPDATE users SET status = 'active' WHERE id = $1::uuid`, userID); err != nil {
			return accountResult{}, err
		}
	} else {
		password, err := generatePassword()
		if err != nil {
			return accountResult{}, err
		}
		hash, err := sharedauth.HashPassword(password)
		if err != nil {
			return accountResult{}, err
		}
		if err := tx.QueryRow(ctx, `
INSERT INTO users (email, username, display_name, password_hash, status, email_verified_at, last_seen_at)
VALUES ($1, $2, $3, $4, 'active', now(), now())
RETURNING id::text
`, options.email, options.username, options.displayName, hash).Scan(&userID); err != nil {
			return accountResult{}, err
		}
		result.created = true
		result.password = password
	}

	if err := grantOwner(ctx, tx, workspaceID, userID); err != nil {
		return accountResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return accountResult{}, err
	}
	return result, nil
}

func grantOwner(ctx context.Context, tx pgx.Tx, workspaceID string, userID string) error {
	if _, err := tx.Exec(ctx, `
INSERT INTO workspace_members (workspace_id, user_id, status, joined_at)
VALUES ($1::uuid, $2::uuid, 'active', now())
ON CONFLICT (workspace_id, user_id) DO UPDATE
SET status = 'active', joined_at = COALESCE(workspace_members.joined_at, EXCLUDED.joined_at)
`, workspaceID, userID); err != nil {
		return err
	}
	var ownerRoleID string
	if err := tx.QueryRow(ctx, `
SELECT id::text
FROM roles
WHERE workspace_id IS NULL
  AND code = 'workspace_owner'
  AND deleted_at IS NULL
ORDER BY created_at
LIMIT 1
`).Scan(&ownerRoleID); errors.Is(err, pgx.ErrNoRows) {
		return errors.New("chưa có vai trò workspace_owner; hãy chạy migration trước")
	} else if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
INSERT INTO workspace_member_roles (workspace_id, user_id, role_id, assigned_by)
VALUES ($1::uuid, $2::uuid, $3::uuid, $2::uuid)
ON CONFLICT (workspace_id, user_id, role_id) DO NOTHING
`, workspaceID, userID, ownerRoleID); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, `
UPDATE workspaces
SET owner_id = COALESCE(owner_id, $2::uuid)
WHERE id = $1::uuid
`, workspaceID, userID)
	return err
}

func resetPassword(ctx context.Context, pool *pgxpool.Pool, username string) (string, string, error) {
	if !usernamePattern.MatchString(username) {
		return "", "", errors.New("tên đăng nhập quản trị không hợp lệ")
	}
	password, err := generatePassword()
	if err != nil {
		return "", "", err
	}
	hash, err := sharedauth.HashPassword(password)
	if err != nil {
		return "", "", err
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return "", "", err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	workspaceID, err := selfHostedWorkspaceID(ctx, tx)
	if err != nil {
		return "", "", err
	}
	var userID string
	var email string
	err = tx.QueryRow(ctx, `
UPDATE users
SET password_hash = $2, status = 'active'
WHERE username = $1 AND deleted_at IS NULL
RETURNING id::text, email::text
`, username, hash).Scan(&userID, &email)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", "", fmt.Errorf("không tìm thấy tài khoản %q; hãy chạy lệnh create trước", username)
	}
	if err != nil {
		return "", "", err
	}
	if _, err := tx.Exec(ctx, `
UPDATE user_sessions
SET revoked_at = COALESCE(revoked_at, now())
WHERE user_id = $1::uuid
`, userID); err != nil {
		return "", "", err
	}
	if err := grantOwner(ctx, tx, workspaceID, userID); err != nil {
		return "", "", err
	}
	if err := tx.Commit(ctx); err != nil {
		return "", "", err
	}
	return password, email, nil
}

func listAdmins(ctx context.Context, pool *pgxpool.Pool) error {
	rows, err := pool.Query(ctx, `
SELECT DISTINCT user_account.username::text, user_account.email::text, user_account.status
FROM zones zone
JOIN workspaces workspace ON workspace.id = zone.primary_workspace_id
JOIN workspace_member_roles member_role ON member_role.workspace_id = workspace.id
JOIN users user_account ON user_account.id = member_role.user_id AND user_account.deleted_at IS NULL
JOIN role_permissions role_permission ON role_permission.role_id = member_role.role_id
JOIN permissions permission ON permission.id = role_permission.permission_id AND permission.code = 'admin.view'
WHERE zone.deleted_at IS NULL
  AND zone.metadata->>'deployment_model' = 'self_hosted'
ORDER BY user_account.username
`)
	if err != nil {
		return err
	}
	defer rows.Close()
	count := 0
	for rows.Next() {
		var username, email, status string
		if err := rows.Scan(&username, &email, &status); err != nil {
			return err
		}
		fmt.Printf("- %s (%s) — %s\n", username, email, status)
		count++
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if count == 0 {
		fmt.Println("Chưa có tài khoản có quyền quản trị.")
	}
	return nil
}

func generatePassword() (string, error) {
	value := make([]byte, 24)
	if _, err := rand.Read(value); err != nil {
		return "", fmt.Errorf("không thể tạo mật khẩu an toàn: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}

func printAccountResult(result accountResult) {
	if result.created {
		fmt.Println("Đã tạo tài khoản quản trị mới.")
	} else if result.password == "" {
		fmt.Println("Tài khoản đã tồn tại và đã được bảo đảm quyền quản trị. Mật khẩu hiện tại không bị thay đổi.")
	} else {
		fmt.Println("Đã cấp lại mật khẩu và đăng xuất các phiên cũ.")
	}
	fmt.Printf("Tên đăng nhập: %s\n", result.username)
	fmt.Printf("Email: %s\n", result.email)
	if result.password != "" {
		fmt.Printf("Mật khẩu mới: %s\n", result.password)
		fmt.Println("Hãy lưu mật khẩu này ngay; hệ thống sẽ không hiển thị lại.")
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "Lỗi:", err)
	os.Exit(1)
}
