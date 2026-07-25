package application

import (
	"context"
	"log/slog"
	"time"

	outboxdomain "github.com/duclamdev/application-chat/backend/internal/modules/outbox/domain"
)

const maxRetries = 5

type Repository interface {
	Claim(ctx context.Context, limit int) ([]outboxdomain.Event, error)
	MarkPublished(ctx context.Context, eventID string) error
	MarkFailed(ctx context.Context, eventID string, retryAfter time.Duration, maxRetries int, reason string) error
}

type Publisher interface {
	Publish(ctx context.Context, event outboxdomain.Event) error
}

type Handler interface {
	Handle(ctx context.Context, event outboxdomain.Event) error
}

type Service struct {
	repo      Repository
	publisher Publisher
	handlers  []Handler
}

func NewService(repo Repository, publisher Publisher, handlers ...Handler) *Service {
	return &Service{repo: repo, publisher: publisher, handlers: handlers}
}

func (s *Service) Process(ctx context.Context, limit int) (int, error) {
	if limit <= 0 {
		limit = 20
	}
	events, err := s.repo.Claim(ctx, limit)
	if err != nil {
		return 0, err
	}
	for _, event := range events {
		if err := s.processEvent(ctx, event); err != nil {
			slog.Warn("Xử lý outbox event thất bại", "event_id", event.ID, "event_type", event.EventType, "error", err)
			_ = s.repo.MarkFailed(ctx, event.ID, retryDelay(event.RetryCount), maxRetries, err.Error())
			continue
		}
		if err := s.repo.MarkPublished(ctx, event.ID); err != nil {
			return 0, err
		}
	}
	return len(events), nil
}

func (s *Service) processEvent(ctx context.Context, event outboxdomain.Event) error {
	for _, handler := range s.handlers {
		if err := handler.Handle(ctx, event); err != nil {
			return err
		}
	}
	if s.publisher == nil {
		return nil
	}
	return s.publisher.Publish(ctx, event)
}

func retryDelay(retryCount int) time.Duration {
	delay := time.Duration(retryCount+1) * 30 * time.Second
	if delay > 5*time.Minute {
		return 5 * time.Minute
	}
	return delay
}
