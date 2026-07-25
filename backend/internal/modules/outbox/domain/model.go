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
	Status        string
	RetryCount    int
	CreatedAt     time.Time
}
