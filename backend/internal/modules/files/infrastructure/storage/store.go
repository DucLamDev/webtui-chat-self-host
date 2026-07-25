package storage

import (
	"context"

	filesapp "github.com/duclamdev/application-chat/backend/internal/modules/files/application"
	platformstorage "github.com/duclamdev/application-chat/backend/internal/platform/storage"
)

type Store struct {
	inner platformstorage.Store
}

func NewStore(inner platformstorage.Store) *Store {
	return &Store{inner: inner}
}

func (s *Store) Put(ctx context.Context, input filesapp.PutObjectInput) (filesapp.StoredObject, error) {
	object, err := s.inner.Put(ctx, platformstorage.PutObjectInput{
		Key:         input.Key,
		Body:        input.Body,
		ContentType: input.ContentType,
		Size:        input.Size,
	})
	if err != nil {
		return filesapp.StoredObject{}, err
	}
	return filesapp.StoredObject{
		Key:         object.Key,
		ContentType: object.ContentType,
		Size:        object.Size,
	}, nil
}

func (s *Store) Get(ctx context.Context, key string) (filesapp.StoredObjectReader, error) {
	object, err := s.inner.Get(ctx, key)
	if err != nil {
		return filesapp.StoredObjectReader{}, err
	}
	return filesapp.StoredObjectReader{
		Info: filesapp.StoredObject{
			Key:         object.Info.Key,
			ContentType: object.Info.ContentType,
			Size:        object.Info.Size,
		},
		Body: object.Body,
	}, nil
}

func (s *Store) Delete(ctx context.Context, key string) error {
	return s.inner.Delete(ctx, key)
}
