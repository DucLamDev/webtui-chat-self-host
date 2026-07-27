package storage

import (
	"context"
	"io"
)

type ObjectInfo struct {
	Key         string
	ContentType string
	Size        int64
}

type PutObjectInput struct {
	Key         string
	Body        io.Reader
	ContentType string
	Size        int64
}

type GetObjectOutput struct {
	Info ObjectInfo
	Body io.ReadCloser
}

type Store interface {
	Put(ctx context.Context, input PutObjectInput) (ObjectInfo, error)
	Get(ctx context.Context, key string) (*GetObjectOutput, error)
	Delete(ctx context.Context, key string) error
	Health(ctx context.Context) error
}

// RangeStore is an optional capability for efficient media streaming.
type RangeStore interface {
	GetRange(ctx context.Context, key string, start int64, end int64) (*GetObjectOutput, error)
}
