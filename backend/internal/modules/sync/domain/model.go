package domain

import (
	"encoding/json"
	"time"
)

type Event struct {
	ID            string
	AggregateType string
	AggregateID   string
	EventType     string
	EventVersion  int
	Payload       json.RawMessage
	CreatedAt     time.Time
}

type CursorAck struct {
	UserID          string
	WorkspaceID     string
	DeviceID        string
	CursorEventID   *string
	CursorCreatedAt *time.Time
	AckedAt         time.Time
}
