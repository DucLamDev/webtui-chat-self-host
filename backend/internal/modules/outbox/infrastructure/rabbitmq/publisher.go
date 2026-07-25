package rabbitmq

import (
	"context"

	outboxdomain "github.com/duclamdev/application-chat/backend/internal/modules/outbox/domain"
	platformrabbit "github.com/duclamdev/application-chat/backend/internal/platform/rabbitmq"
)

type Publisher struct {
	client *platformrabbit.Client
}

func NewPublisher(client *platformrabbit.Client) *Publisher {
	return &Publisher{client: client}
}

func (p *Publisher) Publish(ctx context.Context, event outboxdomain.Event) error {
	if p == nil || p.client == nil {
		return nil
	}
	err := p.client.PublishEvent(ctx, platformrabbit.EventEnvelope{
		EventID:       event.ID,
		EventType:     event.EventType,
		EventVersion:  event.EventVersion,
		AggregateType: event.AggregateType,
		AggregateID:   event.AggregateID,
		Payload:       event.Payload,
		OccurredAt:    event.CreatedAt,
	})
	if platformrabbit.IsDisabled(err) {
		return nil
	}
	return err
}
