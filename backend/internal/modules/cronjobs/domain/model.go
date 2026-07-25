package domain

import (
	"errors"
	"time"
)

var (
	ErrCronJobNotFound      = errors.New("không tìm thấy cronjob")
	ErrCronJobAlreadyExists = errors.New("cronjob đã tồn tại")
)

type CronJob struct {
	ID          string
	Name        string
	Description *string
	Schedule    string
	Runner      string
	Status      string
	Payload     []byte
	LastRunAt   *time.Time
	NextRunAt   *time.Time
	LockedAt    *time.Time
	LockedBy    *string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type CronJobRun struct {
	ID         string
	CronJobID  string
	Status     string
	StartedAt  time.Time
	FinishedAt *time.Time
	Log        *string
	Error      *string
}
