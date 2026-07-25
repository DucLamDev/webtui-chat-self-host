package domain

import (
	"errors"
	"time"
)

var ErrTicketNotFound = errors.New("ticket not found")

type Ticket struct {
	ID          string
	WorkspaceID string
	ChannelID   *string
	Title       string
	Description string
	Status      string
	Priority    string
	CreatedBy   *string
	AssignedTo  *string
	ResolvedAt  *time.Time
	ClosedAt    *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}
