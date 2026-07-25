package postgres

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net"
	"strings"
	"time"

	authapp "github.com/duclamdev/application-chat/backend/internal/modules/auth/application"
	authdomain "github.com/duclamdev/application-chat/backend/internal/modules/auth/domain"
	apperrors "github.com/duclamdev/application-chat/backend/internal/shared/errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	pool               *pgxpool.Pool
	defaultWorkspaceID string
}

func NewRepository(pool *pgxpool.Pool, defaultWorkspaceID ...string) *Repository {
	workspaceID := ""
	if len(defaultWorkspaceID) > 0 {
		workspaceID = strings.TrimSpace(defaultWorkspaceID[0])
	}
	return &Repository{pool: pool, defaultWorkspaceID: workspaceID}
}

func (r *Repository) CreateUser(ctx context.Context, params authapp.CreateUserParams) (authdomain.User, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return authdomain.User{}, err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	row := tx.QueryRow(ctx, `
INSERT INTO users (
    email,
    username,
    display_name,
    password_hash,
    status,
    registration_ip_address,
    registration_device_name,
    registration_user_agent,
    last_ip_address,
    device_name,
    last_user_agent,
    last_seen_at,
    avatar_url,
    email_verified_at
)
VALUES (
    $1,
    $2,
    $3,
    $4,
    'active',
    NULLIF($5, '')::inet,
    NULLIF($6, ''),
    NULLIF($7, ''),
    NULLIF($5, '')::inet,
    NULLIF($6, ''),
    NULLIF($7, ''),
    now(),
    NULLIF($8, ''),
    CASE WHEN $9 THEN now() ELSE NULL END
)
RETURNING id::text, email::text, username::text, display_name, password_hash, avatar_url, status,
          locale, timezone, email_verified_at, last_seen_at,
          host(registration_ip_address), registration_device_name,
          host(last_ip_address), device_name,
          created_at, updated_at
`, params.Email, params.Username, params.DisplayName, params.PasswordHash, params.IPAddress, params.DeviceName, params.UserAgent, params.AvatarURL, params.EmailVerified)

	user, err := scanUser(row)
	if err != nil {
		if isUniqueViolation(err) {
			return authdomain.User{}, authdomain.ErrUserAlreadyExists
		}
		return authdomain.User{}, err
	}
	if err := r.provisionRegistrationAccess(ctx, tx, user.ID, params.Email, params.InviteToken, params.Zone); err != nil {
		return authdomain.User{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return authdomain.User{}, err
	}
	return user, nil
}

func (r *Repository) ResolveZoneAccess(ctx context.Context, domain string) (authapp.ZoneAccess, error) {
	target, err := r.resolveZoneAccessByDomain(ctx, domain)
	if err == nil {
		return target, nil
	}
	if !errors.Is(err, authdomain.ErrZoneNotFound) || (domain != "localhost" && net.ParseIP(domain) == nil) {
		return authapp.ZoneAccess{}, err
	}
	return r.resolveLocalDevelopmentZone(ctx)
}

func (r *Repository) resolveZoneAccessByDomain(ctx context.Context, domain string) (authapp.ZoneAccess, error) {
	var target authapp.ZoneAccess
	err := r.pool.QueryRow(ctx, `
SELECT
    zone.id::text,
    zone.slug::text,
    zone.name,
    zone.kind,
    zone.status,
    zone.registration_mode,
    workspace.id::text,
    workspace.slug::text,
    domain.domain::text
FROM zone_domains domain
JOIN zones zone
  ON zone.id = domain.zone_id
 AND zone.status IN ('active', 'suspended')
 AND zone.deleted_at IS NULL
JOIN workspaces workspace
  ON workspace.id = zone.primary_workspace_id
 AND workspace.zone_id = zone.id
 AND workspace.status IN ('active', 'suspended')
 AND workspace.deleted_at IS NULL
WHERE domain.domain = $1
  AND domain.status IN ('verified', 'active')
  AND domain.deleted_at IS NULL
ORDER BY
    CASE domain.kind WHEN 'primary' THEN 0 ELSE 1 END,
    CASE domain.status WHEN 'active' THEN 0 ELSE 1 END
LIMIT 1
`, domain).Scan(
		&target.ZoneID,
		&target.ZoneSlug,
		&target.ZoneName,
		&target.ZoneKind,
		&target.ZoneStatus,
		&target.RegistrationMode,
		&target.WorkspaceID,
		&target.WorkspaceSlug,
		&target.Domain,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return authapp.ZoneAccess{}, authdomain.ErrZoneNotFound
	}
	return target, err
}

func (r *Repository) resolveLocalDevelopmentZone(ctx context.Context) (authapp.ZoneAccess, error) {
	var target authapp.ZoneAccess
	query := `
SELECT
    zone.id::text,
    zone.slug::text,
    zone.name,
    zone.kind,
    zone.status,
    zone.registration_mode,
    workspace.id::text,
    workspace.slug::text,
    domain.domain::text
FROM zones zone
JOIN workspaces workspace
  ON workspace.id = zone.primary_workspace_id
 AND workspace.zone_id = zone.id
 AND workspace.status = 'active'
 AND workspace.deleted_at IS NULL
JOIN LATERAL (
    SELECT zone_domain.domain
    FROM zone_domains zone_domain
    WHERE zone_domain.zone_id = zone.id
      AND zone_domain.status IN ('verified', 'active')
      AND zone_domain.deleted_at IS NULL
    ORDER BY
        CASE zone_domain.kind WHEN 'primary' THEN 0 ELSE 1 END,
        CASE zone_domain.status WHEN 'active' THEN 0 ELSE 1 END
    LIMIT 1
) domain ON true
WHERE zone.status = 'active'
  AND zone.deleted_at IS NULL
`
	args := []any{}
	if r.defaultWorkspaceID != "" {
		query += " AND workspace.id = $1::uuid"
		args = append(args, r.defaultWorkspaceID)
	}
	query += `
 ORDER BY
    CASE zone.kind WHEN 'vpsttt_internal' THEN 0 ELSE 1 END,
    zone.created_at,
    zone.id
 LIMIT 2`

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return authapp.ZoneAccess{}, err
	}
	defer rows.Close()
	targets := make([]authapp.ZoneAccess, 0, 2)
	for rows.Next() {
		if err := rows.Scan(
			&target.ZoneID,
			&target.ZoneSlug,
			&target.ZoneName,
			&target.ZoneKind,
			&target.ZoneStatus,
			&target.RegistrationMode,
			&target.WorkspaceID,
			&target.WorkspaceSlug,
			&target.Domain,
		); err != nil {
			return authapp.ZoneAccess{}, err
		}
		targets = append(targets, target)
	}
	if err := rows.Err(); err != nil {
		return authapp.ZoneAccess{}, err
	}
	if r.defaultWorkspaceID == "" {
		internalTargets := make([]authapp.ZoneAccess, 0, 1)
		for _, candidate := range targets {
			if candidate.ZoneKind == "vpsttt_internal" {
				internalTargets = append(internalTargets, candidate)
			}
		}
		if len(internalTargets) == 1 {
			return internalTargets[0], nil
		}
	}
	if len(targets) != 1 {
		return authapp.ZoneAccess{}, authdomain.ErrZoneNotFound
	}
	return targets[0], nil
}

