package postgres

import (
	"context"
	"database/sql"
	"errors"
	"time"

	aptokensapp "github.com/duclamdev/application-chat/backend/internal/modules/api_tokens/application"
	aptokensdomain "github.com/duclamdev/application-chat/backend/internal/modules/api_tokens/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) ListScopes(ctx context.Context) ([]aptokensdomain.Scope, error) {
	rows, err := r.pool.Query(ctx, `
SELECT id::text, code::text, name, description, module, action, created_at
FROM api_scopes
ORDER BY module, action, code
`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	scopes := make([]aptokensdomain.Scope, 0)
	for rows.Next() {
		scope, err := scanScope(rows)
		if err != nil {
			return nil, err
		}
		scopes = append(scopes, scope)
	}
	return scopes, rows.Err()
}

func (r *Repository) ListTokens(ctx context.Context, workspaceID string) ([]aptokensdomain.Token, error) {
	rows, err := r.pool.Query(ctx, `
SELECT id::text, workspace_id::text, owner_id::text, name, status, last_used_at, expires_at, created_at, updated_at, revoked_at
FROM api_tokens
WHERE workspace_id = $1::uuid
ORDER BY created_at DESC
`, workspaceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tokens := make([]aptokensdomain.Token, 0)
	for rows.Next() {
		token, err := scanToken(rows)
		if err != nil {
			return nil, err
		}
		token.Scopes, err = r.scopesByTokenID(ctx, token.ID)
		if err != nil {
			return nil, err
		}
		tokens = append(tokens, token)
	}
	return tokens, rows.Err()
}

func (r *Repository) CreateToken(ctx context.Context, params aptokensapp.CreateTokenParams) (aptokensdomain.Token, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return aptokensdomain.Token{}, err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	row := tx.QueryRow(ctx, `
INSERT INTO api_tokens (workspace_id, owner_id, name, token_hash, expires_at)
VALUES ($1::uuid, $2::uuid, $3, $4, $5)
RETURNING id::text, workspace_id::text, owner_id::text, name, status, last_used_at, expires_at, created_at, updated_at, revoked_at
`, params.WorkspaceID, params.OwnerID, params.Name, params.TokenHash, params.ExpiresAt)
	token, err := scanToken(row)
	if err != nil {
		return aptokensdomain.Token{}, err
	}

	for _, code := range params.ScopeCodes {
		command, err := tx.Exec(ctx, `
INSERT INTO api_token_scopes (api_token_id, api_scope_id)
SELECT $1::uuid, id
FROM api_scopes
WHERE code = $2
ON CONFLICT DO NOTHING
`, token.ID, code)
		if err != nil {
			return aptokensdomain.Token{}, err
		}
		if command.RowsAffected() == 0 {
			return aptokensdomain.Token{}, aptokensdomain.ErrScopeNotFound
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return aptokensdomain.Token{}, err
	}
	token.Scopes, err = r.scopesByTokenID(ctx, token.ID)
	if err != nil {
		return aptokensdomain.Token{}, err
	}
	return token, nil
}

func (r *Repository) RevokeToken(ctx context.Context, workspaceID string, tokenID string) error {
	command, err := r.pool.Exec(ctx, `
UPDATE api_tokens
SET status = 'revoked',
    revoked_at = COALESCE(revoked_at, now())
WHERE workspace_id = $1::uuid
  AND id = $2::uuid
  AND status = 'active'
`, workspaceID, tokenID)
	if err != nil {
		return err
	}
	if command.RowsAffected() == 0 {
		return aptokensdomain.ErrTokenNotFound
	}
	return nil
}

func (r *Repository) Authenticate(ctx context.Context, tokenHash string) (aptokensdomain.AuthenticatedToken, error) {
	row := r.pool.QueryRow(ctx, `
UPDATE api_tokens token
SET last_used_at = now()
FROM workspaces workspace
JOIN zones zone
  ON zone.id = workspace.zone_id
 AND zone.status = 'active'
 AND zone.deleted_at IS NULL
WHERE token.token_hash = $1
  AND token.status = 'active'
  AND (token.expires_at IS NULL OR token.expires_at > now())
  AND workspace.id = token.workspace_id
  AND workspace.status = 'active'
  AND workspace.deleted_at IS NULL
RETURNING token.id::text, zone.id::text, token.workspace_id::text, token.owner_id::text
`, tokenHash)
	var authenticated aptokensdomain.AuthenticatedToken
	var ownerID sql.NullString
	if err := row.Scan(&authenticated.TokenID, &authenticated.ZoneID, &authenticated.WorkspaceID, &ownerID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return aptokensdomain.AuthenticatedToken{}, aptokensdomain.ErrTokenInactive
		}
		return aptokensdomain.AuthenticatedToken{}, err
	}
	if ownerID.Valid {
		authenticated.OwnerID = &ownerID.String
	}
	scopes, err := r.scopesByTokenID(ctx, authenticated.TokenID)
	if err != nil {
		return aptokensdomain.AuthenticatedToken{}, err
	}
	authenticated.Scopes = make([]string, 0, len(scopes))
	for _, scope := range scopes {
		authenticated.Scopes = append(authenticated.Scopes, scope.Code)
	}
	return authenticated, nil
}

func (r *Repository) scopesByTokenID(ctx context.Context, tokenID string) ([]aptokensdomain.Scope, error) {
	rows, err := r.pool.Query(ctx, `
SELECT s.id::text, s.code::text, s.name, s.description, s.module, s.action, s.created_at
FROM api_token_scopes ts
JOIN api_scopes s ON s.id = ts.api_scope_id
WHERE ts.api_token_id = $1::uuid
ORDER BY s.module, s.action, s.code
`, tokenID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	scopes := make([]aptokensdomain.Scope, 0)
	for rows.Next() {
		scope, err := scanScope(rows)
		if err != nil {
			return nil, err
		}
		scopes = append(scopes, scope)
	}
	return scopes, rows.Err()
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanToken(row rowScanner) (aptokensdomain.Token, error) {
	var token aptokensdomain.Token
	var ownerID sql.NullString
	var lastUsedAt sql.NullTime
	var expiresAt sql.NullTime
	var revokedAt sql.NullTime
	if err := row.Scan(
		&token.ID,
		&token.WorkspaceID,
		&ownerID,
		&token.Name,
		&token.Status,
		&lastUsedAt,
		&expiresAt,
		&token.CreatedAt,
		&token.UpdatedAt,
		&revokedAt,
	); err != nil {
		return aptokensdomain.Token{}, err
	}
	token.OwnerID = nullStringPtr(ownerID)
	token.LastUsedAt = nullTimePtr(lastUsedAt)
	token.ExpiresAt = nullTimePtr(expiresAt)
	token.RevokedAt = nullTimePtr(revokedAt)
	return token, nil
}

func scanScope(row rowScanner) (aptokensdomain.Scope, error) {
	var scope aptokensdomain.Scope
	var description sql.NullString
	if err := row.Scan(
		&scope.ID,
		&scope.Code,
		&scope.Name,
		&description,
		&scope.Module,
		&scope.Action,
		&scope.CreatedAt,
	); err != nil {
		return aptokensdomain.Scope{}, err
	}
	scope.Description = nullStringPtr(description)
	return scope, nil
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
