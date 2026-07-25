package database

import (
	"context"
	"fmt"
	"time"

	"github.com/duclamdev/application-chat/backend/internal/config"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Postgres struct {
	pool *pgxpool.Pool
}

func NewPostgres(ctx context.Context, cfg config.DatabaseConfig) (*Postgres, error) {
	if !cfg.Enabled {
		return nil, nil
	}

	poolConfig, err := pgxpool.ParseConfig(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("phân tích cấu hình cơ sở dữ liệu: %w", err)
	}

	poolConfig.MaxConns = 20
	poolConfig.MinConns = 2
	poolConfig.MaxConnLifetime = time.Hour
	poolConfig.MaxConnIdleTime = 30 * time.Minute
	poolConfig.HealthCheckPeriod = time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("kết nối cơ sở dữ liệu: %w", err)
	}

	db := &Postgres{pool: pool}
	if err := db.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}

	return db, nil
}

func (p *Postgres) Pool() *pgxpool.Pool {
	return p.pool
}

func (p *Postgres) Ping(ctx context.Context) error {
	if p == nil || p.pool == nil {
		return ErrDisabled
	}
	if err := p.pool.Ping(ctx); err != nil {
		return fmt.Errorf("kiểm tra kết nối cơ sở dữ liệu: %w", err)
	}
	return nil
}

func (p *Postgres) Close() {
	if p == nil || p.pool == nil {
		return
	}
	p.pool.Close()
}

func (p *Postgres) Tx(ctx context.Context, fn func(context.Context, pgx.Tx) error) error {
	if p == nil || p.pool == nil {
		return ErrDisabled
	}

	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("bắt đầu giao dịch: %w", err)
	}

	defer func() {
		_ = tx.Rollback(ctx)
	}()

	if err := fn(ctx, tx); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("xác nhận giao dịch: %w", err)
	}

	return nil
}
