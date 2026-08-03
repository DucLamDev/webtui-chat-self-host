package storage

import (
	"context"

	filesapp "github.com/duclamdev/application-chat/backend/internal/modules/files/application"
	tenantstorage "github.com/duclamdev/application-chat/backend/internal/platform/storage/tenant"
)

type Resolver struct {
	inner *tenantstorage.Resolver
}

func NewResolver(inner *tenantstorage.Resolver) *Resolver {
	return &Resolver{inner: inner}
}

func (r *Resolver) ResolveWorkspace(ctx context.Context, workspaceID string) (filesapp.StorageLocation, error) {
	location, err := r.inner.ResolveWorkspace(ctx, workspaceID)
	if err != nil {
		return filesapp.StorageLocation{}, err
	}
	return filesapp.StorageLocation{
		Store:    NewStore(location.Store),
		Provider: location.Provider,
		Bucket:   location.Bucket,
	}, nil
}
