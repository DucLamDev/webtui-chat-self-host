package postgres

import (
	"context"
	"database/sql"
	"errors"
	"time"

	usersapp "github.com/duclamdev/application-chat/backend/internal/modules/users/application"
	usersdomain "github.com/duclamdev/application-chat/backend/internal/modules/users/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) FindByID(ctx context.Context, id string) (usersdomain.User, error) {
	row := r.pool.QueryRow(ctx, `
SELECT id::text, email::text, username::text, display_name, avatar_url, phone_number, status,
       locale, timezone, email_verified_at, last_seen_at,
       host(registration_ip_address), registration_device_name,
       host(last_ip_address), device_name,
       created_at, updated_at
FROM users
WHERE id = $1::uuid AND deleted_at IS NULL
`, id)
	return scanUser(row)
}

func (r *Repository) FindByIDInZone(
	ctx context.Context,
	id string,
	zoneID string,
	actorUserID string,
	allowZoneWide bool,
) (usersdomain.User, error) {
	row := r.pool.QueryRow(ctx, `
SELECT DISTINCT
       users.id::text, users.email::text, users.username::text, users.display_name,
       users.avatar_url, users.phone_number, users.status,
       users.locale, users.timezone, users.email_verified_at, users.last_seen_at,
       host(users.registration_ip_address), users.registration_device_name,
       host(users.last_ip_address), users.device_name,
       users.created_at, users.updated_at
FROM users
JOIN workspace_members member
  ON member.user_id = users.id
 AND member.status = 'active'
JOIN workspaces workspace
  ON workspace.id = member.workspace_id
 AND workspace.zone_id = $2::uuid
 AND workspace.status = 'active'
 AND workspace.deleted_at IS NULL
WHERE users.id = $1::uuid
  AND users.deleted_at IS NULL
	AND (
		$4::boolean
		OR users.id = $3::uuid
		OR EXISTS (
			SELECT 1
			FROM workspace_members target_member
			JOIN workspace_members actor_member
			  ON actor_member.workspace_id = target_member.workspace_id
			 AND actor_member.user_id = $3::uuid
			 AND actor_member.status = 'active'
			JOIN workspaces shared_workspace
			  ON shared_workspace.id = target_member.workspace_id
			 AND shared_workspace.zone_id = $2::uuid
			 AND shared_workspace.status = 'active'
			 AND shared_workspace.deleted_at IS NULL
			WHERE target_member.user_id = users.id
			  AND target_member.status = 'active'
		)
	)
LIMIT 1
`, id, zoneID, actorUserID, allowZoneWide)
	return scanUser(row)
}

func (r *Repository) List(ctx context.Context, params usersapp.ListUsersParams) ([]usersdomain.User, error) {
	rows, err := r.pool.Query(ctx, `
SELECT id::text, email::text, username::text, display_name, avatar_url, phone_number, status,
       locale, timezone, email_verified_at, last_seen_at,
       host(registration_ip_address), registration_device_name,
       host(last_ip_address), device_name,
       created_at, updated_at
FROM users
WHERE deleted_at IS NULL
  AND EXISTS (
      SELECT 1
      FROM workspace_members member
      JOIN workspaces workspace
        ON workspace.id = member.workspace_id
       AND workspace.zone_id = $4::uuid
       AND workspace.status = 'active'
       AND workspace.deleted_at IS NULL
      WHERE member.user_id = users.id
        AND member.status = 'active'
  )
  AND (
    $1 = ''
    OR display_name ILIKE '%' || $1 || '%'
    OR email::text ILIKE '%' || $1 || '%'
    OR username::text ILIKE '%' || $1 || '%'
    OR (
      regexp_replace($1, '\D', '', 'g') <> ''
      AND regexp_replace(COALESCE(phone_number, ''), '\D', '', 'g') LIKE '%' || regexp_replace($1, '\D', '', 'g') || '%'
    )
  )
  AND ($2 = '' OR status = $2)
	AND (
		$6::boolean
		OR EXISTS (
			SELECT 1
			FROM workspace_members target_member
			JOIN workspace_members actor_member
			  ON actor_member.workspace_id = target_member.workspace_id
			 AND actor_member.user_id = $5::uuid
			 AND actor_member.status = 'active'
			JOIN workspaces shared_workspace
			  ON shared_workspace.id = target_member.workspace_id
			 AND shared_workspace.zone_id = $4::uuid
			 AND shared_workspace.status = 'active'
			 AND shared_workspace.deleted_at IS NULL
			WHERE target_member.user_id = users.id
			  AND target_member.status = 'active'
		)
	)
ORDER BY created_at DESC
LIMIT $3
`, params.Query, params.Status, params.Limit, params.ZoneID, params.ActorUserID, params.AllowZoneWide)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	users := make([]usersdomain.User, 0)
	for rows.Next() {
		user, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		users = append(users, user)
	}
	return users, rows.Err()
}