func (r *Repository) EnsureZoneWorkspaceAccess(ctx context.Context, userID string, target authapp.ZoneAccess) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()
	if err := r.ensureZoneWorkspaceAccess(ctx, tx, userID, target); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *Repository) provisionRegistrationAccess(
	ctx context.Context,
	tx pgx.Tx,
	userID string,
	email string,
	inviteToken string,
	target authapp.ZoneAccess,
) error {
	currentTarget, err := lockRegistrationTarget(ctx, tx, target)
	if err != nil {
		return err
	}
	target.ZoneKind = currentTarget.ZoneKind
	target.ZoneStatus = currentTarget.ZoneStatus
	target.RegistrationMode = currentTarget.RegistrationMode

	var inviteID string
	var inviteRoleID string
	switch target.RegistrationMode {
	case "open":
	case "invite_only":
		inviteToken = strings.TrimSpace(inviteToken)
		if inviteToken == "" {
			return authdomain.ErrInviteRequired
		}
		hash := sha256.Sum256([]byte(inviteToken))
		err := tx.QueryRow(ctx, `
SELECT id::text, role_id::text
FROM workspace_invites
WHERE workspace_id = $1::uuid
  AND email = $2
  AND token_hash = $3
  AND accepted_at IS NULL
  AND revoked_at IS NULL
  AND expires_at > now()
ORDER BY created_at DESC
LIMIT 1
FOR UPDATE
`, target.WorkspaceID, strings.ToLower(strings.TrimSpace(email)), hex.EncodeToString(hash[:])).Scan(
			&inviteID,
			&inviteRoleID,
		)
		if errors.Is(err, pgx.ErrNoRows) {
			return authdomain.ErrInviteRequired
		}
		if err != nil {
			return err
		}
	case "closed":
		return authdomain.ErrRegistrationClosed
	default:
		return authdomain.ErrRegistrationClosed
	}

	if err := addWorkspaceMembership(ctx, tx, userID, target.WorkspaceID, inviteRoleID); err != nil {
		return err
	}
	if target.ZoneKind == "vpsttt_internal" || target.ZoneKind == "customer_dedicated" {
		claimed, err := claimInitialWorkspaceOwnership(ctx, tx, userID, target.WorkspaceID)
		if err != nil {
			return err
		}
		if claimed && target.ZoneKind == "customer_dedicated" {
			if _, err := tx.Exec(ctx, `
UPDATE zones
SET registration_mode = 'invite_only'
WHERE id = $1::uuid
  AND registration_mode = 'open'
  AND deleted_at IS NULL
`, target.ZoneID); err != nil {
				return err
			}
		}
	}
	if inviteID != "" {
		if _, err := tx.Exec(ctx, `
UPDATE workspace_invites
SET accepted_at = now()
WHERE id = $1::uuid
  AND accepted_at IS NULL
`, inviteID); err != nil {
			return err
		}
	}
	return nil
}

