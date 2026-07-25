package domain

import "time"

type Log struct {
	ID          string
	WorkspaceID *string
	ActorUserID *string
	Action      string
	EntityType  string
	EntityID    *string
	IPAddress   *string
	UserAgent   *string
	BeforeData  []byte
	AfterData   []byte
	Metadata    []byte
	CreatedAt   time.Time
}
