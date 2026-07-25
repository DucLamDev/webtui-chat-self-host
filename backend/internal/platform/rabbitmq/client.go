package rabbitmq

import (
	"context"
	"fmt"

	"github.com/duclamdev/application-chat/backend/internal/config"
	amqp "github.com/rabbitmq/amqp091-go"
)

type Client struct {
	conn *amqp.Connection
}

func New(ctx context.Context, cfg config.RabbitMQConfig) (*Client, error) {
	if !cfg.Enabled {
		return nil, nil
	}

	conn, err := amqp.DialConfig(cfg.URL, amqp.Config{})
	if err != nil {
		return nil, fmt.Errorf("kết nối RabbitMQ: %w", err)
	}

	client := &Client{conn: conn}
	if err := client.Ping(ctx); err != nil {
		_ = conn.Close()
		return nil, err
	}

	return client, nil
}

func (c *Client) Connection() *amqp.Connection {
	return c.conn
}

func (c *Client) Ping(ctx context.Context) error {
	if c == nil || c.conn == nil {
		return ErrDisabled
	}
	if c.conn.IsClosed() {
		return fmt.Errorf("kết nối RabbitMQ đã đóng")
	}

	ch, err := c.conn.Channel()
	if err != nil {
		return fmt.Errorf("mở channel RabbitMQ: %w", err)
	}
	defer ch.Close()

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}

func (c *Client) Close() error {
	if c == nil || c.conn == nil {
		return nil
	}
	return c.conn.Close()
}