func lockRegistrationTarget(
	ctx context.Context,
	tx pgx.Tx,
	target authapp.ZoneAccess,
) (authapp.ZoneAccess, error) {
	err := tx.QueryRow(ctx, `
SELECT zone.kind, zone.status, zone.registration_mode
FROM zones zone
JOIN workspaces workspace
  ON workspace.id = $2::uuid
 AND workspace.zone_id = zone.id
 AND workspace.status = 'active'
 AND workspace.deleted_at IS NULL
WHERE zone.id = $1::uuid
  AND zone.status = 'active'
  AND zone.deleted_at IS NULL
FOR UPDATE OF zone, workspace
`, target.ZoneID, target.WorkspaceID).Scan(
		&target.ZoneKind,
		&target.ZoneStatus,
		&target.RegistrationMode,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return authapp.ZoneAccess{}, authdomain.ErrRegistrationClosed
	}
	if err != nil {
		return authapp.ZoneAccess{}, err
	}
	return target, nil
}

func claimInitialWorkspaceOwnership(
	ctx context.Context,
	tx pgx.Tx,
	userID string,
	workspaceID string,
) (bool, error) {
	tag, err := tx.Exec(ctx, `
UPDATE workspaces
SET owner_id = $2::uuid
WHERE id = $1::uuid
  AND owner_id IS NULL
  AND deleted_at IS NULL
`, workspaceID, userID)
	if err != nil {
		return false, err
	}
	if tag.RowsAffected() == 0 {
		return false, nil
	}

	if _, err = tx.Exec(ctx, `
INSERT INTO workspace_member_roles (workspace_id, user_id, role_id, assigned_by)
SELECT $1::uuid, $2::uuid, role.id, $2::uuid
FROM roles role
WHERE role.workspace_id IS NULL
  AND role.code = 'workspace_owner'
  AND role.is_system = true
  AND role.deleted_at IS NULL
ORDER BY role.created_at
LIMIT 1
ON CONFLICT (workspace_id, user_id, role_id) DO NOTHING
`, workspaceID, userID); err != nil {
		return false, err
	}

	var hasOwnerRole bool
	if err := tx.QueryRow(ctx, `
SELECT EXISTS (
    SELECT 1
    FROM workspace_member_roles member_role
    JOIN roles role
      ON role.id = member_role.role_id
     AND role.code = 'workspace_owner'
     AND role.deleted_at IS NULL
    WHERE member_role.workspace_id = $1::uuid
      AND member_role.user_id = $2::uuid
)
`, workspaceID, userID).Scan(&hasOwnerRole); err != nil {
		return false, err
	}
	if !hasOwnerRole {
		return false, defaultWorkspaceUnavailable("workspace_owner_role_unavailable")
	}
	return true, nil
}

