package bootstrap

import (
	"context"
	"errors"
	"log/slog"
	"strings"

	"github.com/duclamdev/application-chat/backend/internal/config"
	"github.com/duclamdev/application-chat/backend/internal/platform/database"
	"github.com/duclamdev/application-chat/backend/internal/platform/rabbitmq"
	"github.com/duclamdev/application-chat/backend/internal/platform/redis"
	"github.com/duclamdev/application-chat/backend/internal/platform/storage"
	"github.com/duclamdev/application-chat/backend/internal/platform/storage/local"
	"github.com/duclamdev/application-chat/backend/internal/platform/storage/minio"
	"github.com/duclamdev/application-chat/backend/internal/platform/websocket"
)

type Resources struct {
	Database  *database.Postgres
	Redis     *redis.Client
	RabbitMQ  *rabbitmq.Client
	Storage   storage.Store
	WebSocket *websocket.Manager
}

func NewResources(ctx context.Context, cfg *config.Config) (*Resources, error) {
	db, err := database.NewPostgres(ctx, cfg.Database)
	if err != nil {
		return nil, err
	}

	redisClient, err := redis.New(ctx, cfg.Redis)
	if err != nil {
		if db != nil {
			db.Close()
		}
		return nil, err
	}

	rabbitClient, err := rabbitmq.New(ctx, cfg.RabbitMQ)
	if err != nil {
		if redisClient != nil {
			_ = redisClient.Close()
		}
		if db != nil {
			db.Close()
		}
		return nil, err
	}

	store, err := newStorage(cfg.Storage)
	if err != nil {
		if rabbitClient != nil {
			_ = rabbitClient.Close()
		}
		if redisClient != nil {
			_ = redisClient.Close()
		}
		if db != nil {
			db.Close()
		}
		return nil, err
	}

	return &Resources{
		Database:  db,
		Redis:     redisClient,
		RabbitMQ:  rabbitClient,
		Storage:   store,
		WebSocket: websocket.NewManager(),
	}, nil
}

func newStorage(cfg config.StorageConfig) (storage.Store, error) {
	switch strings.ToLower(strings.TrimSpace(cfg.Provider)) {
	case "local", "":
		return local.New(cfg.LocalPath)
	case "minio", "s3":
		return minio.New(cfg)
	default:
		return nil, errors.New("nhà cung cấp lưu trữ không được hỗ trợ")
	}
}

func (r *Resources) Close() {
	if r == nil {
		return
	}
	if r.RabbitMQ != nil {
		if err := r.RabbitMQ.Close(); err != nil {
			slog.Warn("Đóng RabbitMQ bị lỗi", "error", err)
		}
	}
	if r.Redis != nil {
		if err := r.Redis.Close(); err != nil {
			slog.Warn("Đóng Redis bị lỗi", "error", err)
		}
	}
	if r.Database != nil {
		r.Database.Close()
	}
}