func (r *Repository) UserBelongsToWorkspace(ctx context.Context, userID string, workspaceID string) (bool, error) {
	var belongs bool
	err := r.pool.QueryRow(ctx, `
SELECT EXISTS (
    SELECT 1
    FROM workspace_members
    WHERE user_id = $1::uuid
      AND workspace_id = $2::uuid
      AND status = 'active'
)
`, userID, workspaceID).Scan(&belongs)
	return belongs, err
}

func (r *Repository) UpdateProfile(ctx context.Context, params usersapp.UpdateProfileParams) (usersdomain.User, error) {
	row := r.pool.QueryRow(ctx, `
UPDATE users
SET display_name = COALESCE($2, display_name),
    avatar_url = COALESCE($3, avatar_url),
    phone_number = COALESCE($4, phone_number),
    locale = COALESCE($5, locale),
    timezone = COALESCE($6, timezone)
WHERE id = $1::uuid AND deleted_at IS NULL
RETURNING id::text, email::text, username::text, display_name, avatar_url, phone_number, status,
          locale, timezone, email_verified_at, last_seen_at,
          host(registration_ip_address), registration_device_name,
          host(last_ip_address), device_name,
          created_at, updated_at
`, params.UserID, params.DisplayName, params.AvatarURL, params.PhoneNumber, params.Locale, params.Timezone)
	return scanUser(row)
}

func (r *Repository) UpdateUser(ctx context.Context, params usersapp.UpdateUserParams) (usersdomain.User, error) {
	row := r.pool.QueryRow(ctx, `
UPDATE users
SET display_name = COALESCE($2, display_name),
    avatar_url = COALESCE($3, avatar_url),
    phone_number = COALESCE($4, phone_number),
    locale = COALESCE($5, locale),
    timezone = COALESCE($6, timezone),
    status = COALESCE($7, status)
WHERE id = $1::uuid AND deleted_at IS NULL
RETURNING id::text, email::text, username::text, display_name, avatar_url, phone_number, status,
          locale, timezone, email_verified_at, last_seen_at,
          host(registration_ip_address), registration_device_name,
          host(last_ip_address), device_name,
          created_at, updated_at
`, params.UserID, params.DisplayName, params.AvatarURL, params.PhoneNumber, params.Locale, params.Timezone, params.Status)
	return scanUser(row)
}

func (r *Repository) DeleteUser(ctx context.Context, userID string) error {
	command, err := r.pool.Exec(ctx, `
UPDATE users
SET status = 'disabled', deleted_at = now()
WHERE id = $1::uuid AND deleted_at IS NULL
`, userID)
	if err != nil {
		return err
	}
	if command.RowsAffected() == 0 {
		return usersdomain.ErrUserNotFound
	}
	return nil
}