func (r *Repository) ensureZoneWorkspaceAccess(
	ctx context.Context,
	tx pgx.Tx,
	userID string,
	target authapp.ZoneAccess,
) error {
	if err := validateZoneTarget(ctx, tx, target); err != nil {
		return err
	}

	var membershipStatus string
	err := tx.QueryRow(ctx, `
SELECT status
FROM workspace_members
WHERE workspace_id = $1::uuid
  AND user_id = $2::uuid
FOR UPDATE
`, target.WorkspaceID, userID).Scan(&membershipStatus)
	if errors.Is(err, pgx.ErrNoRows) {
		if target.ZoneStatus != "active" || target.RegistrationMode != "open" {
			return authdomain.ErrZoneAccessDenied
		}
		return addWorkspaceMembership(ctx, tx, userID, target.WorkspaceID, "")
	}
	if err != nil {
		return err
	}
	if membershipStatus != "active" {
		return authdomain.ErrZoneAccessDenied
	}
	return repairWorkspaceAccess(ctx, tx, userID, target.WorkspaceID)
}

func validateZoneTarget(ctx context.Context, tx pgx.Tx, target authapp.ZoneAccess) error {
	var valid bool
	err := tx.QueryRow(ctx, `
SELECT EXISTS (
    SELECT 1
    FROM zones zone
    JOIN workspaces workspace
     ON workspace.id = $2::uuid
     AND workspace.zone_id = zone.id
     AND workspace.status = CASE
         WHEN zone.status = 'suspended' THEN 'suspended'
         ELSE 'active'
     END
     AND workspace.deleted_at IS NULL
    WHERE zone.id = $1::uuid
      AND zone.status IN ('active', 'suspended')
      AND zone.deleted_at IS NULL
)
`, target.ZoneID, target.WorkspaceID).Scan(&valid)
	if err != nil {
		return err
	}
	if !valid {
		return authdomain.ErrZoneNotFound
	}
	return nil
}

func addWorkspaceMembership(
	ctx context.Context,
	tx pgx.Tx,
	userID string,
	workspaceID string,
	inviteRoleID string,
) error {
	if _, err := tx.Exec(ctx, `
INSERT INTO workspace_members (workspace_id, user_id, status, joined_at)
VALUES ($1::uuid, $2::uuid, 'active', now())
ON CONFLICT (workspace_id, user_id)
DO UPDATE SET status = 'active', joined_at = COALESCE(workspace_members.joined_at, now())
WHERE workspace_members.status IN ('invited', 'left')
`, workspaceID, userID); err != nil {
		return err
	}
	var membershipActive bool
	if err := tx.QueryRow(ctx, `
SELECT EXISTS (
    SELECT 1
    FROM workspace_members
    WHERE workspace_id = $1::uuid
      AND user_id = $2::uuid
      AND status = 'active'
)
`, workspaceID, userID).Scan(&membershipActive); err != nil {
		return err
	}
	if !membershipActive {
		return authdomain.ErrZoneAccessDenied
	}

	if inviteRoleID != "" {
		if _, err := tx.Exec(ctx, `
INSERT INTO workspace_member_roles (workspace_id, user_id, role_id, assigned_by)
VALUES ($1::uuid, $2::uuid, $3::uuid, NULL)
ON CONFLICT (workspace_id, user_id, role_id) DO NOTHING
`, workspaceID, userID, inviteRoleID); err != nil {
			return err
		}
	}
	return repairWorkspaceAccess(ctx, tx, userID, workspaceID)
}

