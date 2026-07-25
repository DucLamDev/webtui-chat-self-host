package domain

import (
	"errors"
	"time"
)

var (
	ErrTokenNotFound = errors.New("không tìm thấy API token")
	ErrScopeNotFound = errors.New("không tìm thấy API scope")
	ErrTokenInactive = errors.New("API token không còn hoạt động")
)

type Token struct {
	ID          string
	WorkspaceID string
	OwnerID     *string
	Name        string
	Status      string
	LastUsedAt  *time.Time
	ExpiresAt   *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
	RevokedAt   *time.Time
	Scopes      []Scope
}

type Scope struct {
	ID          string
	Code        string
	Name        string
	Description *string
	Module      string
	Action      string
	CreatedAt   time.Time
}

type AuthenticatedToken struct {
	TokenID     string
	ZoneID      string
	WorkspaceID string
	OwnerID     *string
	Scopes      []string
}
