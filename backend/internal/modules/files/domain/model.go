package domain

import (
	"errors"
	"time"
)

var (
	ErrFileNotFound       = errors.New("không tìm thấy file")
	ErrMessageNotFound    = errors.New("không tìm thấy tin nhắn")
	ErrAttachmentNotFound = errors.New("không tìm thấy attachment")
)

type File struct {
	ID              string
	WorkspaceID     *string
	OwnerID         *string
	StorageProvider string
	Bucket          *string
	ObjectKey       string
	OriginalName    string
	MimeType        string
	ByteSize        int64
	ChecksumSHA256  *string
	Status          string
	Metadata        []byte
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type Version struct {
	ID              string
	FileID          string
	VersionNumber   int
	StorageProvider string
	Bucket          *string
	ObjectKey       string
	MimeType        string
	ByteSize        int64
	ChecksumSHA256  *string
	CreatedBy       *string
	CreatedAt       time.Time
}

type Attachment struct {
	WorkspaceID string
	MessageID   string
	FileID      string
	SortOrder   int
	CreatedAt   time.Time
	File        File
}
