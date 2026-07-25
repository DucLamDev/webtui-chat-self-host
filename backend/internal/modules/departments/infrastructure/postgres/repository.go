package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	departmentsapp "github.com/duclamdev/application-chat/backend/internal/modules/departments/application"
	departmentsdomain "github.com/duclamdev/application-chat/backend/internal/modules/departments/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) Create(ctx context.Context, params departmentsapp.CreateParams) (departmentsdomain.Department, error) {
	row := r.pool.QueryRow(ctx, `
INSERT INTO departments (workspace_id, parent_id, slug, name, description, created_by)
VALUES ($1::uuid, NULLIF($2, '')::uuid, $3, $4, NULLIF($5, ''), $6::uuid)
RETURNING id::text, workspace_id::text, parent_id::text, slug::text, name, description, created_by::text, created_at, updated_at
`, params.WorkspaceID, params.ParentID, params.Slug, params.Name, params.Description, params.CreatedBy)
	department, err := scanDepartment(row)
	if err != nil {
		if isUniqueViolation(err) {
			return departmentsdomain.Department{}, departmentsdomain.ErrDepartmentConflict
		}
		return departmentsdomain.Department{}, err
	}
	return department, nil
}

func (r *Repository) Find(ctx context.Context, workspaceID string, departmentID string) (departmentsdomain.Department, error) {
	row := r.pool.QueryRow(ctx, `
SELECT id::text, workspace_id::text, parent_id::text, slug::text, name, description, created_by::text, created_at, updated_at
FROM departments
WHERE workspace_id = $1::uuid AND id = $2::uuid AND deleted_at IS NULL
`, workspaceID, departmentID)
	return scanDepartment(row)
}

