package rabbitmq

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

const EventExchange = "webtui.events"

type EventEnvelope struct {
	EventID       string          `json:"event_id"`
	EventType     string          `json:"event_type"`
	EventVersion  int             `json:"event_version"`
	AggregateType string          `json:"aggregate_type"`
	AggregateID   string          `json:"aggregate_id"`
	Payload       json.RawMessage `json:"payload"`
	OccurredAt    time.Time       `json:"occurred_at"`
}

func (c *Client) PublishEvent(ctx context.Context, envelope EventEnvelope) error {
	if c == nil || c.conn == nil {
		return ErrDisabled
	}
	channel, err := c.conn.Channel()
	if err != nil {
		return err
	}
	defer channel.Close()

	if err := channel.ExchangeDeclare(EventExchange, "topic", true, false, false, false, nil); err != nil {
		return err
	}
	body, err := json.Marshal(envelope)
	if err != nil {
		return err
	}
	routingKey := envelope.AggregateType + "." + envelope.EventType
	return channel.PublishWithContext(ctx, EventExchange, routingKey, false, false, amqp.Publishing{
		ContentType:  "application/json",
		DeliveryMode: amqp.Persistent,
		Timestamp:    time.Now().UTC(),
		Type:         envelope.EventType,
		MessageId:    envelope.EventID,
		Body:         body,
	})
}

func IsDisabled(err error) bool {
	return errors.Is(err, ErrDisabled)
}
