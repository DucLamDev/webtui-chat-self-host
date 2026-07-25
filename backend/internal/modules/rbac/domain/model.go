package domain

import "errors"

var (
	ErrRoleAlreadyExists = errors.New("role đã tồn tại")
	ErrRoleNotFound      = errors.New("không tìm thấy role")
)

type Permission struct {
	ID          string
	Code        string
	Module      string
	Action      string
	Name        string
	Description *string
}

type Role struct {
	ID          string
	WorkspaceID *string
	Code        string
	Name        string
	Description *string
	IsSystem    bool
	CreatedBy   *string
	Permissions []Permission
}