func repairWorkspaceAccess(ctx context.Context, tx pgx.Tx, userID string, workspaceID string) error {
	if _, err := tx.Exec(ctx, `
INSERT INTO workspace_member_roles (workspace_id, user_id, role_id, assigned_by)
SELECT $1::uuid, $2::uuid, role.id, NULL
FROM roles role
WHERE role.code = 'workspace_member'
  AND role.workspace_id IS NULL
  AND role.is_system = true
  AND role.deleted_at IS NULL
  AND NOT EXISTS (
      SELECT 1
      FROM workspace_member_roles existing
      WHERE existing.workspace_id = $1::uuid
        AND existing.user_id = $2::uuid
  )
ORDER BY role.created_at
LIMIT 1
ON CONFLICT (workspace_id, user_id, role_id) DO NOTHING
`, workspaceID, userID); err != nil {
		return err
	}

	var hasRole bool
	if err := tx.QueryRow(ctx, `
SELECT EXISTS (
    SELECT 1
    FROM workspace_member_roles member_role
    JOIN roles role
      ON role.id = member_role.role_id
     AND role.deleted_at IS NULL
    WHERE member_role.workspace_id = $1::uuid
      AND member_role.user_id = $2::uuid
)
`, workspaceID, userID).Scan(&hasRole); err != nil {
		return err
	}
	if !hasRole {
		return defaultWorkspaceUnavailable("default_member_role_unavailable")
	}

	_, err := tx.Exec(ctx, `
INSERT INTO channel_members (channel_id, user_id, status)
SELECT channel.id, $2::uuid, 'active'
FROM channels channel
WHERE channel.workspace_id = $1::uuid
  AND channel.type = 'public'
  AND channel.status = 'active'
  AND channel.deleted_at IS NULL
  AND COALESCE(channel.settings->>'bot_session_mode', '') <> 'private'
ON CONFLICT (channel_id, user_id) DO NOTHING
`, workspaceID, userID)
	return err
}

