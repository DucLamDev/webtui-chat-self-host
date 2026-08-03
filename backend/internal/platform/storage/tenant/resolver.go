package tenant

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/duclamdev/application-chat/backend/internal/config"
	tenancyapp "github.com/duclamdev/application-chat/backend/internal/modules/tenancy/application"
	platformstorage "github.com/duclamdev/application-chat/backend/internal/platform/storage"
	localstorage "github.com/duclamdev/application-chat/backend/internal/platform/storage/local"
	miniostorage "github.com/duclamdev/application-chat/backend/internal/platform/storage/minio"
	"github.com/duclamdev/application-chat/backend/internal/shared/securevalue"
)

type Location struct {
	Store    platformstorage.Store
	Provider string
	Bucket   string
}

type cachedLocation struct {
	version  time.Time
	location Location
}

type Resolver struct {
	pool         *pgxpool.Pool
	local        platformstorage.Store
	masterSecret string
	mu           sync.RWMutex
	cache        map[string]cachedLocation
}

func New(pool *pgxpool.Pool, localPath string, masterSecret string) (*Resolver, error) {
	store, err := localstorage.New(localPath)
	if err != nil {
		return nil, err
	}
	return &Resolver{
		pool: pool, local: store, masterSecret: masterSecret,
		cache: make(map[string]cachedLocation),
	}, nil
}

func (r *Resolver) ResolveWorkspace(ctx context.Context, workspaceID string) (Location, error) {
	var zoneID string
	err := r.pool.QueryRow(ctx, `
SELECT COALESCE(zone_id::text, '')
FROM workspaces
WHERE id = $1::uuid AND deleted_at IS NULL
`, strings.TrimSpace(workspaceID)).Scan(&zoneID)
	if errors.Is(err, pgx.ErrNoRows) {
		return Location{}, errors.New("không tìm thấy workspace để xác định storage")
	}
	if err != nil {
		return Location{}, err
	}
	if zoneID == "" {
		return Location{Store: r.local, Provider: "local"}, nil
	}
	return r.ResolveZone(ctx, zoneID)
}

func (r *Resolver) ResolveZone(ctx context.Context, zoneID string) (Location, error) {
	zoneID = strings.TrimSpace(zoneID)
	if zoneID == "" {
		return Location{}, errors.New("zone id là bắt buộc để xác định storage")
	}

	var provider, endpoint, region, bucket, accessKeyID, secretEncrypted string
	var updatedAt time.Time
	err := r.pool.QueryRow(ctx, `
SELECT provider, COALESCE(endpoint, ''), COALESCE(region, ''), COALESCE(bucket, ''),
       COALESCE(access_key_id, ''), COALESCE(secret_access_key_encrypted, ''), updated_at
FROM zone_storage_configs
WHERE zone_id = $1::uuid
`, zoneID).Scan(
		&provider, &endpoint, &region, &bucket, &accessKeyID, &secretEncrypted, &updatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Location{Store: r.local, Provider: "local"}, nil
	}
	if err != nil {
		return Location{}, err
	}

	r.mu.RLock()
	cached, ok := r.cache[zoneID]
	r.mu.RUnlock()
	if ok && cached.version.Equal(updatedAt) {
		return cached.location, nil
	}

	provider = strings.ToLower(strings.TrimSpace(provider))
	var location Location
	switch provider {
	case "local", "":
		location = Location{Store: r.local, Provider: "local"}
	case "minio", "s3":
		secret, decryptErr := securevalue.Decrypt(
			r.masterSecret,
			secretEncrypted,
			"vpsttt:zone-storage:"+zoneID,
		)
		if decryptErr != nil {
			return Location{}, fmt.Errorf("giải mã Secret Key storage của zone: %w", decryptErr)
		}
		store, createErr := miniostorage.New(config.StorageConfig{
			Provider: provider, Endpoint: endpoint, Region: region, Bucket: bucket,
			AccessKeyID: accessKeyID, SecretAccessKey: secret,
		})
		if createErr != nil {
			return Location{}, createErr
		}
		location = Location{Store: store, Provider: provider, Bucket: bucket}
	default:
		return Location{}, fmt.Errorf("nhà cung cấp storage %q không được hỗ trợ", provider)
	}

	r.mu.Lock()
	r.cache[zoneID] = cachedLocation{version: updatedAt, location: location}
	r.mu.Unlock()
	return location, nil
}

func (r *Resolver) Test(ctx context.Context, input tenancyapp.ZoneStorageConnectionConfig) error {
	if input.Provider == "local" {
		return r.local.Health(ctx)
	}
	store, err := miniostorage.New(config.StorageConfig{
		Provider: input.Provider, Endpoint: input.Endpoint, Region: input.Region,
		Bucket: input.Bucket, AccessKeyID: input.AccessKeyID,
		SecretAccessKey: input.SecretAccessKey,
	})
	if err != nil {
		return err
	}
	return store.Health(ctx)
}

func (r *Resolver) Invalidate(zoneID string) {
	r.mu.Lock()
	delete(r.cache, strings.TrimSpace(zoneID))
	r.mu.Unlock()
}
