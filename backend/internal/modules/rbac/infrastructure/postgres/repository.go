package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	rbacapp "github.com/duclamdev/application-chat/backend/internal/modules/rbac/application"
	rbacdomain "github.com/duclamdev/application-chat/backend/internal/modules/rbac/domain"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) WorkspaceBelongsToZone(ctx context.Context, workspaceID string, zoneID string) (bool, error) {
	var matches bool
	err := r.pool.QueryRow(ctx, `
SELECT EXISTS (
    SELECT 1
    FROM workspaces
    WHERE id = $1::uuid
      AND zone_id = $2::uuid
      AND status = 'active'
      AND deleted_at IS NULL
)
`, workspaceID, zoneID).Scan(&matches)
	return matches, err
}

func (r *Repository) ListPermissions(ctx context.Context) ([]rbacdomain.Permission, error) {
	rows, err := r.pool.Query(ctx, `
SELECT id::text, code::text, module, action, name, description
FROM permissions
ORDER BY module, action, code
`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var permissions []rbacdomain.Permission
	for rows.Next() {
		permission, err := scanPermission(rows)
		if err != nil {
			return nil, err
		}
		permissions = append(permissions, permission)
	}
	return permissions, rows.Err()
}

func (r *Repository) ListRoles(ctx context.Context, workspaceID string) ([]rbacdomain.Role, error) {
	rows, err := r.pool.Query(ctx, `
SELECT id::text, workspace_id::text, code::text, name, description, is_system, created_by::text
FROM roles
WHERE deleted_at IS NULL
  AND (workspace_id IS NULL OR workspace_id = NULLIF($1, '')::uuid)
ORDER BY workspace_id NULLS FIRST, is_system DESC, name
`, workspaceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var roles []rbacdomain.Role
	for rows.Next() {
		role, err := scanRole(rows)
		if err != nil {
			return nil, err
		}
		permissions, err := r.permissionsByRoleID(ctx, role.ID)
		if err != nil {
			return nil, err
		}
		role.Permissions = permissions
		roles = append(roles, role)
	}
	return roles, rows.Err()
}

func (r *Repository) CreateRole(ctx context.Context, params rbacapp.CreateRoleParams) (rbacdomain.Role, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return rbacdomain.Role{}, err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	row := tx.QueryRow(ctx, `
INSERT INTO roles (workspace_id, code, name, description, is_system, created_by)
VALUES ($1::uuid, $2, $3, NULLIF($4, ''), false, $5::uuid)
RETURNING id::text, workspace_id::text, code::text, name, description, is_system, created_by::text
`, params.WorkspaceID, params.Code, params.Name, params.Description, params.CreatedBy)
	role, err := scanRole(row)
	if err != nil {
		if isUniqueViolation(err) {
			return rbacdomain.Role{}, rbacdomain.ErrRoleAlreadyExists
		}
		return rbacdomain.Role{}, err
	}

	for _, code := range params.PermissionCodes {
		if _, err := tx.Exec(ctx, `
INSERT INTO role_permissions (role_id, permission_id)
SELECT $1::uuid, id
FROM permissions
WHERE code = $2
ON CONFLICT DO NOTHING
`, role.ID, code); err != nil {
			return rbacdomain.Role{}, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return rbacdomain.Role{}, err
	}
	role.Permissions, err = r.permissionsByRoleID(ctx, role.ID)
	if err != nil {
		return rbacdomain.Role{}, err
	}
	return role, nil
}

func (r *Repository) ListWorkspaceMemberRoles(ctx context.Context, workspaceID string, userID string) ([]rbacdomain.Role, error) {
	rows, err := r.pool.Query(ctx, `
SELECT r.id::text, r.workspace_id::text, r.code::text, r.name, r.description, r.is_system, r.created_by::text
FROM workspace_member_roles wmr
JOIN roles r ON r.id = wmr.role_id AND r.deleted_at IS NULL
WHERE wmr.workspace_id = $1::uuid
  AND wmr.user_id = $2::uuid
ORDER BY r.is_system DESC, r.name
`, workspaceID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var roles []rbacdomain.Role
	for rows.Next() {
		role, err := scanRole(rows)
		if err != nil {
			return nil, err
		}
		role.Permissions, err = r.permissionsByRoleID(ctx, role.ID)
		if err != nil {
			return nil, err
		}
		roles = append(roles, role)
	}
	return roles, rows.Err()
}

func (r *Repository) AssignWorkspaceRole(ctx context.Context, params rbacapp.AssignWorkspaceRoleParams) error {
	command, err := r.pool.Exec(ctx, `
INSERT INTO workspace_member_roles (workspace_id, user_id, role_id, assigned_by)
SELECT $1::uuid, $2::uuid, r.id, $4::uuid
FROM roles r
JOIN workspace_members wm
  ON wm.workspace_id = $1::uuid AND wm.user_id = $2::uuid AND wm.status = 'active'
WHERE r.id = $3::uuid
  AND r.deleted_at IS NULL
  AND (r.workspace_id IS NULL OR r.workspace_id = $1::uuid)
ON CONFLICT DO NOTHING
`, params.WorkspaceID, params.UserID, params.RoleID, params.AssignedBy)
	if err != nil {
		return err
	}
	if command.RowsAffected() == 0 {
		return rbacdomain.ErrRoleNotFound
	}
	return nil
}

func (r *Repository) RevokeWorkspaceRole(ctx context.Context, params rbacapp.RevokeWorkspaceRoleParams) error {
	_, err := r.pool.Exec(ctx, `
DELETE FROM workspace_member_roles
WHERE workspace_id = $1::uuid
  AND user_id = $2::uuid
  AND role_id = $3::uuid
`, params.WorkspaceID, params.UserID, params.RoleID)
	return err
}

func (r *Repository) ListUserWorkspacePermissions(ctx context.Context, userID string, workspaceID string) ([]rbacdomain.Permission, error) {
	rows, err := r.pool.Query(ctx, `
WITH owner_workspace AS (
    SELECT 1
    FROM workspaces
    WHERE id = $2::uuid AND owner_id = $1::uuid AND deleted_at IS NULL
)
SELECT DISTINCT p.id::text, p.code::text, p.module, p.action, p.name, p.description
FROM permissions p
WHERE EXISTS (SELECT 1 FROM owner_workspace)
UNION
SELECT DISTINCT p.id::text, p.code::text, p.module, p.action, p.name, p.description
FROM workspace_members wm
JOIN workspace_member_roles wmr
  ON wmr.workspace_id = wm.workspace_id AND wmr.user_id = wm.user_id
JOIN roles r
  ON r.id = wmr.role_id AND r.deleted_at IS NULL
JOIN role_permissions rp
  ON rp.role_id = r.id
JOIN permissions p
  ON p.id = rp.permission_id
WHERE wm.workspace_id = $2::uuid
  AND wm.user_id = $1::uuid
  AND wm.status = 'active'
  AND (r.workspace_id IS NULL OR r.workspace_id = $2::uuid)
ORDER BY module, action, code
`, userID, workspaceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var permissions []rbacdomain.Permission
	for rows.Next() {
		permission, err := scanPermission(rows)
		if err != nil {
			return nil, err
		}
		permissions = append(permissions, permission)
	}
	return permissions, rows.Err()
}

func (r *Repository) HasWorkspacePermission(ctx context.Context, userID string, workspaceID string, permissionCode string) (bool, error) {
	var allowed bool
	err := r.pool.QueryRow(ctx, `
SELECT EXISTS (
    SELECT 1
    FROM workspaces
    WHERE id = $2::uuid AND owner_id = $1::uuid AND deleted_at IS NULL
)
OR EXISTS (
    SELECT 1
    FROM workspace_members wm
    JOIN workspace_member_roles wmr
      ON wmr.workspace_id = wm.workspace_id AND wmr.user_id = wm.user_id
    JOIN roles r
      ON r.id = wmr.role_id AND r.deleted_at IS NULL
    JOIN role_permissions rp
      ON rp.role_id = r.id
    JOIN permissions p
      ON p.id = rp.permission_id
    WHERE wm.workspace_id = $2::uuid
      AND wm.user_id = $1::uuid
      AND wm.status = 'active'
      AND (r.workspace_id IS NULL OR r.workspace_id = $2::uuid)
      AND p.code = $3
)
`, userID, workspaceID, permissionCode).Scan(&allowed)
	return allowed, err
}

func (r *Repository) HasAnyWorkspacePermission(ctx context.Context, userID string, permissionCode string) (bool, error) {
	var allowed bool
	err := r.pool.QueryRow(ctx, `
SELECT EXISTS (
    SELECT 1
    FROM workspace_members wm
    JOIN workspaces w
      ON w.id = wm.workspace_id AND w.deleted_at IS NULL AND w.status = 'active'
    JOIN workspace_member_roles wmr
      ON wmr.workspace_id = wm.workspace_id AND wmr.user_id = wm.user_id
    JOIN roles r
      ON r.id = wmr.role_id AND r.deleted_at IS NULL
    JOIN role_permissions rp
      ON rp.role_id = r.id
    JOIN permissions p
      ON p.id = rp.permission_id
    WHERE wm.user_id = $1::uuid
      AND wm.status = 'active'
      AND (r.workspace_id IS NULL OR r.workspace_id = wm.workspace_id)
      AND p.code = $2
)
`, userID, permissionCode).Scan(&allowed)
	return allowed, err
}

func (r *Repository) HasAnyZonePermission(ctx context.Context, userID string, zoneID string, permissionCode string) (bool, error) {
	var allowed bool
	err := r.pool.QueryRow(ctx, `
SELECT EXISTS (
    SELECT 1
    FROM workspace_members wm
    JOIN workspaces w
      ON w.id = wm.workspace_id
     AND w.zone_id = $2::uuid
     AND w.deleted_at IS NULL
     AND w.status = 'active'
    JOIN workspace_member_roles wmr
      ON wmr.workspace_id = wm.workspace_id AND wmr.user_id = wm.user_id
    JOIN roles r
      ON r.id = wmr.role_id AND r.deleted_at IS NULL
    JOIN role_permissions rp
      ON rp.role_id = r.id
    JOIN permissions p
      ON p.id = rp.permission_id
    WHERE wm.user_id = $1::uuid
      AND wm.status = 'active'
      AND (r.workspace_id IS NULL OR r.workspace_id = wm.workspace_id)
      AND p.code = $3
)
`, userID, zoneID, permissionCode).Scan(&allowed)
	return allowed, err
}

func (r *Repository) RecordAudit(ctx context.Context, event rbacapp.AuditEvent) error {
	metadata := event.Metadata
	if metadata == nil {
		metadata = map[string]any{}
	}
	metadataBytes, err := json.Marshal(metadata)
	if err != nil {
		return err
	}

	_, err = r.pool.Exec(ctx, `
INSERT INTO audit_logs (workspace_id, actor_user_id, action, entity_type, entity_id, metadata)
VALUES (NULLIF($1, '')::uuid, NULLIF($2, '')::uuid, $3, $4, NULLIF($5, '')::uuid, $6::jsonb)
`, event.WorkspaceID, event.ActorUserID, event.Action, event.EntityType, event.EntityID, string(metadataBytes))
	return err
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanPermission(row rowScanner) (rbacdomain.Permission, error) {
	var permission rbacdomain.Permission
	var description sql.NullString
	if err := row.Scan(
		&permission.ID,
		&permission.Code,
		&permission.Module,
		&permission.Action,
		&permission.Name,
		&description,
	); err != nil {
		return rbacdomain.Permission{}, err
	}
	if description.Valid {
		permission.Description = &description.String
	}
	return permission, nil
}

func scanRole(row rowScanner) (rbacdomain.Role, error) {
	var role rbacdomain.Role
	var workspaceID sql.NullString
	var description sql.NullString
	var createdBy sql.NullString
	if err := row.Scan(
		&role.ID,
		&workspaceID,
		&role.Code,
		&role.Name,
		&description,
		&role.IsSystem,
		&createdBy,
	); err != nil {
		return rbacdomain.Role{}, err
	}
	if workspaceID.Valid {
		role.WorkspaceID = &workspaceID.String
	}
	if description.Valid {
		role.Description = &description.String
	}
	if createdBy.Valid {
		role.CreatedBy = &createdBy.String
	}
	return role, nil
}

func (r *Repository) permissionsByRoleID(ctx context.Context, roleID string) ([]rbacdomain.Permission, error) {
	rows, err := r.pool.Query(ctx, `
SELECT p.id::text, p.code::text, p.module, p.action, p.name, p.description
FROM role_permissions rp
JOIN permissions p ON p.id = rp.permission_id
WHERE rp.role_id = $1::uuid
ORDER BY p.module, p.action, p.code
`, roleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var permissions []rbacdomain.Permission
	for rows.Next() {
		permission, err := scanPermission(rows)
		if err != nil {
			return nil, err
		}
		permissions = append(permissions, permission)
	}
	return permissions, rows.Err()
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
