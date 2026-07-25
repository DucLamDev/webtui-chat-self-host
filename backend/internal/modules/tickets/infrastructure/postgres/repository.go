package postgres

import (
	"context"
	"database/sql"
	"errors"
	"time"

	ticketsapp "github.com/duclamdev/application-chat/backend/internal/modules/tickets/application"
	ticketsdomain "github.com/duclamdev/application-chat/backend/internal/modules/tickets/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) Create(ctx context.Context, params ticketsapp.SaveParams) (ticketsdomain.Ticket, error) {
	row := r.pool.QueryRow(ctx, `
INSERT INTO tickets (workspace_id, channel_id, title, description, status, priority, created_by, assigned_to)
SELECT $1::uuid, NULLIF($2, '')::uuid, $3, COALESCE($4, ''), $5, $6, NULLIF($7, '')::uuid, NULLIF($8, '')::uuid
WHERE ($2 = '' OR EXISTS (
    SELECT 1 FROM channels c
    WHERE c.id = NULLIF($2, '')::uuid
      AND c.workspace_id = $1::uuid
      AND c.deleted_at IS NULL
))
  AND ($8 = '' OR EXISTS (
    SELECT 1 FROM workspace_members wm
    WHERE wm.workspace_id = $1::uuid
      AND wm.user_id = NULLIF($8, '')::uuid
      AND wm.status IN ('active', 'muted')
))
RETURNING id::text, workspace_id::text, channel_id::text, title, description, status, priority,
          created_by::text, assigned_to::text, resolved_at, closed_at, created_at, updated_at
`, params.WorkspaceID, optionalString(params.ChannelID), requiredString(params.Title), optionalString(params.Description), requiredString(params.Status), requiredString(params.Priority), params.CreatedBy, optionalString(params.AssignedTo))
	return scanTicket(row)
}

func (r *Repository) Get(ctx context.Context, workspaceID string, ticketID string) (ticketsdomain.Ticket, error) {
	row := r.pool.QueryRow(ctx, `
SELECT id::text, workspace_id::text, channel_id::text, title, description, status, priority,
       created_by::text, assigned_to::text, resolved_at, closed_at, created_at, updated_at
FROM tickets
WHERE workspace_id = $1::uuid AND id = $2::uuid AND deleted_at IS NULL
`, workspaceID, ticketID)
	return scanTicket(row)
}

func (r *Repository) List(ctx context.Context, params ticketsapp.ListParams) ([]ticketsdomain.Ticket, error) {
	rows, err := r.pool.Query(ctx, `
SELECT id::text, workspace_id::text, channel_id::text, title, description, status, priority,
       created_by::text, assigned_to::text, resolved_at, closed_at, created_at, updated_at
FROM tickets
WHERE workspace_id = $1::uuid
  AND deleted_at IS NULL
  AND ($2 = '' OR status = $2)
ORDER BY
  CASE priority WHEN 'urgent' THEN 0 WHEN 'high' THEN 1 WHEN 'normal' THEN 2 ELSE 3 END,
  updated_at DESC
LIMIT $3
`, params.WorkspaceID, params.Status, params.Limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tickets := make([]ticketsdomain.Ticket, 0)
	for rows.Next() {
		ticket, err := scanTicket(rows)
		if err != nil {
			return nil, err
		}
		tickets = append(tickets, ticket)
	}
	return tickets, rows.Err()
}

func (r *Repository) Update(ctx context.Context, params ticketsapp.SaveParams) (ticketsdomain.Ticket, error) {
	row := r.pool.QueryRow(ctx, `
UPDATE tickets t
SET channel_id = CASE WHEN $3::text IS NULL THEN t.channel_id ELSE NULLIF($3::text, '')::uuid END,
    title = COALESCE($4::text, t.title),
    description = COALESCE($5::text, t.description),
    status = COALESCE($6::text, t.status),
    priority = COALESCE($7::text, t.priority),
    assigned_to = CASE WHEN $8::text IS NULL THEN t.assigned_to ELSE NULLIF($8::text, '')::uuid END,
    resolved_at = CASE
        WHEN $6::text IN ('resolved', 'closed') THEN COALESCE(t.resolved_at, now())
        WHEN $6::text IS NOT NULL THEN NULL
        ELSE t.resolved_at
    END,
    closed_at = CASE
        WHEN $6::text = 'closed' THEN COALESCE(t.closed_at, now())
        WHEN $6::text IS NOT NULL THEN NULL
        ELSE t.closed_at
    END
WHERE t.workspace_id = $1::uuid
  AND t.id = $2::uuid
  AND t.deleted_at IS NULL
  AND ($3::text IS NULL OR $3::text = '' OR EXISTS (
    SELECT 1 FROM channels c
    WHERE c.id = NULLIF($3::text, '')::uuid
      AND c.workspace_id = $1::uuid
      AND c.deleted_at IS NULL
  ))
  AND ($8::text IS NULL OR $8::text = '' OR EXISTS (
    SELECT 1 FROM workspace_members wm
    WHERE wm.workspace_id = $1::uuid
      AND wm.user_id = NULLIF($8::text, '')::uuid
      AND wm.status IN ('active', 'muted')
  ))
RETURNING id::text, workspace_id::text, channel_id::text, title, description, status, priority,
          created_by::text, assigned_to::text, resolved_at, closed_at, created_at, updated_at
`, params.WorkspaceID, params.TicketID, params.ChannelID, params.Title, params.Description, params.Status, params.Priority, params.AssignedTo)
	return scanTicket(row)
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanTicket(row rowScanner) (ticketsdomain.Ticket, error) {
	var ticket ticketsdomain.Ticket
	var channelID sql.NullString
	var createdBy sql.NullString
	var assignedTo sql.NullString
	var resolvedAt sql.NullTime
	var closedAt sql.NullTime
	if err := row.Scan(
		&ticket.ID,
		&ticket.WorkspaceID,
		&channelID,
		&ticket.Title,
		&ticket.Description,
		&ticket.Status,
		&ticket.Priority,
		&createdBy,
		&assignedTo,
		&resolvedAt,
		&closedAt,
		&ticket.CreatedAt,
		&ticket.UpdatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ticketsdomain.Ticket{}, ticketsdomain.ErrTicketNotFound
		}
		return ticketsdomain.Ticket{}, err
	}
	ticket.ChannelID = nullStringPtr(channelID)
	ticket.CreatedBy = nullStringPtr(createdBy)
	ticket.AssignedTo = nullStringPtr(assignedTo)
	ticket.ResolvedAt = nullTimePtr(resolvedAt)
	ticket.ClosedAt = nullTimePtr(closedAt)
	return ticket, nil
}

func optionalString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func requiredString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
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
