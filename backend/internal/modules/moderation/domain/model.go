package domain

import (
	"encoding/json"
	"errors"
	"time"
)

var (
	ErrReportNotFound   = errors.New("moderation report not found")
	ErrReportDuplicate  = errors.New("an active report already exists for this target")
	ErrReportTarget     = errors.New("report target not found or not visible")
	ErrWorkspaceMember  = errors.New("active workspace member not found")
	ErrReportRateLimit  = errors.New("report submission rate limit exceeded")
	ErrBlockNotFound    = errors.New("user block not found")
	ErrInvalidBlockPair = errors.New("invalid user block pair")
)

type Report struct {
	ID                    string
	WorkspaceID           string
	ReporterUserID        *string
	ReporterDisplayName   *string
	TargetType            string
	TargetID              string
	TargetUserID          *string
	TargetUserDisplayName *string
	TargetSnapshot        json.RawMessage
	Reason                string
	Details               *string
	Status                string
	ResolutionNote        *string
	ResolvedBy            *string
	ResolvedAt            *time.Time
	CreatedAt             time.Time
	UpdatedAt             time.Time
}

type ReportTarget struct {
	UserID   string
	Snapshot json.RawMessage
}

type UserBlock struct {
	ID                 string
	WorkspaceID        string
	BlockerUserID      string
	BlockedUserID      string
	BlockedUsername    string
	BlockedDisplayName string
	BlockedAvatarURL   *string
	Reason             *string
	CreatedAt          time.Time
	UpdatedAt          time.Time
}
