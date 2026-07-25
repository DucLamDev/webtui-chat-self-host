package redis

import (
	"context"
	"fmt"

	"github.com/duclamdev/application-chat/backend/internal/config"
	goredis "github.com/redis/go-redis/v9"
)

type Client struct {
	client *goredis.Client
}

func New(ctx context.Context, cfg config.RedisConfig) (*Client, error) {
	if !cfg.Enabled {
		return nil, nil
	}

	options, err := goredis.ParseURL(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("phân tích Redis URL: %w", err)
	}

	client := goredis.NewClient(options)
	r := &Client{client: client}
	if err := r.Ping(ctx); err != nil {
		_ = client.Close()
		return nil, err
	}

	return r, nil
}

func (c *Client) Raw() *goredis.Client {
	return c.client
}

func (c *Client) Ping(ctx context.Context) error {
	if c == nil || c.client == nil {
		return ErrDisabled
	}
	if err := c.client.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("kiểm tra kết nối Redis: %w", err)
	}
	return nil
}

func (c *Client) Close() error {
	if c == nil || c.client == nil {
		return nil
	}
	return c.client.Close()
}
