package application

import (
	"context"

	apperrors "github.com/duclamdev/application-chat/backend/internal/shared/errors"
)

type unavailableProductivityRepository struct{}

func productivityCapabilityUnavailable() error {
	return apperrors.ServiceUnavailable(
		"MESSAGE_PRODUCTIVITY_UNAVAILABLE",
		"Message productivity features are not available with the configured messages repository.",
	)
}

func (unavailableProductivityRepository) ScheduleMessage(context.Context, ScheduleMessageParams) (ScheduledMessageDTO, error) {
	return ScheduledMessageDTO{}, productivityCapabilityUnavailable()
}

func (unavailableProductivityRepository) ListScheduledMessages(context.Context, UserProductivityParams) ([]ScheduledMessageDTO, error) {
	return nil, productivityCapabilityUnavailable()
}

func (unavailableProductivityRepository) CancelScheduledMessage(context.Context, ScheduledMessageRef) error {
	return productivityCapabilityUnavailable()
}

func (unavailableProductivityRepository) ProcessDueScheduledMessages(context.Context, int) (int, error) {
	return 0, productivityCapabilityUnavailable()
}

func (unavailableProductivityRepository) CreateReminder(context.Context, CreateReminderParams) (MessageReminderDTO, error) {
	return MessageReminderDTO{}, productivityCapabilityUnavailable()
}

func (unavailableProductivityRepository) ListReminders(context.Context, UserProductivityParams) ([]MessageReminderDTO, error) {
	return nil, productivityCapabilityUnavailable()
}

func (unavailableProductivityRepository) CancelReminder(context.Context, ReminderRef) error {
	return productivityCapabilityUnavailable()
}

func (unavailableProductivityRepository) ProcessDueReminders(context.Context, int) (int, error) {
	return 0, productivityCapabilityUnavailable()
}

func (unavailableProductivityRepository) GetThreadDetails(context.Context, ThreadDetailsParams) (ThreadDetailsDTO, error) {
	return ThreadDetailsDTO{}, productivityCapabilityUnavailable()
}

func (unavailableProductivityRepository) UpsertThreadDetails(context.Context, UpsertThreadDetailsParams) (ThreadDetailsDTO, error) {
	return ThreadDetailsDTO{}, productivityCapabilityUnavailable()
}

func (unavailableProductivityRepository) ListThreadDetails(context.Context, ListThreadDetailsParams) ([]ThreadDetailsDTO, error) {
	return nil, productivityCapabilityUnavailable()
}

func (unavailableProductivityRepository) SetThreadSubscription(context.Context, ThreadSubscriptionParams) (ThreadDetailsDTO, error) {
	return ThreadDetailsDTO{}, productivityCapabilityUnavailable()
}

func (unavailableProductivityRepository) MarkThreadRead(context.Context, ThreadReadParams) (ThreadDetailsDTO, error) {
	return ThreadDetailsDTO{}, productivityCapabilityUnavailable()
}
