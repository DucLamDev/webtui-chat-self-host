package domain

import (
	"errors"
	"time"
)

var (
	ErrWebhookQuotaExceeded       = errors.New("zone da vuot webhook quota")
	ErrIncomingWebhookNotFound    = errors.New("không tìm thấy incoming webhook")
	ErrOutgoingWebhookNotFound    = errors.New("không tìm thấy outgoing webhook")
	ErrWebhookDeliveryNotFound    = errors.New("không tìm thấy webhook delivery")
	ErrIntegrationChannelNotFound = errors.New("không tìm thấy kênh tích hợp")
)

type IncomingWebhook struct {
	ID          string
	WorkspaceID string
	ChannelID   *string
	Name        string
	Status      string
	CreatedBy   *string
	CreatedAt   time.Time
	UpdatedAt   time.Time
	LastUsedAt  *time.Time
}

type OutgoingWebhook struct {
	ID                     string
	WorkspaceID            string
	Name                   string
	TargetURL              string
	SigningSecretEncrypted string
	EventTypes             []string
	Status                 string
	CreatedBy              *string
	CreatedAt              time.Time
	UpdatedAt              time.Time
}

type Delivery struct {
	ID                     string
	OutgoingWebhookID      string
	TargetURL              string
	SigningSecretEncrypted string
	EventID                *string
	EventType              string
	RequestBody            []byte
	ResponseStatus         *int
	ResponseBody           *string
	Status                 string
	AttemptCount           int
	NextAttemptAt          *time.Time
	DeliveredAt            *time.Time
	CreatedAt              time.Time
	UpdatedAt              time.Time
}

type IntegrationMessage struct {
	ID          string
	WorkspaceID string
	ChannelID   string
	Kind        string
	Body        string
	Metadata    []byte
	CreatedAt   time.Time
}