func (r *Repository) List(ctx context.Context, workspaceID string) ([]departmentsdomain.Department, error) {
	rows, err := r.pool.Query(ctx, `
SELECT d.id::text, d.workspace_id::text, d.parent_id::text, d.slug::text, d.name, d.description, d.created_by::text,
       d.created_at, d.updated_at,
       (SELECT count(*)::int FROM department_members dm JOIN users u ON u.id = dm.user_id AND u.deleted_at IS NULL WHERE dm.department_id = d.id),
       (SELECT count(*)::int FROM department_members dm JOIN users u ON u.id = dm.user_id AND u.deleted_at IS NULL WHERE dm.department_id = d.id AND dm.role = 'lead'),
       (SELECT count(*)::int FROM channels c WHERE c.department_id = d.id AND c.deleted_at IS NULL)
FROM departments d
WHERE d.workspace_id = $1::uuid AND d.deleted_at IS NULL
ORDER BY d.name
`, workspaceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var departments []departmentsdomain.Department
	for rows.Next() {
		department, err := scanDepartmentSummary(rows)
		if err != nil {
			return nil, err
		}
		departments = append(departments, department)
	}
	return departments, rows.Err()
}

func scanDepartmentSummary(row rowScanner) (departmentsdomain.Department, error) {
	var department departmentsdomain.Department
	var parentID sql.NullString
	var description sql.NullString
	var createdBy sql.NullString
	if err := row.Scan(
		&department.ID,
		&department.WorkspaceID,
		&parentID,
		&department.Slug,
		&department.Name,
		&description,
		&createdBy,
		&department.CreatedAt,
		&department.UpdatedAt,
		&department.MemberCount,
		&department.LeadCount,
		&department.ChannelCount,
	); err != nil {
		return departmentsdomain.Department{}, err
	}
	department.ParentID = nullStringPtr(parentID)
	department.Description = nullStringPtr(description)
	department.CreatedBy = nullStringPtr(createdBy)
	return department, nil
}

func (r *Repository) CanSetParent(ctx context.Context, workspaceID string, departmentID string, parentID string) (bool, error) {
	var allowed bool
	err := r.pool.QueryRow(ctx, `
WITH RECURSIVE descendants AS (
    SELECT id
    FROM departments
    WHERE $2 <> ''
      AND workspace_id = $1::uuid
      AND parent_id = NULLIF($2, '')::uuid
      AND deleted_at IS NULL
    UNION
    SELECT child.id
    FROM departments child
    JOIN descendants parent ON child.parent_id = parent.id
    WHERE child.workspace_id = $1::uuid AND child.deleted_at IS NULL
)
SELECT EXISTS (
    SELECT 1
    FROM departments candidate
    WHERE candidate.workspace_id = $1::uuid
      AND candidate.id = $3::uuid
      AND candidate.deleted_at IS NULL
      AND ($2 = '' OR candidate.id <> NULLIF($2, '')::uuid)
      AND NOT EXISTS (SELECT 1 FROM descendants WHERE id = candidate.id)
)
`, workspaceID, departmentID, parentID).Scan(&allowed)
	return allowed, err
}

func (r *Repository) Update(ctx context.Context, params departmentsapp.UpdateParams) (departmentsdomain.Department, error) {
	row := r.pool.QueryRow(ctx, `
UPDATE departments
SET parent_id = CASE WHEN $3::text IS NULL THEN parent_id ELSE NULLIF($3::text, '')::uuid END,
    slug = COALESCE($4, slug),
    name = COALESCE($5, name),
    description = COALESCE($6, description)
WHERE workspace_id = $1::uuid AND id = $2::uuid AND deleted_at IS NULL
RETURNING id::text, workspace_id::text, parent_id::text, slug::text, name, description, created_by::text, created_at, updated_at
`, params.WorkspaceID, params.DepartmentID, params.ParentID, params.Slug, params.Name, params.Description)
	department, err := scanDepartment(row)
	if err != nil && isUniqueViolation(err) {
		return departmentsdomain.Department{}, departmentsdomain.ErrDepartmentConflict
	}
	return department, err
}

func (r *Repository) Delete(ctx context.Context, workspaceID string, departmentID string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	command, err := tx.Exec(ctx, `
UPDATE departments
SET deleted_at = now()
WHERE workspace_id = $1::uuid AND id = $2::uuid AND deleted_at IS NULL
`, workspaceID, departmentID)
	if err != nil {
		return err
	}
	if command.RowsAffected() == 0 {
		return departmentsdomain.ErrDepartmentNotFound
	}
	if _, err := tx.Exec(ctx, `
UPDATE departments
SET parent_id = NULL
WHERE workspace_id = $1::uuid AND parent_id = $2::uuid AND deleted_at IS NULL
`, workspaceID, departmentID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
UPDATE channels
SET department_id = NULL
WHERE workspace_id = $1::uuid AND department_id = $2::uuid AND deleted_at IS NULL
`, workspaceID, departmentID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *Repository) AddMember(ctx context.Context, params departmentsapp.AddMemberParams) (departmentsdomain.Member, error) {
	command, err := r.pool.Exec(ctx, `
INSERT INTO department_members (department_id, user_id, role)
SELECT d.id, $3::uuid, $4
FROM departments d
JOIN workspace_members wm
  ON wm.workspace_id = d.workspace_id AND wm.user_id = $3::uuid AND wm.status = 'active'
WHERE d.workspace_id = $1::uuid AND d.id = $2::uuid AND d.deleted_at IS NULL
ON CONFLICT (department_id, user_id)
DO UPDATE SET role = EXCLUDED.role
`, params.WorkspaceID, params.DepartmentID, params.UserID, params.Role)
	if err != nil {
		return departmentsdomain.Member{}, err
	}
	if command.RowsAffected() == 0 {
		return departmentsdomain.Member{}, departmentsdomain.ErrMemberNotFound
	}
	return r.member(ctx, params.WorkspaceID, params.DepartmentID, params.UserID)
}

func (r *Repository) RemoveMember(ctx context.Context, workspaceID string, departmentID string, userID string) error {
	command, err := r.pool.Exec(ctx, `
DELETE FROM department_members dm
USING departments d
WHERE dm.department_id = d.id
  AND d.workspace_id = $1::uuid
  AND d.id = $2::uuid
  AND dm.user_id = $3::uuid
`, workspaceID, departmentID, userID)
	if err != nil {
		return err
	}
	if command.RowsAffected() == 0 {
		return departmentsdomain.ErrMemberNotFound
	}
	return nil
}

func (r *Repository) ListMembers(ctx context.Context, workspaceID string, departmentID string) ([]departmentsdomain.Member, error) {
	rows, err := r.pool.Query(ctx, `
SELECT dm.department_id::text, dm.user_id::text, u.email::text, u.username::text, u.display_name, u.avatar_url, dm.role, dm.created_at
FROM department_members dm
JOIN departments d ON d.id = dm.department_id AND d.deleted_at IS NULL
JOIN users u ON u.id = dm.user_id AND u.deleted_at IS NULL
WHERE d.workspace_id = $1::uuid AND d.id = $2::uuid
ORDER BY u.display_name
`, workspaceID, departmentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var members []departmentsdomain.Member
	for rows.Next() {
		member, err := scanMember(rows)
		if err != nil {
			return nil, err
		}
		members = append(members, member)
	}
	return members, rows.Err()
}

func (r *Repository) RecordAudit(ctx context.Context, event departmentsapp.AuditEvent) error {
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

func (r *Repository) member(ctx context.Context, workspaceID string, departmentID string, userID string) (departmentsdomain.Member, error) {
	row := r.pool.QueryRow(ctx, `
SELECT dm.department_id::text, dm.user_id::text, u.email::text, u.username::text, u.display_name, u.avatar_url, dm.role, dm.created_at
FROM department_members dm
JOIN departments d ON d.id = dm.department_id AND d.deleted_at IS NULL
JOIN users u ON u.id = dm.user_id AND u.deleted_at IS NULL
WHERE d.workspace_id = $1::uuid AND d.id = $2::uuid AND dm.user_id = $3::uuid
`, workspaceID, departmentID, userID)
	return scanMember(row)
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanDepartment(row rowScanner) (departmentsdomain.Department, error) {
	var department departmentsdomain.Department
	var parentID sql.NullString
	var description sql.NullString
	var createdBy sql.NullString
	if err := row.Scan(
		&department.ID,
		&department.WorkspaceID,
		&parentID,
		&department.Slug,
		&department.Name,
		&description,
		&createdBy,
		&department.CreatedAt,
		&department.UpdatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return departmentsdomain.Department{}, departmentsdomain.ErrDepartmentNotFound
		}
		return departmentsdomain.Department{}, err
	}
	department.ParentID = nullStringPtr(parentID)
	department.Description = nullStringPtr(description)
	department.CreatedBy = nullStringPtr(createdBy)
	return department, nil
}

func scanMember(row rowScanner) (departmentsdomain.Member, error) {
	var member departmentsdomain.Member
	var avatarURL sql.NullString
	if err := row.Scan(
		&member.DepartmentID,
		&member.UserID,
		&member.Email,
		&member.Username,
		&member.DisplayName,
		&avatarURL,
		&member.Role,
		&member.CreatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return departmentsdomain.Member{}, departmentsdomain.ErrMemberNotFound
		}
		return departmentsdomain.Member{}, err
	}
	member.AvatarURL = nullStringPtr(avatarURL)
	return member, nil
}

func nullStringPtr(value sql.NullString) *string {
	if !value.Valid {
		return nil
	}
	return &value.String
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
