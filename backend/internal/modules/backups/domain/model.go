package domain

import (
	"errors"
	"time"
)

var ErrBackupJobNotFound = errors.New("không tìm thấy backup job")

type Job struct {
	ID          string
	WorkspaceID *string
	Name        string
	Target      string
	BackupType  string
	Schedule    *string
	Status      string
	Config      []byte
	LastRunAt   *time.Time
	NextRunAt   *time.Time
	LockedAt    *time.Time
	LockedBy    *string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type Run struct {
	ID          string
	BackupJobID *string
	Status      string
	BackupType  string
	ObjectKey   *string
	ByteSize    *int64
	Checksum    *string
	StartedAt   time.Time
	FinishedAt  *time.Time
	Error       *string
}