// EnsureDefaultWorkspaceMembership provisions the baseline access required by
// the chat UI. It is idempotent so existing accounts are repaired on login.
// A configured workspace is preferred; a single active workspace is treated
// as the implicit default for self-hosted/single-tenant installations.
func (r *Repository) EnsureDefaultWorkspaceMembership(ctx context.Context, userID string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	if err := r.ensureDefaultWorkspaceMembership(ctx, tx, userID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *Repository) ensureDefaultWorkspaceMembership(ctx context.Context, tx pgx.Tx, userID string) error {
	workspaceID, err := r.resolveDefaultWorkspaceID(ctx, tx)
	if err != nil {
		return err
	}

	if _, err := tx.Exec(ctx, `
INSERT INTO workspace_members (workspace_id, user_id, status, joined_at)
SELECT $1::uuid, u.id, 'active', now()
FROM users u
WHERE u.id = $2::uuid AND u.status = 'active' AND u.deleted_at IS NULL
ON CONFLICT (workspace_id, user_id)
DO UPDATE SET status = 'active', joined_at = COALESCE(workspace_members.joined_at, now())
WHERE workspace_members.status = 'invited'
`, workspaceID, userID); err != nil {
		return err
	}
	var activeMember bool
	if err := tx.QueryRow(ctx, `
SELECT EXISTS (
    SELECT 1
    FROM workspace_members
    WHERE workspace_id = $1::uuid AND user_id = $2::uuid AND status = 'active'
)
`, workspaceID, userID).Scan(&activeMember); err != nil {
		return err
	}
	if !activeMember {
		return apperrors.New(
			"WORKSPACE_ACCESS_DISABLED",
			"Quyền truy cập workspace của tài khoản đã bị vô hiệu hóa.",
			403,
		)
	}

	// Do not overwrite or broaden an existing owner/admin role. Accounts with
	// no role receive only the baseline workspace_member role.
	if _, err := tx.Exec(ctx, `
INSERT INTO workspace_member_roles (workspace_id, user_id, role_id, assigned_by)
SELECT $1::uuid, $2::uuid, r.id, NULL
FROM roles r
WHERE r.code = 'workspace_member'
  AND r.workspace_id IS NULL
  AND r.is_system = true
  AND r.deleted_at IS NULL
  AND NOT EXISTS (
      SELECT 1
      FROM workspace_member_roles existing
      WHERE existing.workspace_id = $1::uuid AND existing.user_id = $2::uuid
  )
ORDER BY r.created_at
LIMIT 1
ON CONFLICT (workspace_id, user_id, role_id) DO NOTHING
`, workspaceID, userID); err != nil {
		return err
	}

	var hasRole bool
	if err := tx.QueryRow(ctx, `
SELECT EXISTS (
    SELECT 1
    FROM workspace_member_roles wmr
    JOIN roles r ON r.id = wmr.role_id AND r.deleted_at IS NULL
    WHERE wmr.workspace_id = $1::uuid AND wmr.user_id = $2::uuid
)
`, workspaceID, userID).Scan(&hasRole); err != nil {
		return err
	}
	if !hasRole {
		return defaultWorkspaceUnavailable("default_member_role_unavailable")
	}

	_, err = tx.Exec(ctx, `
INSERT INTO channel_members (channel_id, user_id, status)
SELECT c.id, $2::uuid, 'active'
FROM channels c
WHERE c.workspace_id = $1::uuid
  AND c.type = 'public'
  AND c.status = 'active'
  AND c.deleted_at IS NULL
  AND COALESCE(c.settings->>'bot_session_mode', '') <> 'private'
ON CONFLICT (channel_id, user_id) DO NOTHING
`, workspaceID, userID)
	return err
}

func (r *Repository) resolveDefaultWorkspaceID(ctx context.Context, tx pgx.Tx) (string, error) {
	if r.defaultWorkspaceID != "" {
		var workspaceID string
		err := tx.QueryRow(ctx, `
SELECT id::text
FROM workspaces
WHERE id = $1::uuid AND status = 'active' AND deleted_at IS NULL
`, r.defaultWorkspaceID).Scan(&workspaceID)
		if errors.Is(err, pgx.ErrNoRows) {
			return "", defaultWorkspaceUnavailable("configured_workspace_unavailable")
		}
		return workspaceID, err
	}

	rows, err := tx.Query(ctx, `
SELECT id::text
FROM workspaces
WHERE status = 'active' AND deleted_at IS NULL
ORDER BY created_at, id
LIMIT 2
`)
	if err != nil {
		return "", err
	}
	defer rows.Close()

	workspaceIDs := make([]string, 0, 2)
	for rows.Next() {
		var workspaceID string
		if err := rows.Scan(&workspaceID); err != nil {
			return "", err
		}
		workspaceIDs = append(workspaceIDs, workspaceID)
	}
	if err := rows.Err(); err != nil {
		return "", err
	}
	return resolveImplicitWorkspaceID(workspaceIDs)
}

func resolveImplicitWorkspaceID(workspaceIDs []string) (string, error) {
	switch len(workspaceIDs) {
	case 1:
		return workspaceIDs[0], nil
	case 0:
		return "", defaultWorkspaceUnavailable("no_active_workspace")
	default:
		return "", defaultWorkspaceUnavailable("ambiguous_active_workspaces")
	}
}

func defaultWorkspaceUnavailable(reason string) *apperrors.AppError {
	err := apperrors.New(
		"DEFAULT_WORKSPACE_UNAVAILABLE",
		"Workspace mặc định chưa sẵn sàng. Vui lòng liên hệ quản trị viên.",
		503,
	)
	err.Details = map[string]any{"reason": reason}
	return err
}

func (r *Repository) FindUserByID(ctx context.Context, id string) (authdomain.User, error) {
	row := r.pool.QueryRow(ctx, `
SELECT id::text, email::text, username::text, display_name, password_hash, avatar_url, status,
       locale, timezone, email_verified_at, last_seen_at,
       host(registration_ip_address), registration_device_name,
       host(last_ip_address), device_name,
       created_at, updated_at
FROM users
WHERE id = $1::uuid AND deleted_at IS NULL
`, id)
	return scanUser(row)
}

func (r *Repository) FindUserByIdentifier(ctx context.Context, identifier string) (authdomain.User, error) {
	row := r.pool.QueryRow(ctx, `
SELECT id::text, email::text, username::text, display_name, password_hash, avatar_url, status,
       locale, timezone, email_verified_at, last_seen_at,
       host(registration_ip_address), registration_device_name,
       host(last_ip_address), device_name,
       created_at, updated_at
FROM users
WHERE (email = $1 OR username = $1) AND deleted_at IS NULL
`, identifier)
	return scanUser(row)
}

func (r *Repository) UpdateLastLoginInfo(ctx context.Context, params authapp.UpdateLastLoginInfoParams) error {
	_, err := r.pool.Exec(ctx, `
UPDATE users
SET last_seen_at = $2,
    last_ip_address = NULLIF($3, '')::inet,
    device_name = NULLIF($4, ''),
    last_user_agent = NULLIF($5, '')
WHERE id = $1::uuid AND deleted_at IS NULL
`, params.UserID, params.SeenAt, params.IPAddress, params.DeviceName, params.UserAgent)
	return err
}

func (r *Repository) CreateSession(ctx context.Context, params authapp.CreateSessionParams) (authdomain.Session, error) {
	row := r.pool.QueryRow(ctx, `
INSERT INTO user_sessions (
    user_id, zone_id, workspace_id, domain, refresh_token_hash,
    device_name, ip_address, user_agent, expires_at
)
VALUES (
    $1::uuid, $2::uuid, $3::uuid, $4, $5,
    NULLIF($6, ''), NULLIF($7, '')::inet, NULLIF($8, ''), $9
)
RETURNING id::text, user_id::text, zone_id::text, workspace_id::text, domain::text,
          refresh_token_hash, device_name, host(ip_address), user_agent,
          expires_at, revoked_at, created_at, updated_at
`, params.UserID, params.ZoneID, params.WorkspaceID, params.Domain, params.RefreshTokenHash,
		params.DeviceName, params.IPAddress, params.UserAgent, params.ExpiresAt)
	return scanSession(row)
}

func (r *Repository) FindSessionByRefreshTokenHash(ctx context.Context, hash string) (authdomain.Session, error) {
	row := r.pool.QueryRow(ctx, `
SELECT id::text, user_id::text, zone_id::text, workspace_id::text, domain::text,
       refresh_token_hash, device_name, host(ip_address), user_agent,
       expires_at, revoked_at, created_at, updated_at
FROM user_sessions
WHERE refresh_token_hash = $1
`, hash)
	return scanSession(row)
}

func (r *Repository) RotateSessionRefreshToken(ctx context.Context, params authapp.RotateSessionParams) (authdomain.Session, error) {
	row := r.pool.QueryRow(ctx, `
UPDATE user_sessions
SET refresh_token_hash = $3, expires_at = $4, revoked_at = NULL
WHERE id = $1::uuid AND user_id = $2::uuid AND revoked_at IS NULL
RETURNING id::text, user_id::text, zone_id::text, workspace_id::text, domain::text,
          refresh_token_hash, device_name, host(ip_address), user_agent,
          expires_at, revoked_at, created_at, updated_at
`, params.SessionID, params.UserID, params.RefreshTokenHash, params.ExpiresAt)
	return scanSession(row)
}

func (r *Repository) RevokeSessionByRefreshTokenHash(ctx context.Context, hash string, revokedAt time.Time) error {
	_, err := r.pool.Exec(ctx, `
UPDATE user_sessions
SET revoked_at = $2
WHERE refresh_token_hash = $1 AND revoked_at IS NULL
`, hash, revokedAt)
	return err
}

func (r *Repository) RevokeSessionByID(ctx context.Context, userID string, zoneID string, sessionID string, revokedAt time.Time) error {
	command, err := r.pool.Exec(ctx, `
UPDATE user_sessions
SET revoked_at = $4
WHERE id = $1::uuid
  AND user_id = $2::uuid
  AND zone_id = $3::uuid
  AND revoked_at IS NULL
`, sessionID, userID, zoneID, revokedAt)
	if err != nil {
		return err
	}
	if command.RowsAffected() == 0 {
		return authdomain.ErrSessionNotFound
	}
	return nil
}

func (r *Repository) RevokeAllSessions(ctx context.Context, userID string, zoneID string, revokedAt time.Time) error {
	_, err := r.pool.Exec(ctx, `
UPDATE user_sessions
SET revoked_at = $3
WHERE user_id = $1::uuid
  AND zone_id = $2::uuid
  AND revoked_at IS NULL
`, userID, zoneID, revokedAt)
	return err
}

func (r *Repository) ListSessions(ctx context.Context, userID string, zoneID string) ([]authdomain.Session, error) {
	rows, err := r.pool.Query(ctx, `
SELECT id::text, user_id::text, zone_id::text, workspace_id::text, domain::text,
       refresh_token_hash, device_name, host(ip_address), user_agent,
       expires_at, revoked_at, created_at, updated_at
FROM user_sessions
WHERE user_id = $1::uuid
  AND zone_id = $2::uuid
ORDER BY created_at DESC
`, userID, zoneID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessions []authdomain.Session
	for rows.Next() {
		session, err := scanSession(rows)
		if err != nil {
			return nil, err
		}
		sessions = append(sessions, session)
	}
	return sessions, rows.Err()
}

func (r *Repository) RecordAudit(ctx context.Context, event authapp.AuditEvent) error {
	metadata := event.Metadata
	if metadata == nil {
		metadata = map[string]any{}
	}
	metadataBytes, err := json.Marshal(metadata)
	if err != nil {
		return err
	}

	_, err = r.pool.Exec(ctx, `
INSERT INTO audit_logs (
    zone_id, workspace_id, actor_user_id, action, entity_type, entity_id,
    ip_address, user_agent, metadata
)
VALUES (
    NULLIF($1, '')::uuid, NULLIF($2, '')::uuid, NULLIF($3, '')::uuid,
    $4, $5, NULLIF($6, '')::uuid, NULLIF($7, '')::inet, NULLIF($8, ''), $9::jsonb
)
	`, event.ZoneID, event.WorkspaceID, event.ActorUserID, event.Action, event.EntityType,
		event.EntityID, event.IPAddress, event.UserAgent, string(metadataBytes))
	return err
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanUser(row rowScanner) (authdomain.User, error) {
	var user authdomain.User
	var avatarURL sql.NullString
	var emailVerifiedAt sql.NullTime
	var lastSeenAt sql.NullTime
	var registrationIP sql.NullString
	var registrationDevice sql.NullString
	var lastIPAddress sql.NullString
	var deviceName sql.NullString

	err := row.Scan(
		&user.ID,
		&user.Email,
		&user.Username,
		&user.DisplayName,
		&user.PasswordHash,
		&avatarURL,
		&user.Status,
		&user.Locale,
		&user.Timezone,
		&emailVerifiedAt,
		&lastSeenAt,
		&registrationIP,
		&registrationDevice,
		&lastIPAddress,
		&deviceName,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return authdomain.User{}, authdomain.ErrUserNotFound
		}
		return authdomain.User{}, err
	}

	user.AvatarURL = nullStringPtr(avatarURL)
	user.EmailVerifiedAt = nullTimePtr(emailVerifiedAt)
	user.LastSeenAt = nullTimePtr(lastSeenAt)
	user.RegistrationIP = nullStringPtr(registrationIP)
	user.RegistrationDev = nullStringPtr(registrationDevice)
	user.LastIPAddress = nullStringPtr(lastIPAddress)
	user.DeviceName = nullStringPtr(deviceName)
	return user, nil
}

func scanSession(row rowScanner) (authdomain.Session, error) {
	var session authdomain.Session
	var deviceName sql.NullString
	var ipAddress sql.NullString
	var userAgent sql.NullString
	var revokedAt sql.NullTime

	err := row.Scan(
		&session.ID,
		&session.UserID,
		&session.ZoneID,
		&session.WorkspaceID,
		&session.Domain,
		&session.RefreshTokenHash,
		&deviceName,
		&ipAddress,
		&userAgent,
		&session.ExpiresAt,
		&revokedAt,
		&session.CreatedAt,
		&session.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return authdomain.Session{}, authdomain.ErrSessionNotFound
		}
		return authdomain.Session{}, err
	}

	session.DeviceName = nullStringPtr(deviceName)
	session.IPAddress = nullStringPtr(ipAddress)
	session.UserAgent = nullStringPtr(userAgent)
	session.RevokedAt = nullTimePtr(revokedAt)
	return session, nil
}

func nullStringPtr(value sql.NullString) *string {
	if !value.Valid {
		return nil
	}
	return &value.String
}

func nullTimePtr(value sql.NullTime) *time.Time {
	if !value.Valid {
		return nil
	}
	return &value.Time
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
