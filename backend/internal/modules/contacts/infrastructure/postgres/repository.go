package postgres

import (
	"context"
	"database/sql"
	"errors"
	"time"

	contactsdomain "github.com/duclamdev/application-chat/backend/internal/modules/contacts/domain"
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

func (r *Repository) ListContacts(ctx context.Context, zoneID string, actorUserID string) ([]contactsdomain.ContactRequest, error) {
	return r.list(ctx, zoneID, actorUserID, "accepted")
}

func (r *Repository) ListRequests(ctx context.Context, zoneID string, actorUserID string, status string) ([]contactsdomain.ContactRequest, error) {
	if status == "all" {
		status = ""
	}
	return r.list(ctx, zoneID, actorUserID, status)
}

func (r *Repository) CreateRequest(ctx context.Context, zoneID string, actorUserID string, receiverID string) (contactsdomain.ContactRequest, error) {
	if actorUserID == receiverID {
		return contactsdomain.ContactRequest{}, contactsdomain.ErrCannotContactSelf
	}
	userExists, err := r.userExistsInZone(ctx, zoneID, receiverID)
	if err != nil {
		return contactsdomain.ContactRequest{}, err
	}
	if !userExists {
		return contactsdomain.ContactRequest{}, contactsdomain.ErrUserNotFound
	}

	row := r.pool.QueryRow(ctx, `
INSERT INTO contact_requests (zone_id, requester_id, receiver_id, status)
VALUES ($1::uuid, $2::uuid, $3::uuid, 'pending')
RETURNING id::text
`, zoneID, actorUserID, receiverID)
	var requestID string
	if err := row.Scan(&requestID); err != nil {
		if isUniqueViolation(err) {
			return r.byPair(ctx, zoneID, actorUserID, receiverID, actorUserID)
		}
		return contactsdomain.ContactRequest{}, err
	}
	return r.byID(ctx, zoneID, requestID, actorUserID)
}

func (r *Repository) AcceptRequest(ctx context.Context, zoneID string, actorUserID string, requestID string) (contactsdomain.ContactRequest, error) {
	command, err := r.pool.Exec(ctx, `
UPDATE contact_requests
SET status = 'accepted',
    responded_at = now()
WHERE id = $1::uuid
  AND receiver_id = $2::uuid
  AND zone_id = $3::uuid
  AND status = 'pending'
  AND deleted_at IS NULL
`, requestID, actorUserID, zoneID)
	if err != nil {
		return contactsdomain.ContactRequest{}, err
	}
	if command.RowsAffected() == 0 {
		return contactsdomain.ContactRequest{}, contactsdomain.ErrContactRequestNotFound
	}
	return r.byID(ctx, zoneID, requestID, actorUserID)
}

func (r *Repository) RejectRequest(ctx context.Context, zoneID string, actorUserID string, requestID string) (contactsdomain.ContactRequest, error) {
	command, err := r.pool.Exec(ctx, `
UPDATE contact_requests
SET status = 'rejected',
    responded_at = now()
WHERE id = $1::uuid
  AND receiver_id = $2::uuid
  AND zone_id = $3::uuid
  AND status = 'pending'
  AND deleted_at IS NULL
`, requestID, actorUserID, zoneID)
	if err != nil {
		return contactsdomain.ContactRequest{}, err
	}
	if command.RowsAffected() == 0 {
		return contactsdomain.ContactRequest{}, contactsdomain.ErrContactRequestNotFound
	}
	return r.byID(ctx, zoneID, requestID, actorUserID)
}

func (r *Repository) CancelRequest(ctx context.Context, zoneID string, actorUserID string, requestID string) (contactsdomain.ContactRequest, error) {
	command, err := r.pool.Exec(ctx, `
UPDATE contact_requests
SET status = 'cancelled',
    deleted_at = now()
WHERE id = $1::uuid
  AND requester_id = $2::uuid
  AND zone_id = $3::uuid
  AND status = 'pending'
  AND deleted_at IS NULL
`, requestID, actorUserID, zoneID)
	if err != nil {
		return contactsdomain.ContactRequest{}, err
	}
	if command.RowsAffected() == 0 {
		return contactsdomain.ContactRequest{}, contactsdomain.ErrContactRequestNotFound
	}
	row := r.pool.QueryRow(ctx, `
SELECT cr.id::text, cr.requester_id::text, cr.receiver_id::text, cr.status,
       other_user.id::text, other_user.email::text, other_user.username::text, other_user.display_name,
       other_user.avatar_url, other_user.phone_number, other_user.status,
       cr.requested_at, cr.responded_at, cr.created_at, cr.updated_at
FROM contact_requests cr
JOIN users other_user
  ON other_user.id = CASE
      WHEN cr.requester_id = $2::uuid THEN cr.receiver_id
      ELSE cr.requester_id
  END
 AND other_user.deleted_at IS NULL
WHERE cr.id = $1::uuid
  AND (cr.requester_id = $2::uuid OR cr.receiver_id = $2::uuid)
  AND cr.zone_id = $3::uuid
`, requestID, actorUserID, zoneID)
	return scanContactRequest(row)
}

func (r *Repository) list(ctx context.Context, zoneID string, actorUserID string, status string) ([]contactsdomain.ContactRequest, error) {
	rows, err := r.pool.Query(ctx, `
SELECT cr.id::text, cr.requester_id::text, cr.receiver_id::text, cr.status,
       other_user.id::text, other_user.email::text, other_user.username::text, other_user.display_name,
       other_user.avatar_url, other_user.phone_number, other_user.status,
       cr.requested_at, cr.responded_at, cr.created_at, cr.updated_at
FROM contact_requests cr
JOIN users other_user
  ON other_user.id = CASE
      WHEN cr.requester_id = $1::uuid THEN cr.receiver_id
      ELSE cr.requester_id
  END
 AND other_user.deleted_at IS NULL
WHERE (cr.requester_id = $1::uuid OR cr.receiver_id = $1::uuid)
  AND cr.zone_id = $2::uuid
  AND cr.deleted_at IS NULL
  AND ($3 = '' OR cr.status = $3)
ORDER BY cr.updated_at DESC
`, actorUserID, zoneID, status)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]contactsdomain.ContactRequest, 0)
	for rows.Next() {
		item, err := scanContactRequest(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *Repository) byPair(ctx context.Context, zoneID string, leftUserID string, rightUserID string, actorUserID string) (contactsdomain.ContactRequest, error) {
	row := r.pool.QueryRow(ctx, `
SELECT cr.id::text, cr.requester_id::text, cr.receiver_id::text, cr.status,
       other_user.id::text, other_user.email::text, other_user.username::text, other_user.display_name,
       other_user.avatar_url, other_user.phone_number, other_user.status,
       cr.requested_at, cr.responded_at, cr.created_at, cr.updated_at
FROM contact_requests cr
JOIN users other_user
  ON other_user.id = CASE
      WHEN cr.requester_id = $4::uuid THEN cr.receiver_id
      ELSE cr.requester_id
  END
 AND other_user.deleted_at IS NULL
WHERE cr.deleted_at IS NULL
  AND cr.zone_id = $1::uuid
  AND cr.status IN ('pending', 'accepted')
  AND LEAST(cr.requester_id, cr.receiver_id) = LEAST($2::uuid, $3::uuid)
  AND GREATEST(cr.requester_id, cr.receiver_id) = GREATEST($2::uuid, $3::uuid)
`, zoneID, leftUserID, rightUserID, actorUserID)
	return scanContactRequest(row)
}

func (r *Repository) byID(ctx context.Context, zoneID string, requestID string, actorUserID string) (contactsdomain.ContactRequest, error) {
	row := r.pool.QueryRow(ctx, `
SELECT cr.id::text, cr.requester_id::text, cr.receiver_id::text, cr.status,
       other_user.id::text, other_user.email::text, other_user.username::text, other_user.display_name,
       other_user.avatar_url, other_user.phone_number, other_user.status,
       cr.requested_at, cr.responded_at, cr.created_at, cr.updated_at
FROM contact_requests cr
JOIN users other_user
  ON other_user.id = CASE
      WHEN cr.requester_id = $2::uuid THEN cr.receiver_id
      ELSE cr.requester_id
  END
 AND other_user.deleted_at IS NULL
WHERE cr.id = $1::uuid
  AND (cr.requester_id = $2::uuid OR cr.receiver_id = $2::uuid)
  AND cr.zone_id = $3::uuid
  AND cr.deleted_at IS NULL
`, requestID, actorUserID, zoneID)
	return scanContactRequest(row)
}

func (r *Repository) userExistsInZone(ctx context.Context, zoneID string, userID string) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx, `
SELECT EXISTS (
    SELECT 1
    FROM users u
    JOIN workspace_members wm
      ON wm.user_id = u.id
     AND wm.status = 'active'
    JOIN workspaces w
      ON w.id = wm.workspace_id
     AND w.zone_id = $1::uuid
     AND w.status = 'active'
     AND w.deleted_at IS NULL
    WHERE u.id = $2::uuid
      AND u.deleted_at IS NULL
      AND u.status = 'active'
)
`, zoneID, userID).Scan(&exists)
	return exists, err
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanContactRequest(row rowScanner) (contactsdomain.ContactRequest, error) {
	var item contactsdomain.ContactRequest
	var avatarURL sql.NullString
	var phoneNumber sql.NullString
	var respondedAt sql.NullTime
	if err := row.Scan(
		&item.ID,
		&item.RequesterID,
		&item.ReceiverID,
		&item.Status,
		&item.User.ID,
		&item.User.Email,
		&item.User.Username,
		&item.User.DisplayName,
		&avatarURL,
		&phoneNumber,
		&item.User.Status,
		&item.RequestedAt,
		&respondedAt,
		&item.CreatedAt,
		&item.UpdatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return contactsdomain.ContactRequest{}, contactsdomain.ErrContactRequestNotFound
		}
		return contactsdomain.ContactRequest{}, err
	}
	item.User.AvatarURL = nullStringPtr(avatarURL)
	item.User.PhoneNumber = nullStringPtr(phoneNumber)
	item.RespondedAt = nullTimePtr(respondedAt)
	return item, nil
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