func (r *Repository) DeleteOwnAccount(ctx context.Context, userID string, ownershipSuccessorEmail string) error {
	// A hard delete is intentional for a user-initiated privacy request. The
	// schema cascades private account data (sessions, device tokens, presence,
	// memberships and preferences) while organisational records such as sent
	// messages retain their history with a NULL sender. Locking the user makes
	// the ownership check and delete atomic with concurrent ownership changes.
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	var lockedUserID string
	err = tx.QueryRow(ctx, `
SELECT id::text
FROM users
WHERE id = $1::uuid AND deleted_at IS NULL
FOR UPDATE
`, userID).Scan(&lockedUserID)
	if errors.Is(err, pgx.ErrNoRows) {
		return usersdomain.ErrUserNotFound
	}
	if err != nil {
		return err
	}

	rows, err := tx.Query(ctx, `
SELECT id::text
FROM workspaces
WHERE owner_id = $1::uuid
  AND deleted_at IS NULL
ORDER BY id
FOR UPDATE
`, lockedUserID)
	if err != nil {
		return err
	}
	ownedWorkspaceIDs := make([]string, 0)
	for rows.Next() {
		var workspaceID string
		if err := rows.Scan(&workspaceID); err != nil {
			rows.Close()
			return err
		}
		ownedWorkspaceIDs = append(ownedWorkspaceIDs, workspaceID)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()

	if len(ownedWorkspaceIDs) > 0 {
		if ownershipSuccessorEmail == "" {
			return usersdomain.ErrUserOwnsWorkspace
		}

		var successorUserID string
		err := tx.QueryRow(ctx, `
SELECT id::text
FROM users
WHERE email = $1
  AND id <> $2::uuid
  AND status = 'active'
  AND deleted_at IS NULL
FOR KEY SHARE
`, ownershipSuccessorEmail, lockedUserID).Scan(&successorUserID)
		if errors.Is(err, pgx.ErrNoRows) {
			return usersdomain.ErrOwnershipSuccessorNotEligible
		}
		if err != nil {
			return err
		}

		for _, workspaceID := range ownedWorkspaceIDs {
			var memberUserID string
			err := tx.QueryRow(ctx, `
SELECT user_id::text
FROM workspace_members
WHERE workspace_id = $1::uuid
  AND user_id = $2::uuid
  AND status = 'active'
FOR UPDATE
`, workspaceID, successorUserID).Scan(&memberUserID)
			if errors.Is(err, pgx.ErrNoRows) {
				return usersdomain.ErrOwnershipSuccessorNotEligible
			}
			if err != nil {
				return err
			}

			roleAssignment, err := tx.Exec(ctx, `
INSERT INTO workspace_member_roles (workspace_id, user_id, role_id, assigned_by)
SELECT $1::uuid, $2::uuid, role.id, $3::uuid
FROM roles role
WHERE role.code = 'workspace_owner'
  AND role.is_system = true
  AND role.deleted_at IS NULL
  AND (role.workspace_id = $1::uuid OR role.workspace_id IS NULL)
ORDER BY (role.workspace_id = $1::uuid) DESC, role.created_at
LIMIT 1
ON CONFLICT (workspace_id, user_id, role_id) DO NOTHING
`, workspaceID, successorUserID, lockedUserID)
			if err != nil {
				return err
			}
			if roleAssignment.RowsAffected() == 0 {
				var alreadyAssigned bool
				if err := tx.QueryRow(ctx, `
SELECT EXISTS (
    SELECT 1
    FROM workspace_member_roles member_role
    JOIN roles role ON role.id = member_role.role_id
    WHERE member_role.workspace_id = $1::uuid
      AND member_role.user_id = $2::uuid
      AND role.code = 'workspace_owner'
      AND role.deleted_at IS NULL
)
`, workspaceID, successorUserID).Scan(&alreadyAssigned); err != nil {
					return err
				}
				if !alreadyAssigned {
					return errors.New("workspace_owner system role is unavailable")
				}
			}

			ownershipUpdate, err := tx.Exec(ctx, `
UPDATE workspaces
SET owner_id = $2::uuid
WHERE id = $1::uuid
  AND owner_id = $3::uuid
  AND deleted_at IS NULL
`, workspaceID, successorUserID, lockedUserID)
			if err != nil {
				return err
			}
			if ownershipUpdate.RowsAffected() != 1 {
				return errors.New("workspace ownership changed during account deletion")
			}
		}
	}

	command, err := tx.Exec(ctx, `
DELETE FROM users
WHERE id = $1::uuid AND deleted_at IS NULL
`, lockedUserID)
	if err != nil {
		return err
	}
	if command.RowsAffected() == 0 {
		return usersdomain.ErrUserNotFound
	}
	return tx.Commit(ctx)
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanUser(row rowScanner) (usersdomain.User, error) {
	var user usersdomain.User
	var avatarURL sql.NullString
	var phoneNumber sql.NullString
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
		&avatarURL,
		&phoneNumber,
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
			return usersdomain.User{}, usersdomain.ErrUserNotFound
		}
		return usersdomain.User{}, err
	}

	user.AvatarURL = nullStringPtr(avatarURL)
	user.PhoneNumber = nullStringPtr(phoneNumber)
	user.EmailVerifiedAt = nullTimePtr(emailVerifiedAt)
	user.LastSeenAt = nullTimePtr(lastSeenAt)
	user.RegistrationIP = nullStringPtr(registrationIP)
	user.RegistrationDev = nullStringPtr(registrationDevice)
	user.LastIPAddress = nullStringPtr(lastIPAddress)
	user.DeviceName = nullStringPtr(deviceName)
	return user, nil
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
