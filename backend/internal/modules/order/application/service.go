package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/duclamdev/application-chat/backend/internal/shared/botauto"
	apperrors "github.com/duclamdev/application-chat/backend/internal/shared/errors"
)

var (
	ErrOrderBotTargetNotFound = errors.New("order bot target not found")
	ErrOrderChannelNotFound   = errors.New("order bot channel not found")
)

type UpstreamError struct {
	StatusCode int
	Message    string
}

func (e *UpstreamError) Error() string {
	if e == nil {
		return "order API request failed"
	}
	if strings.TrimSpace(e.Message) != "" {
		return strings.TrimSpace(e.Message)
	}
	return fmt.Sprintf("order API returned status %d", e.StatusCode)
}

const (
	PermissionOrderView           = "order.view"
	PermissionOrderBilling        = "order.billing"
	PermissionOrderPaymentRequest = "order.payment_request"

	defaultSupportBotSlug = "cskh-bot"
	defaultTicketBotSlug  = "ticket-bot"
	defaultPaymentBotSlug = "thanh-toan-bot"
	defaultRenewalBotSlug = "gia-han-bot"
	defaultAlertBotSlug   = "server-alert-bot"

	defaultSupportChannelSlug = "ticket"
	defaultPaymentChannelSlug = "ke-toan"
	defaultRenewalChannelSlug = "gia-han"
	defaultAlertChannelSlug   = "server-alert"
)

var (
	uuidPattern        = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
	emailPattern       = regexp.MustCompile(`(?i)[a-z0-9._%+\-]+@[a-z0-9.\-]+\.[a-z]{2,}`)
	labeledIntPattern  = regexp.MustCompile(`(?i)(user[_\s-]*id|uid|id|số ngày|so ngay|days?|ngày|ngay|số tiền|so tien|amount)\s*[:=]\s*([0-9][0-9\.\,\s]*[kKmM]?)`)
	plainDaysPattern   = regexp.MustCompile(`(?i)\b([0-9]{1,3})\s*(ngày|ngay|days?)\b`)
	amountHintPattern  = regexp.MustCompile(`(?i)(nạp|nap|qr|thanh toán|thanh toan|số tiền|so tien|amount|ck|chuyển khoản|chuyen khoan)`)
	serviceTypePattern = regexp.MustCompile(`(?i)(loại dịch vụ|loai dich vu|service[_\s-]*type|dịch vụ|dich vu)\s*[:=]\s*([^\n\r]+)`)
	intentIDPattern    = regexp.MustCompile(`(?i)intent[_\s-]*id\s*[:=#]?\s*([0-9]+)`)
	intentCodePattern  = regexp.MustCompile(`(?i)(?:intent[_\s-]*code|mã đơn|ma don|đơn hàng|don hang)\s*[:=#]?\s*([A-Z0-9][A-Z0-9_-]{5,})`)
	quickIntentPattern = regexp.MustCompile(`(?i)\bQOI[A-Z0-9_-]{6,}\b`)
	orderRefPattern    = regexp.MustCompile(`(?i)(?:reference|mã tham chiếu|ma tham chieu)\s*[:=#]?\s*([A-Z0-9][A-Z0-9_-]{5,})`)
	serviceIDPattern   = regexp.MustCompile(`(?i)(?:dịch vụ|dich vu|service|vps|proxy|hosting|s3|drive|waf|domain|separate)(?:\s+[a-z0-9._-]+)?\s*#\s*([0-9]+)`)
	serviceNamePattern = regexp.MustCompile(`(?i)(?:gia hạn|gia han|renew|extend)(?:\s+(?:dịch vụ|dich vu|service))?\s+(.+?)\s+(?:của|cua)\s+(?:tài khoản|tai khoan|email)\b`)
	monthsPattern      = regexp.MustCompile(`(?i)\b([0-9]{1,2})\s*(tháng|thang|months?)\b`)
)

type PermissionChecker interface {
	HasWorkspacePermission(ctx context.Context, userID string, workspaceID string, permissionCode string) (bool, error)
}

type Client interface {
	Configured() bool
	QuickOrderConfigured() bool
	WalletBalance(ctx context.Context, input UserLookupRequest) (WalletBalanceEnvelope, error)
	CreateDepositQR(ctx context.Context, input WalletDepositQRRequest) (WalletDepositQREnvelope, error)
	ServicesExpiring(ctx context.Context, input ServicesExpiringRequest) (ServicesExpiringEnvelope, error)
	RenewService(ctx context.Context, input RenewServiceRequest) (RenewServiceEnvelope, error)
	CreateOrderPaymentQR(ctx context.Context, input OrderPaymentQRRequest) (OrderPaymentQREnvelope, error)
}

type Repository interface {
	ChannelByID(ctx context.Context, workspaceID string, channelID string) (ChannelDTO, error)
	SendBotMessage(ctx context.Context, params SendBotMessageParams) (BotMessageDTO, error)
	UserEmailByID(ctx context.Context, userID string) (string, error)
	WorkspaceSupportsOrderBot(ctx context.Context, workspaceID string) (bool, error)
}

type Service struct {
	client  Client
	repo    Repository
	checker PermissionChecker
	now     func() time.Time
}

type UserLookupRequest struct {
	Email  string `json:"email,omitempty"`
	UserID int    `json:"user_id,omitempty"`
}

type WalletBalanceInput struct {
	ActorUserID      string
	WorkspaceID      string
	TriggerMessageID string `json:"-"`
	Email            string `json:"email,omitempty"`
	UserID           int    `json:"user_id,omitempty"`
	ChannelID        string `json:"channel_id,omitempty"`
	PostToChannel    *bool  `json:"post_to_channel,omitempty"`
}

type WalletDepositQRInput struct {
	ActorUserID      string
	WorkspaceID      string
	TriggerMessageID string `json:"-"`
	Email            string `json:"email,omitempty"`
	Amount           int    `json:"amount,omitempty"`
	ExpiresMinutes   int    `json:"expires_minutes,omitempty"`
	ChannelID        string `json:"channel_id,omitempty"`
	PostToChannel    *bool  `json:"post_to_channel,omitempty"`
}

type ServicesExpiringInput struct {
	ActorUserID      string
	WorkspaceID      string
	TriggerMessageID string `json:"-"`
	Email            string `json:"email,omitempty"`
	UserID           int    `json:"user_id,omitempty"`
	Days             int    `json:"days,omitempty"`
	IncludeExpired   bool   `json:"include_expired,omitempty"`
	ServiceType      string `json:"service_type,omitempty"`
	ChannelID        string `json:"channel_id,omitempty"`
	PostToChannel    *bool  `json:"post_to_channel,omitempty"`
}

type RenewServiceInput struct {
	ActorUserID      string
	WorkspaceID      string
	TriggerMessageID string `json:"-"`
	Email            string `json:"email,omitempty"`
	UserID           int    `json:"user_id,omitempty"`
	ServiceType      string `json:"service_type,omitempty"`
	ServiceID        int    `json:"service_id,omitempty"`
	ServiceName      string `json:"service_name,omitempty"`
	Months           int    `json:"months,omitempty"`
	ChannelID        string `json:"channel_id,omitempty"`
	PostToChannel    *bool  `json:"post_to_channel,omitempty"`
}

type WalletDepositQRRequest struct {
	Email          string `json:"email"`
	Amount         int    `json:"amount"`
	ExpiresMinutes int    `json:"expires_minutes,omitempty"`
}

type OrderPaymentQRInput struct {
	ActorUserID      string
	WorkspaceID      string
	TriggerMessageID string `json:"-"`
	IntentID         int    `json:"intent_id,omitempty"`
	IntentCode       string `json:"intent_code,omitempty"`
	Reference        string `json:"reference,omitempty"`
	ChannelID        string `json:"channel_id,omitempty"`
	PostToChannel    *bool  `json:"post_to_channel,omitempty"`
}

type OrderPaymentQRRequest struct {
	IntentID   int    `json:"intent_id,omitempty"`
	IntentCode string `json:"intent_code,omitempty"`
	Reference  string `json:"reference,omitempty"`
}

type ServicesExpiringRequest struct {
	Email          string `json:"email,omitempty"`
	UserID         int    `json:"user_id,omitempty"`
	Days           int    `json:"days,omitempty"`
	IncludeExpired bool   `json:"include_expired"`
	ServiceType    string `json:"service_type,omitempty"`
}

type RenewServiceRequest struct {
	Email          string `json:"email,omitempty"`
	UserID         int    `json:"user_id,omitempty"`
	ServiceType    string `json:"service_type,omitempty"`
	ServiceID      int    `json:"service_id,omitempty"`
	ServiceName    string `json:"service_name,omitempty"`
	Months         int    `json:"months"`
	IdempotencyKey string `json:"idempotency_key"`
}

type SendBotMessageParams struct {
	WorkspaceID string
	BotSlug     string
	ChannelID   string
	ChannelSlug string
	Body        string
	Metadata    []byte
}

type ChannelDTO struct {
	ID          string
	WorkspaceID string
	Slug        string
	Name        string
}

type BotMessageDTO struct {
	ID          string          `json:"id"`
	WorkspaceID string          `json:"workspace_id"`
	ChannelID   string          `json:"channel_id"`
	BotID       string          `json:"bot_id"`
	Kind        string          `json:"kind"`
	Body        string          `json:"body"`
	Metadata    json.RawMessage `json:"metadata"`
	CreatedAt   string          `json:"created_at"`
}

type StatusDTO struct {
	Configured           bool   `json:"configured"`
	QuickOrderConfigured bool   `json:"quick_order_configured"`
	BaseURL              string `json:"base_url,omitempty"`
}

type WalletBalanceResult struct {
	Data       WalletBalanceData `json:"data"`
	BotMessage *BotMessageDTO    `json:"bot_message,omitempty"`
}

type WalletDepositQRResult struct {
	Data       WalletDepositQRData `json:"data"`
	BotMessage *BotMessageDTO      `json:"bot_message,omitempty"`
}

type OrderPaymentQRResult struct {
	Data       OrderPaymentQRData `json:"data"`
	BotMessage *BotMessageDTO     `json:"bot_message,omitempty"`
}

type ServicesExpiringResult struct {
	Data       ServicesExpiringData `json:"data"`
	BotMessage *BotMessageDTO       `json:"bot_message,omitempty"`
}

type RenewServiceResult struct {
	Data       RenewServiceData `json:"data"`
	BotMessage *BotMessageDTO   `json:"bot_message,omitempty"`
}

type WalletBalanceEnvelope struct {
	OK      bool              `json:"ok"`
	Status  string            `json:"status,omitempty"`
	Message string            `json:"message,omitempty"`
	Data    WalletBalanceData `json:"data"`
}

type WalletBalanceData struct {
	UserID     int            `json:"user_id,omitempty"`
	Email      string         `json:"email,omitempty"`
	Name       string         `json:"name,omitempty"`
	Balance    float64        `json:"balance,omitempty"`
	BalanceVND int            `json:"balance_vnd,omitempty"`
	Money      float64        `json:"money,omitempty"`
	Agency     string         `json:"agency,omitempty"`
	Services   map[string]int `json:"services,omitempty"`
}

type WalletDepositQREnvelope struct {
	OK      bool                `json:"ok"`
	Status  string              `json:"status,omitempty"`
	Message string              `json:"message,omitempty"`
	Data    WalletDepositQRData `json:"data"`
}

type WalletDepositQRData struct {
	RequestID         int               `json:"request_id,omitempty"`
	Reference         string            `json:"reference,omitempty"`
	Email             string            `json:"email,omitempty"`
	UserID            int               `json:"user_id,omitempty"`
	Name              string            `json:"name,omitempty"`
	Amount            int               `json:"amount,omitempty"`
	Currency          string            `json:"currency,omitempty"`
	Status            string            `json:"status,omitempty"`
	QRURL             string            `json:"qr_url,omitempty"`
	Bank              WalletDepositBank `json:"bank,omitempty"`
	TransferContent   string            `json:"transfer_content,omitempty"`
	UserBalanceBefore float64           `json:"user_balance_before,omitempty"`
	ExpiresAt         string            `json:"expires_at,omitempty"`
}

type WalletDepositBank struct {
	BankCode        string `json:"bank_code,omitempty"`
	BIN             string `json:"bin,omitempty"`
	AccountNumber   string `json:"account_number,omitempty"`
	AccountName     string `json:"account_name,omitempty"`
	TransferContent string `json:"transfer_content,omitempty"`
	RequestedAmount int    `json:"requested_amount,omitempty"`
	AutoCheck       bool   `json:"auto_check,omitempty"`
}

type OrderPaymentQREnvelope struct {
	OK      bool               `json:"ok"`
	Status  string             `json:"status,omitempty"`
	Message string             `json:"message,omitempty"`
	Data    OrderPaymentQRData `json:"data"`
}

type OrderPaymentQRData struct {
	PaymentID       int            `json:"payment_id,omitempty"`
	IntentID        int            `json:"intent_id,omitempty"`
	ExternalOrderID string         `json:"external_order_id,omitempty"`
	Reference       string         `json:"reference,omitempty"`
	CustomerEmail   string         `json:"customer_email,omitempty"`
	Amount          int            `json:"amount,omitempty"`
	Currency        string         `json:"currency,omitempty"`
	Status          string         `json:"status,omitempty"`
	QRURL           string         `json:"qr_url,omitempty"`
	Bank            map[string]any `json:"bank,omitempty"`
	ExpiresAt       string         `json:"expires_at,omitempty"`
}

type ServicesExpiringEnvelope struct {
	OK      bool                 `json:"ok"`
	Status  string               `json:"status,omitempty"`
	Message string               `json:"message,omitempty"`
	Data    ServicesExpiringData `json:"data"`
}

type RenewServiceEnvelope struct {
	OK      bool             `json:"ok"`
	Status  string           `json:"status,omitempty"`
	Message string           `json:"message,omitempty"`
	Data    RenewServiceData `json:"data"`
}

type RenewServiceData struct {
	Outcome         string              `json:"outcome,omitempty"`
	TransactionID   string              `json:"transaction_id,omitempty"`
	User            ExpiringUserSummary `json:"user,omitempty"`
	ServiceType     string              `json:"service_type,omitempty"`
	ServiceID       int                 `json:"service_id,omitempty"`
	ServiceName     string              `json:"service_name,omitempty"`
	Months          int                 `json:"months,omitempty"`
	Amount          int                 `json:"amount,omitempty"`
	BalanceBefore   int                 `json:"balance_before,omitempty"`
	BalanceAfter    int                 `json:"balance_after,omitempty"`
	ShortageAmount  int                 `json:"shortage_amount,omitempty"`
	ExpiresAtBefore string              `json:"expires_at_before,omitempty"`
	ExpiresAtAfter  string              `json:"expires_at_after,omitempty"`
}

type ServicesExpiringData struct {
	User           ExpiringUserSummary     `json:"user,omitempty"`
	Days           int                     `json:"days,omitempty"`
	IncludeExpired bool                    `json:"include_expired,omitempty"`
	ServiceType    string                  `json:"service_type,omitempty"`
	Summary        ServicesExpiringSummary `json:"summary,omitempty"`
	Items          []ServiceExpiringItem   `json:"items,omitempty"`
}

type ExpiringUserSummary struct {
	UserID  int     `json:"user_id,omitempty"`
	Email   string  `json:"email,omitempty"`
	Name    string  `json:"name,omitempty"`
	Balance float64 `json:"balance,omitempty"`
}

type ServicesExpiringSummary struct {
	Total        int               `json:"total,omitempty"`
	Expired      int               `json:"expired,omitempty"`
	Expiring     int               `json:"expiring,omitempty"`
	AutoRenewOff int               `json:"auto_renew_off,omitempty"`
	ByType       ServiceTypeCounts `json:"by_type,omitempty"`
}

type ServiceTypeCounts map[string]int

func (counts *ServiceTypeCounts) UnmarshalJSON(data []byte) error {
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" || trimmed == "null" {
		*counts = ServiceTypeCounts{}
		return nil
	}
	if strings.HasPrefix(trimmed, "{") {
		var values map[string]int
		if err := json.Unmarshal(data, &values); err != nil {
			return err
		}
		*counts = ServiceTypeCounts(values)
		return nil
	}
	if strings.HasPrefix(trimmed, "[") {
		var rows []struct {
			ServiceTypeKey string `json:"service_type_key"`
			ServiceType    string `json:"service_type"`
			Type           string `json:"type"`
			Key            string `json:"key"`
			Count          *int   `json:"count"`
			Total          *int   `json:"total"`
			Value          *int   `json:"value"`
		}
		if err := json.Unmarshal(data, &rows); err != nil {
			return err
		}
		values := ServiceTypeCounts{}
		for _, row := range rows {
			key := strings.ToLower(strings.TrimSpace(firstNonEmpty(row.ServiceTypeKey, row.ServiceType, row.Type, row.Key)))
			if key == "" {
				continue
			}
			switch {
			case row.Count != nil:
				values[key] = *row.Count
			case row.Total != nil:
				values[key] = *row.Total
			case row.Value != nil:
				values[key] = *row.Value
			}
		}
		*counts = values
		return nil
	}
	return fmt.Errorf("invalid services summary by_type JSON")
}

type ServiceExpiringItem struct {
	ServiceTypeKey         string `json:"service_type_key,omitempty"`
	ServiceType            string `json:"service_type,omitempty"`
	ServiceID              int    `json:"service_id,omitempty"`
	Name                   string `json:"name,omitempty"`
	Meta                   string `json:"meta,omitempty"`
	Status                 string `json:"status,omitempty"`
	StatusLabel            string `json:"status_label,omitempty"`
	ExpiresAt              string `json:"expires_at,omitempty"`
	DaysRemaining          int    `json:"days_remaining,omitempty"`
	AutoExtend             *int   `json:"autoextend,omitempty"`
	Route                  string `json:"route,omitempty"`
	RenewalTransferContent string `json:"renewal_transfer_content,omitempty"`
}

func NewService(client Client, repo Repository, checker PermissionChecker) *Service {
	return &Service{
		client:  client,
		repo:    repo,
		checker: checker,
		now:     func() time.Time { return time.Now().UTC() },
	}
}

func (s *Service) Status(ctx context.Context, actorUserID string, workspaceID string) (StatusDTO, error) {
	if err := s.ensurePermission(ctx, actorUserID, workspaceID, PermissionOrderView); err != nil {
		return StatusDTO{}, err
	}
	return StatusDTO{
		Configured:           s.client != nil && s.client.Configured(),
		QuickOrderConfigured: s.client != nil && s.client.QuickOrderConfigured(),
	}, nil
}

func (s *Service) HandleMessage(ctx context.Context, input botauto.MessageInput) ([]botauto.BotMessage, error) {
	workspaceID := strings.TrimSpace(input.WorkspaceID)
	channelID := strings.TrimSpace(input.ChannelID)
	body := strings.TrimSpace(input.Body)
	if s == nil {
		slog.Warn("Order bot auto responder chua duoc khoi tao",
			"workspace_id", workspaceID,
			"channel_id", channelID,
			"message_id", input.MessageID,
		)
		return nil, nil
	}
	if s.repo == nil {
		slog.Warn("Order bot repository chua duoc cau hinh",
			"workspace_id", workspaceID,
			"channel_id", channelID,
			"message_id", input.MessageID,
		)
		return nil, nil
	}
	if body == "" {
		slog.Debug("Order bot bo qua tin nhan rong",
			"workspace_id", workspaceID,
			"channel_id", channelID,
			"message_id", input.MessageID,
		)
		return nil, nil
	}
	slog.Debug("Order bot nhan tin hieu auto responder",
		"workspace_id", workspaceID,
		"channel_id", channelID,
		"message_id", input.MessageID,
		"body_len", len([]rune(body)),
	)
	channel, err := s.repo.ChannelByID(ctx, workspaceID, channelID)
	if err != nil {
		slog.Warn("Order bot khong lay duoc thong tin kenh",
			"workspace_id", workspaceID,
			"channel_id", channelID,
			"message_id", input.MessageID,
			"error", err,
		)
		return nil, nil
	}

	command := parseAutoBotCommand(input.Body)
	channelSlug := strings.TrimSpace(channel.Slug)
	fields := append([]any{
		"workspace_id", workspaceID,
		"channel_id", channelID,
		"channel_slug", channelSlug,
		"message_id", input.MessageID,
	}, autoBotCommandLogFields(command)...)
	slog.Info("Order bot kiem tra kenh va intent", fields...)
	switch channelSlug {
	case defaultRenewalChannelSlug:
		return s.handleRenewalAutoMessage(ctx, input, command)
	case defaultPaymentChannelSlug:
		return s.handlePaymentAutoMessage(ctx, input, command)
	case defaultSupportChannelSlug:
		return s.handleTicketAutoMessage(ctx, input, command)
	case defaultAlertChannelSlug:
		return s.handleAlertAutoMessage(ctx, input, command)
	default:
		slog.Debug("Order bot bo qua kenh khong thuoc bot order",
			"workspace_id", workspaceID,
			"channel_id", channelID,
			"channel_slug", channelSlug,
			"message_id", input.MessageID,
		)
		return nil, nil
	}
}

func (s *Service) handleRenewalAutoMessage(ctx context.Context, input botauto.MessageInput, command autoBotCommand) ([]botauto.BotMessage, error) {
	if command.IsHelp || !command.HasLookup {
		fields := append([]any{
			"workspace_id", input.WorkspaceID,
			"channel_id", input.ChannelID,
			"message_id", input.MessageID,
		}, autoBotCommandLogFields(command)...)
		slog.Info("Gia Han Bot gui huong dan vi thieu lookup hoac nguoi dung hoi help", fields...)
		return s.postAutoGuide(ctx, input, defaultRenewalBotSlug, defaultRenewalChannelSlug, "auto_help_gia_han", renewalBotGuide())
	}
	if command.RenewalIntent {
		if command.ServiceID <= 0 && strings.TrimSpace(command.ServiceName) == "" {
			return s.postAutoGuide(ctx, input, defaultRenewalBotSlug, defaultRenewalChannelSlug, "auto_help_gia_han", renewalBotGuide())
		}
		fields := append([]any{
			"workspace_id", input.WorkspaceID,
			"channel_id", input.ChannelID,
			"message_id", input.MessageID,
		}, autoBotCommandLogFields(command)...)
		slog.Info("Gia Han Bot bat dau yeu cau gia han dich vu", fields...)
		result, err := s.RenewService(ctx, RenewServiceInput{
			ActorUserID:      input.ActorUserID,
			WorkspaceID:      input.WorkspaceID,
			TriggerMessageID: input.MessageID,
			Email:            command.Email,
			UserID:           command.UserID,
			ServiceType:      command.ServiceType,
			ServiceID:        command.ServiceID,
			ServiceName:      command.ServiceName,
			Months:           command.Months,
			ChannelID:        input.ChannelID,
		})
		if err != nil {
			return s.postAutoError(ctx, input, defaultRenewalBotSlug, defaultRenewalChannelSlug, "Gia Hạn Bot", err, renewalBotGuide())
		}
		return autoBotMessages(result.BotMessage), nil
	}
	fields := append([]any{
		"workspace_id", input.WorkspaceID,
		"channel_id", input.ChannelID,
		"message_id", input.MessageID,
	}, autoBotCommandLogFields(command)...)
	slog.Info("Gia Han Bot bat dau tra cuu dich vu sap het han", fields...)
	result, err := s.ServicesExpiring(ctx, ServicesExpiringInput{
		ActorUserID:      input.ActorUserID,
		WorkspaceID:      input.WorkspaceID,
		TriggerMessageID: input.MessageID,
		Email:            command.Email,
		UserID:           command.UserID,
		Days:             command.Days,
		IncludeExpired:   command.IncludeExpired,
		ServiceType:      command.ServiceType,
		ChannelID:        input.ChannelID,
	})
	if err != nil {
		return s.postAutoError(ctx, input, defaultRenewalBotSlug, defaultRenewalChannelSlug, "Gia Hạn Bot", err, renewalBotGuide())
	}
	return autoBotMessages(result.BotMessage), nil
}

func (s *Service) handlePaymentAutoMessage(ctx context.Context, input botauto.MessageInput, command autoBotCommand) ([]botauto.BotMessage, error) {
	if command.IsHelp {
		return s.postAutoGuide(ctx, input, defaultPaymentBotSlug, defaultPaymentChannelSlug, "auto_help_payment", paymentBotGuide())
	}
	if command.HasOrderPayment {
		result, err := s.CreateOrderPaymentQR(ctx, OrderPaymentQRInput{
			ActorUserID:      input.ActorUserID,
			WorkspaceID:      input.WorkspaceID,
			TriggerMessageID: input.MessageID,
			IntentID:         command.IntentID,
			IntentCode:       command.IntentCode,
			Reference:        command.Reference,
			ChannelID:        input.ChannelID,
		})
		if err != nil {
			return s.postAutoError(ctx, input, defaultPaymentBotSlug, defaultPaymentChannelSlug, "Thanh Toán Bot", err, paymentBotGuide())
		}
		return autoBotMessages(result.BotMessage), nil
	}
	if !command.PaymentIntent && !command.HasAmount {
		return nil, nil
	}
	if command.Email == "" || command.Amount < 1000 {
		return s.postAutoGuide(ctx, input, defaultPaymentBotSlug, defaultPaymentChannelSlug, "auto_help_payment", paymentBotGuide())
	}
	result, err := s.CreateDepositQR(ctx, WalletDepositQRInput{
		ActorUserID:      input.ActorUserID,
		WorkspaceID:      input.WorkspaceID,
		TriggerMessageID: input.MessageID,
		Email:            command.Email,
		Amount:           command.Amount,
		ExpiresMinutes:   command.ExpiresMinutes,
		ChannelID:        input.ChannelID,
	})
	if err != nil {
		return s.postAutoError(ctx, input, defaultPaymentBotSlug, defaultPaymentChannelSlug, "Thanh Toán Bot", err, paymentBotGuide())
	}
	return autoBotMessages(result.BotMessage), nil
}

func (s *Service) handleTicketAutoMessage(ctx context.Context, input botauto.MessageInput, command autoBotCommand) ([]botauto.BotMessage, error) {
	if command.IsHelp {
		return s.postAutoGuide(ctx, input, defaultTicketBotSlug, defaultSupportChannelSlug, "auto_help_ticket", ticketBotGuide())
	}
	if command.HasLookup && command.WalletIntent {
		result, err := s.WalletBalance(ctx, WalletBalanceInput{
			ActorUserID:      input.ActorUserID,
			WorkspaceID:      input.WorkspaceID,
			TriggerMessageID: input.MessageID,
			Email:            command.Email,
			UserID:           command.UserID,
			ChannelID:        input.ChannelID,
		})
		if err != nil {
			return s.postAutoError(ctx, input, defaultSupportBotSlug, defaultSupportChannelSlug, "CSKH Bot", err, ticketBotGuide())
		}
		return autoBotMessages(result.BotMessage), nil
	}
	if command.TicketIntent {
		return s.postAutoText(ctx, input, defaultTicketBotSlug, defaultSupportChannelSlug, formatTicketTriageMessage(input.Body, command), map[string]any{
			"source":             "vpsttt_order",
			"action":             "ticket_triage",
			"trigger_message_id": input.MessageID,
			"email":              command.Email,
			"user_id":            command.UserID,
		})
	}
	return nil, nil
}

func (s *Service) handleAlertAutoMessage(ctx context.Context, input botauto.MessageInput, command autoBotCommand) ([]botauto.BotMessage, error) {
	if command.IsHelp {
		return s.postAutoGuide(ctx, input, defaultAlertBotSlug, defaultAlertChannelSlug, "auto_help_alert", alertBotGuide())
	}
	if !command.AlertIntent {
		return nil, nil
	}
	return s.postAutoText(ctx, input, defaultAlertBotSlug, defaultAlertChannelSlug, formatServerAlertMessage(input.Body), map[string]any{
		"source":             "vpsttt_order",
		"action":             "server_alert_triage",
		"trigger_message_id": input.MessageID,
	})
}

func (s *Service) WalletBalance(ctx context.Context, input WalletBalanceInput) (WalletBalanceResult, error) {
	if err := s.ensurePermission(ctx, input.ActorUserID, input.WorkspaceID, PermissionOrderView); err != nil {
		return WalletBalanceResult{}, err
	}
	if err := s.ensureConfigured(); err != nil {
		return WalletBalanceResult{}, err
	}
	lookup, err := normalizeLookup(input.Email, input.UserID)
	if err != nil {
		return WalletBalanceResult{}, err
	}
	if err := validateOptionalChannelID(input.ChannelID); err != nil {
		return WalletBalanceResult{}, err
	}
	envelope, err := s.client.WalletBalance(ctx, lookup)
	if err != nil {
		slog.Warn("Order API request failed",
			"action", "wallet_balance",
			"workspace_id", input.WorkspaceID,
			"error", err,
		)
		return WalletBalanceResult{}, mapOrderClientError(err)
	}
	if err := ensureRemoteOK(envelope.OK, envelope.Status, envelope.Message); err != nil {
		return WalletBalanceResult{}, err
	}

	result := WalletBalanceResult{Data: envelope.Data}
	if shouldPost(input.PostToChannel) {
		message, err := s.postBotMessage(ctx, input.WorkspaceID, defaultSupportBotSlug, input.ChannelID, defaultSupportChannelSlug, formatWalletBalanceMessage(envelope.Data), map[string]any{
			"source":             "vpsttt_order",
			"action":             "wallet_balance",
			"trigger_message_id": input.TriggerMessageID,
			"email":              lookup.Email,
			"user_id":            lookup.UserID,
		})
		if err != nil {
			return WalletBalanceResult{}, err
		}
		result.BotMessage = &message
	}
	return result, nil
}

func (s *Service) CreateDepositQR(ctx context.Context, input WalletDepositQRInput) (WalletDepositQRResult, error) {
	if err := s.ensurePermission(ctx, input.ActorUserID, input.WorkspaceID, PermissionOrderPaymentRequest); err != nil {
		return WalletDepositQRResult{}, err
	}
	if err := s.ensureConfigured(); err != nil {
		return WalletDepositQRResult{}, err
	}
	email := normalizeEmail(input.Email)
	if email == "" {
		return WalletDepositQRResult{}, apperrors.BadRequest("VALIDATION_ERROR", "Email khách hàng là bắt buộc.")
	}
	if input.Amount < 1000 {
		return WalletDepositQRResult{}, apperrors.BadRequest("VALIDATION_ERROR", "Số tiền nạp ví tối thiểu là 1.000 VND.")
	}
	expiresMinutes := input.ExpiresMinutes
	if expiresMinutes == 0 {
		expiresMinutes = 1440
	}
	if expiresMinutes < 5 || expiresMinutes > 43200 {
		return WalletDepositQRResult{}, apperrors.BadRequest("VALIDATION_ERROR", "Thời hạn QR phải từ 5 đến 43.200 phút.")
	}
	if err := validateOptionalChannelID(input.ChannelID); err != nil {
		return WalletDepositQRResult{}, err
	}

	envelope, err := s.client.CreateDepositQR(ctx, WalletDepositQRRequest{
		Email:          email,
		Amount:         input.Amount,
		ExpiresMinutes: expiresMinutes,
	})
	if err != nil {
		slog.Warn("Order API request failed",
			"action", "wallet_deposit_qr",
			"workspace_id", input.WorkspaceID,
			"error", err,
		)
		return WalletDepositQRResult{}, mapOrderClientError(err)
	}
	if err := ensureRemoteOK(envelope.OK, envelope.Status, envelope.Message); err != nil {
		return WalletDepositQRResult{}, err
	}

	result := WalletDepositQRResult{Data: envelope.Data}
	if shouldPost(input.PostToChannel) {
		message, err := s.postBotMessage(ctx, input.WorkspaceID, defaultPaymentBotSlug, input.ChannelID, defaultPaymentChannelSlug, formatDepositQRMessage(envelope.Data), map[string]any{
			"source":             "vpsttt_order",
			"action":             "wallet_deposit_qr",
			"card_type":          "payment_qr",
			"trigger_message_id": input.TriggerMessageID,
			"email":              email,
			"amount":             input.Amount,
			"reference":          envelope.Data.Reference,
			"qr_url":             envelope.Data.QRURL,
			"transfer_content":   envelope.Data.TransferContent,
			"expires_at":         envelope.Data.ExpiresAt,
		})
		if err != nil {
			return WalletDepositQRResult{}, err
		}
		result.BotMessage = &message
	}
	return result, nil
}

func (s *Service) CreateOrderPaymentQR(ctx context.Context, input OrderPaymentQRInput) (OrderPaymentQRResult, error) {
	if err := s.ensurePermission(ctx, input.ActorUserID, input.WorkspaceID, PermissionOrderPaymentRequest); err != nil {
		return OrderPaymentQRResult{}, err
	}
	if err := s.ensureQuickOrderConfigured(); err != nil {
		return OrderPaymentQRResult{}, err
	}
	intentCode := strings.TrimSpace(input.IntentCode)
	reference := strings.TrimSpace(input.Reference)
	if input.IntentID <= 0 && intentCode == "" && reference == "" {
		return OrderPaymentQRResult{}, apperrors.BadRequest("VALIDATION_ERROR", "Cần truyền intent_id, intent_code hoặc mã tham chiếu của đơn hàng.")
	}
	if err := validateOptionalChannelID(input.ChannelID); err != nil {
		return OrderPaymentQRResult{}, err
	}

	envelope, err := s.client.CreateOrderPaymentQR(ctx, OrderPaymentQRRequest{
		IntentID:   input.IntentID,
		IntentCode: intentCode,
		Reference:  reference,
	})
	if err != nil {
		return OrderPaymentQRResult{}, mapOrderClientError(err)
	}
	if err := ensureRemoteOK(envelope.OK, envelope.Status, envelope.Message); err != nil {
		return OrderPaymentQRResult{}, err
	}

	result := OrderPaymentQRResult{Data: envelope.Data}
	if shouldPost(input.PostToChannel) {
		message, err := s.postBotMessage(ctx, input.WorkspaceID, defaultPaymentBotSlug, input.ChannelID, defaultPaymentChannelSlug, formatOrderPaymentQRMessage(envelope.Data), map[string]any{
			"source":             "vpsttt_order",
			"action":             "order_payment_qr",
			"card_type":          "payment_qr",
			"trigger_message_id": input.TriggerMessageID,
			"intent_id":          envelope.Data.IntentID,
			"external_order_id":  envelope.Data.ExternalOrderID,
			"reference":          envelope.Data.Reference,
			"amount":             envelope.Data.Amount,
			"qr_url":             envelope.Data.QRURL,
			"expires_at":         envelope.Data.ExpiresAt,
		})
		if err != nil {
			return OrderPaymentQRResult{}, err
		}
		result.BotMessage = &message
	}
	return result, nil
}

func (s *Service) ServicesExpiring(ctx context.Context, input ServicesExpiringInput) (ServicesExpiringResult, error) {
	if err := s.ensurePermission(ctx, input.ActorUserID, input.WorkspaceID, PermissionOrderView); err != nil {
		return ServicesExpiringResult{}, err
	}
	if err := s.ensureConfigured(); err != nil {
		return ServicesExpiringResult{}, err
	}
	lookup, err := normalizeLookup(input.Email, input.UserID)
	if err != nil {
		slog.Warn("Order API request failed",
			"action", "services_expiring",
			"workspace_id", input.WorkspaceID,
			"error", err,
		)
		return ServicesExpiringResult{}, mapOrderClientError(err)
	}
	days := input.Days
	if days == 0 {
		days = 7
	}
	if days < 0 || days > 365 {
		return ServicesExpiringResult{}, apperrors.BadRequest("VALIDATION_ERROR", "Số ngày lọc dịch vụ phải từ 0 đến 365.")
	}
	serviceType := normalizeServiceType(input.ServiceType)
	if strings.TrimSpace(input.ServiceType) != "" && serviceType == "" {
		return ServicesExpiringResult{}, apperrors.BadRequest("VALIDATION_ERROR", "Loại dịch vụ không hợp lệ.")
	}
	if serviceType == "" {
		serviceType = "all"
	}
	if err := validateOptionalChannelID(input.ChannelID); err != nil {
		return ServicesExpiringResult{}, err
	}

	envelope, err := s.client.ServicesExpiring(ctx, ServicesExpiringRequest{
		Email:          lookup.Email,
		UserID:         lookup.UserID,
		Days:           days,
		IncludeExpired: input.IncludeExpired,
		ServiceType:    serviceType,
	})
	if err != nil {
		return ServicesExpiringResult{}, err
	}
	if err := ensureRemoteOK(envelope.OK, envelope.Status, envelope.Message); err != nil {
		return ServicesExpiringResult{}, err
	}

	result := ServicesExpiringResult{Data: envelope.Data}
	if shouldPost(input.PostToChannel) {
		message, err := s.postBotMessage(ctx, input.WorkspaceID, defaultRenewalBotSlug, input.ChannelID, defaultRenewalChannelSlug, formatExpiringServicesMessage(envelope.Data), map[string]any{
			"source":             "vpsttt_order",
			"action":             "services_expiring",
			"trigger_message_id": input.TriggerMessageID,
			"email":              lookup.Email,
			"user_id":            lookup.UserID,
			"days":               days,
			"service_type":       serviceType,
		})
		if err != nil {
			return ServicesExpiringResult{}, err
		}
		result.BotMessage = &message
	}
	return result, nil
}

func (s *Service) RenewService(ctx context.Context, input RenewServiceInput) (RenewServiceResult, error) {
	if err := s.ensureConfigured(); err != nil {
		return RenewServiceResult{}, err
	}
	lookup, err := normalizeLookup(input.Email, input.UserID)
	if err != nil {
		return RenewServiceResult{}, err
	}
	if err := s.ensureRenewalPermission(ctx, input.ActorUserID, input.WorkspaceID, lookup.Email); err != nil {
		return RenewServiceResult{}, err
	}
	serviceType := normalizeServiceType(input.ServiceType)
	if strings.TrimSpace(input.ServiceType) != "" && serviceType == "" {
		return RenewServiceResult{}, apperrors.BadRequest("VALIDATION_ERROR", "Loại dịch vụ không hợp lệ.")
	}
	serviceName := strings.TrimSpace(input.ServiceName)
	if input.ServiceID <= 0 && serviceName == "" {
		return RenewServiceResult{}, apperrors.BadRequest("VALIDATION_ERROR", "Cần cung cấp mã hoặc tên dịch vụ muốn gia hạn.")
	}
	months := input.Months
	if months == 0 {
		months = 1
	}
	if months < 1 || months > 36 {
		return RenewServiceResult{}, apperrors.BadRequest("VALIDATION_ERROR", "Thời gian gia hạn phải từ 1 đến 36 tháng.")
	}
	if err := validateOptionalChannelID(input.ChannelID); err != nil {
		return RenewServiceResult{}, err
	}
	idempotencyKey := strings.TrimSpace(input.TriggerMessageID)
	if idempotencyKey == "" {
		return RenewServiceResult{}, apperrors.BadRequest("VALIDATION_ERROR", "Thiếu mã chống gửi trùng cho yêu cầu gia hạn.")
	}

	envelope, err := s.client.RenewService(ctx, RenewServiceRequest{
		Email:          lookup.Email,
		UserID:         lookup.UserID,
		ServiceType:    serviceType,
		ServiceID:      input.ServiceID,
		ServiceName:    serviceName,
		Months:         months,
		IdempotencyKey: idempotencyKey,
	})
	if err != nil {
		slog.Warn("Order API request failed",
			"action", "service_renew",
			"workspace_id", input.WorkspaceID,
			"service_type", serviceType,
			"service_id", input.ServiceID,
			"error", err,
		)
		return RenewServiceResult{}, mapRenewalClientError(err)
	}
	if err := ensureRemoteOK(envelope.OK, envelope.Status, envelope.Message); err != nil {
		return RenewServiceResult{}, err
	}

	result := RenewServiceResult{Data: envelope.Data}
	if shouldPost(input.PostToChannel) {
		message, err := s.postBotMessage(ctx, input.WorkspaceID, defaultRenewalBotSlug, input.ChannelID, defaultRenewalChannelSlug, formatRenewServiceMessage(envelope.Data), map[string]any{
			"source":             "vpsttt_order",
			"action":             "service_renew",
			"trigger_message_id": input.TriggerMessageID,
			"transaction_id":     envelope.Data.TransactionID,
			"outcome":            envelope.Data.Outcome,
			"service_type":       envelope.Data.ServiceType,
			"service_id":         envelope.Data.ServiceID,
			"months":             envelope.Data.Months,
			"amount":             envelope.Data.Amount,
			"shortage_amount":    envelope.Data.ShortageAmount,
			"expires_at_after":   envelope.Data.ExpiresAtAfter,
		})
		if err != nil {
			return RenewServiceResult{}, err
		}
		result.BotMessage = &message
	}
	return result, nil
}

type autoBotCommand struct {
	Email           string
	UserID          int
	Days            int
	IncludeExpired  bool
	ServiceType     string
	Amount          int
	ExpiresMinutes  int
	IntentID        int
	IntentCode      string
	Reference       string
	ServiceID       int
	ServiceName     string
	Months          int
	IsHelp          bool
	HasLookup       bool
	HasAmount       bool
	HasOrderPayment bool
	WalletIntent    bool
	PaymentIntent   bool
	TicketIntent    bool
	AlertIntent     bool
	RenewalIntent   bool
}

func parseAutoBotCommand(body string) autoBotCommand {
	body = strings.TrimSpace(body)
	lower := strings.ToLower(body)
	plain := normalizeText(lower)
	command := autoBotCommand{
		Email:          strings.ToLower(emailPattern.FindString(body)),
		Days:           7,
		ServiceType:    "all",
		ExpiresMinutes: 1440,
		Months:         1,
		IsHelp:         containsAny(plain, "help", "huong dan", "cach dung", "/bot", "/help"),
		IncludeExpired: containsAny(plain, "include_expired true", "include expired true", "gom het han", "bao gom het han", "ca het han"),
		WalletIntent:   containsAny(plain, "tra vi", "so du", "wallet", "balance", "kiem tra vi"),
		PaymentIntent:  amountHintPattern.MatchString(lower) || containsAny(plain, "nap vi", "tao qr", "thanh toan", "chuyen khoan", "qr"),
		TicketIntent:   containsAny(plain, "ticket", "ho tro", "khach", "loi", "khong truy cap", "khong vao duoc", "vps", "hosting", "domain", "proxy"),
		AlertIntent:    containsAny(plain, "alert", "canh bao", "server", "down", "mat ping", "ping", "port", "cpu", "ram", "disk", "service", "timeout", "critical"),
		RenewalIntent:  containsAny(plain, "gia han", "renew", "extend") && !containsAny(plain, "sap het han", "kiem tra", "thong ke", "bao cao"),
	}
	if match := intentIDPattern.FindStringSubmatch(body); len(match) == 2 {
		command.IntentID, _ = strconv.Atoi(match[1])
	}
	if match := intentCodePattern.FindStringSubmatch(body); len(match) == 2 {
		command.IntentCode = strings.TrimSpace(match[1])
	} else if match := quickIntentPattern.FindString(body); match != "" {
		command.IntentCode = strings.TrimSpace(match)
	}
	if match := orderRefPattern.FindStringSubmatch(body); len(match) == 2 {
		command.Reference = strings.TrimSpace(match[1])
	}
	if match := serviceIDPattern.FindStringSubmatch(body); len(match) == 2 {
		command.ServiceID, _ = strconv.Atoi(match[1])
	}
	if match := serviceNamePattern.FindStringSubmatch(body); len(match) == 2 {
		command.ServiceName = cleanServiceName(match[1])
	}
	if match := monthsPattern.FindStringSubmatch(body); len(match) == 3 {
		if value := parseHumanInt(match[1]); value > 0 {
			command.Months = value
		}
	}
	command.HasOrderPayment = command.IntentID > 0 || command.IntentCode != "" || command.Reference != ""

	for _, match := range labeledIntPattern.FindAllStringSubmatch(body, -1) {
		if len(match) != 3 {
			continue
		}
		label := normalizeText(match[1])
		value := parseHumanInt(match[2])
		if value <= 0 {
			continue
		}
		switch {
		case containsAny(label, "user", "uid") || label == "id":
			command.UserID = value
		case containsAny(label, "ngay", "day"):
			command.Days = value
		case containsAny(label, "tien", "amount"):
			command.Amount = value
			command.HasAmount = true
		}
	}

	if command.Days == 7 {
		if match := plainDaysPattern.FindStringSubmatch(body); len(match) == 3 {
			if value := parseHumanInt(match[1]); value >= 0 {
				command.Days = value
			}
		}
	}
	if match := serviceTypePattern.FindStringSubmatch(body); len(match) == 3 {
		if value := normalizeServiceTypeAlias(match[2]); value != "" {
			command.ServiceType = value
		}
	}
	if command.ServiceType == "all" {
		command.ServiceType = inferServiceType(plain)
	}
	if command.Amount == 0 && command.PaymentIntent {
		command.Amount = firstMoneyValue(body)
		command.HasAmount = command.Amount > 0
	}
	command.HasLookup = command.Email != "" || command.UserID > 0
	return command
}

func autoBotCommandLogFields(command autoBotCommand) []any {
	return []any{
		"email", maskLogEmail(command.Email),
		"user_id", command.UserID,
		"days", command.Days,
		"service_type", command.ServiceType,
		"service_id", command.ServiceID,
		"service_name", compactSummary(command.ServiceName, 80),
		"months", command.Months,
		"amount", command.Amount,
		"has_lookup", command.HasLookup,
		"has_amount", command.HasAmount,
		"has_order_payment", command.HasOrderPayment,
		"is_help", command.IsHelp,
		"wallet_intent", command.WalletIntent,
		"payment_intent", command.PaymentIntent,
		"ticket_intent", command.TicketIntent,
		"alert_intent", command.AlertIntent,
		"renewal_intent", command.RenewalIntent,
	}
}

func maskLogEmail(email string) string {
	email = strings.TrimSpace(strings.ToLower(email))
	if email == "" {
		return ""
	}
	parts := strings.SplitN(email, "@", 2)
	if len(parts) != 2 {
		return "***"
	}
	local := []rune(parts[0])
	if len(local) <= 2 {
		return "**@" + parts[1]
	}
	return string(local[:2]) + "***@" + parts[1]
}

func (s *Service) postAutoGuide(ctx context.Context, input botauto.MessageInput, botSlug string, channelSlug string, action string, body string) ([]botauto.BotMessage, error) {
	return s.postAutoText(ctx, input, botSlug, channelSlug, body, map[string]any{
		"source":             "vpsttt_order",
		"action":             action,
		"trigger_message_id": input.MessageID,
	})
}

func (s *Service) postAutoError(ctx context.Context, input botauto.MessageInput, botSlug string, channelSlug string, botName string, err error, guide string) ([]botauto.BotMessage, error) {
	slog.Warn("Order bot tao phan hoi loi",
		"workspace_id", input.WorkspaceID,
		"channel_id", input.ChannelID,
		"message_id", input.MessageID,
		"bot_slug", botSlug,
		"target_channel_slug", channelSlug,
		"bot_name", botName,
		"error", err,
	)
	body := strings.ToUpper(botName) + " · CHƯA THỂ XỬ LÝ\n"
	body += "Chi tiết: " + strings.TrimSpace(err.Error()) + "\n\n"
	body += guide
	return s.postAutoText(ctx, input, botSlug, channelSlug, body, map[string]any{
		"source":             "vpsttt_order",
		"action":             "auto_error",
		"trigger_message_id": input.MessageID,
	})
}

func (s *Service) postAutoText(ctx context.Context, input botauto.MessageInput, botSlug string, channelSlug string, body string, metadata map[string]any) ([]botauto.BotMessage, error) {
	action := any(nil)
	if metadata != nil {
		action = metadata["action"]
	}
	slog.Info("Order bot chuan bi gui phan hoi",
		"workspace_id", input.WorkspaceID,
		"channel_id", input.ChannelID,
		"message_id", input.MessageID,
		"bot_slug", botSlug,
		"target_channel_slug", channelSlug,
		"body_len", len([]rune(body)),
		"action", action,
	)
	message, err := s.postBotMessage(ctx, input.WorkspaceID, botSlug, input.ChannelID, channelSlug, body, metadata)
	if err != nil {
		slog.Warn("Order bot gui phan hoi that bai",
			"workspace_id", input.WorkspaceID,
			"channel_id", input.ChannelID,
			"message_id", input.MessageID,
			"bot_slug", botSlug,
			"target_channel_slug", channelSlug,
			"error", err,
		)
		return nil, err
	}
	return autoBotMessages(&message), nil
}

func autoBotMessages(message *BotMessageDTO) []botauto.BotMessage {
	if message == nil {
		return nil
	}
	return []botauto.BotMessage{{
		ID:          message.ID,
		WorkspaceID: message.WorkspaceID,
		ChannelID:   message.ChannelID,
		BotID:       message.BotID,
		Kind:        message.Kind,
		Body:        message.Body,
		Metadata:    message.Metadata,
		CreatedAt:   message.CreatedAt,
	}}
}

func renewalBotGuide() string {
	return strings.TrimSpace(`GIA HẠN · CÚ PHÁP HỖ TRỢ
Kiểm tra dịch vụ sắp hết hạn:
Email: khach@example.com
Số ngày: 7
Loại dịch vụ: Tất cả

Gia hạn theo mã dịch vụ:
Tôi muốn gia hạn VPS #1234 của tài khoản khach@example.com thêm 1 tháng.

Gia hạn theo tên dịch vụ:
Gia hạn dịch vụ vps-hanoi-01 của tài khoản khach@example.com trong 3 tháng.

Hỗ trợ: Tất cả, VPS, Proxy, Hosting, S3, Drive, WAF, Domain và Separate.`)
}

func paymentBotGuide() string {
	return strings.TrimSpace(`THANH TOÁN · CÚ PHÁP HỖ TRỢ

QR nạp ví
Email: khach@example.com
Số tiền: 200000

QR thanh toán đơn hàng Quick Order
Tạo QR cho đơn hàng
Intent code: QOIABCD1234EFGH5678

Lưu ý: Số tiền nạp tối thiểu là 1.000 VND; mã QR hết hạn sau 24 giờ.`)
}

func ticketBotGuide() string {
	return strings.TrimSpace(`TICKET · CÚ PHÁP HỖ TRỢ
Gửi nội dung sự cố theo mẫu:

Khách: Nguyễn Văn A
Email: khach@example.com
Lỗi: VPS không truy cập được SSH

Tra cứu ví nhanh: Tra ví email@example.com`)
}

func alertBotGuide() string {
	return strings.TrimSpace(`SERVER ALERT · CÚ PHÁP HỖ TRỢ
Gửi cảnh báo vận hành theo mẫu:

Server: vps-01
Lỗi: mất ping 3 phút
Port: 22 timeout
Mức độ: critical`)
}

func formatTicketTriageMessage(body string, command autoBotCommand) string {
	priority := "P3 - Bình thường"
	if containsAny(normalizeText(body), "down", "mat ping", "khong truy cap", "khong vao duoc", "critical", "khach vip", "mat du lieu") {
		priority = "P1 - Cần xử lý ngay"
	} else if containsAny(normalizeText(body), "loi", "timeout", "cham", "khong gui duoc", "khong nhan duoc") {
		priority = "P2 - Ưu tiên cao"
	}
	customer := firstNonEmpty(command.Email, "chưa rõ")
	return strings.TrimSpace(fmt.Sprintf(`TICKET · Đã tiếp nhận
Khách hàng: %s
Mức ưu tiên: %s
Tóm tắt: %s

Hướng xử lý
• Xác minh tài khoản và dịch vụ liên quan.
• Kiểm tra trạng thái dịch vụ cùng log gần nhất.
• Chuyển #server-alert hoặc #ky-thuat nếu là sự cố hạ tầng.
• Cập nhật tiến độ trong luồng ticket này.`, customer, priority, compactSummary(body, 180)))
}

func formatServerAlertMessage(body string) string {
	plain := normalizeText(body)
	severity := "Warning"
	if containsAny(plain, "critical", "down", "mat ping", "timeout", "port 22", "het disk", "full disk") {
		severity = "Critical"
	}
	signals := make([]string, 0, 4)
	if containsAny(plain, "ping", "mat ping") {
		signals = append(signals, "network/ping")
	}
	if containsAny(plain, "port", "timeout") {
		signals = append(signals, "port/service")
	}
	if containsAny(plain, "cpu", "ram", "memory", "disk") {
		signals = append(signals, "resource")
	}
	if len(signals) == 0 {
		signals = append(signals, "general")
	}
	return strings.TrimSpace(fmt.Sprintf(`VẬN HÀNH · CẢNH BÁO %s
Tín hiệu: %s
Tóm tắt: %s

Checklist xử lý
• Kiểm tra ping, traceroute và SSH.
• Kiểm tra CPU, RAM, ổ đĩa và network.
• Kiểm tra service bằng systemctl hoặc docker logs.
• Báo #ticket nếu sự cố đang ảnh hưởng khách hàng.`, strings.ToUpper(severity), strings.Join(signals, ", "), compactSummary(body, 180)))
}

func normalizeServiceTypeAlias(value string) string {
	value = normalizeText(strings.TrimSpace(value))
	value = strings.Trim(value, " .,:;|")
	switch {
	case value == "", strings.HasPrefix(value, "tat ca"), value == "all":
		return "all"
	case strings.HasPrefix(value, "vps"):
		return "vps"
	case strings.HasPrefix(value, "proxy"):
		return "proxy"
	case strings.HasPrefix(value, "hosting"):
		return "hosting"
	case strings.HasPrefix(value, "s3"):
		return "s3"
	case strings.HasPrefix(value, "drive"):
		return "drive"
	case strings.HasPrefix(value, "waf"):
		return "waf"
	case strings.HasPrefix(value, "domain"):
		return "domain"
	case strings.HasPrefix(value, "separate"):
		return "separate"
	default:
		return ""
	}
}

func firstMoneyValue(body string) int {
	best := 0
	for _, match := range regexp.MustCompile(`(?i)\b[0-9][0-9\.\,\s]*[kKmM]?\b`).FindAllString(body, -1) {
		value := parseHumanInt(match)
		if value > best {
			best = value
		}
	}
	return best
}

func parseHumanInt(value string) int {
	value = strings.TrimSpace(value)
	multiplier := 1
	lower := strings.ToLower(value)
	if strings.HasSuffix(lower, "k") {
		multiplier = 1000
		value = value[:len(value)-1]
	} else if strings.HasSuffix(lower, "m") {
		multiplier = 1000000
		value = value[:len(value)-1]
	}
	value = strings.NewReplacer(".", "", ",", "", " ", "").Replace(value)
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0
	}
	return parsed * multiplier
}

func compactSummary(value string, limit int) string {
	value = strings.Join(strings.Fields(value), " ")
	if len([]rune(value)) <= limit {
		return value
	}
	runes := []rune(value)
	return string(runes[:limit]) + "..."
}

func containsAny(value string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(value, needle) {
			return true
		}
	}
	return false
}

func normalizeText(value string) string {
	value = strings.ToLower(value)
	return strings.NewReplacer(
		"à", "a", "á", "a", "ạ", "a", "ả", "a", "ã", "a", "â", "a", "ầ", "a", "ấ", "a", "ậ", "a", "ẩ", "a", "ẫ", "a", "ă", "a", "ằ", "a", "ắ", "a", "ặ", "a", "ẳ", "a", "ẵ", "a",
		"è", "e", "é", "e", "ẹ", "e", "ẻ", "e", "ẽ", "e", "ê", "e", "ề", "e", "ế", "e", "ệ", "e", "ể", "e", "ễ", "e",
		"ì", "i", "í", "i", "ị", "i", "ỉ", "i", "ĩ", "i",
		"ò", "o", "ó", "o", "ọ", "o", "ỏ", "o", "õ", "o", "ô", "o", "ồ", "o", "ố", "o", "ộ", "o", "ổ", "o", "ỗ", "o", "ơ", "o", "ờ", "o", "ớ", "o", "ợ", "o", "ở", "o", "ỡ", "o",
		"ù", "u", "ú", "u", "ụ", "u", "ủ", "u", "ũ", "u", "ư", "u", "ừ", "u", "ứ", "u", "ự", "u", "ử", "u", "ữ", "u",
		"ỳ", "y", "ý", "y", "ỵ", "y", "ỷ", "y", "ỹ", "y",
		"đ", "d",
	).Replace(value)
}

func (s *Service) ensurePermission(ctx context.Context, userID string, workspaceID string, permissionCode string) error {
	if err := s.ensureVPSTTTWorkspace(ctx, workspaceID); err != nil {
		return err
	}
	if s.checker == nil {
		return nil
	}
	allowed, err := s.checker.HasWorkspacePermission(ctx, strings.TrimSpace(userID), strings.TrimSpace(workspaceID), permissionCode)
	if err != nil {
		return err
	}
	if !allowed {
		return apperrors.Forbidden("Bạn không có quyền sử dụng bot order VPSTTT.")
	}
	return nil
}

func (s *Service) ensureRenewalPermission(ctx context.Context, userID string, workspaceID string, targetEmail string) error {
	if err := s.ensureVPSTTTWorkspace(ctx, workspaceID); err != nil {
		return err
	}
	if s.checker == nil {
		return nil
	}
	billingAllowed, err := s.checker.HasWorkspacePermission(ctx, strings.TrimSpace(userID), strings.TrimSpace(workspaceID), PermissionOrderBilling)
	if err != nil {
		return err
	}
	if billingAllowed {
		return nil
	}
	selfServiceAllowed, err := s.checker.HasWorkspacePermission(ctx, strings.TrimSpace(userID), strings.TrimSpace(workspaceID), PermissionOrderPaymentRequest)
	if err != nil {
		return err
	}
	if !selfServiceAllowed || s.repo == nil || normalizeEmail(targetEmail) == "" {
		return apperrors.Forbidden("Bạn không có quyền gia hạn dịch vụ của tài khoản này.")
	}
	actorEmail, err := s.repo.UserEmailByID(ctx, strings.TrimSpace(userID))
	if err != nil {
		return err
	}
	if normalizeEmail(actorEmail) != normalizeEmail(targetEmail) {
		return apperrors.Forbidden("Email yêu cầu gia hạn phải trùng email tài khoản chat đã xác minh. Vui lòng dùng đúng tài khoản hoặc liên hệ #ke-toan.")
	}
	return nil
}

func (s *Service) ensureVPSTTTWorkspace(ctx context.Context, workspaceID string) error {
	if s.repo == nil {
		return apperrors.Internal("Order bot repository chưa được cấu hình.")
	}
	allowed, err := s.repo.WorkspaceSupportsOrderBot(ctx, strings.TrimSpace(workspaceID))
	if err != nil {
		return err
	}
	if !allowed {
		return apperrors.Forbidden("Bot order VPSTTT chỉ khả dụng trong zone nội bộ VPSTTT.")
	}
	return nil
}

func (s *Service) ensureConfigured() error {
	if s.client == nil || !s.client.Configured() {
		return apperrors.Internal("Chưa cấu hình ORDER_INTERNAL_API_KEY cho bot order VPSTTT.")
	}
	return nil
}

func (s *Service) ensureQuickOrderConfigured() error {
	if s.client == nil || !s.client.QuickOrderConfigured() {
		return apperrors.Internal("Chưa cấu hình ORDER_QUICK_ORDER_KEY cho QR thanh toán đơn hàng.")
	}
	return nil
}

func (s *Service) postBotMessage(ctx context.Context, workspaceID string, botSlug string, channelID string, channelSlug string, body string, metadata map[string]any) (BotMessageDTO, error) {
	slog.Info("Order bot bat dau postBotMessage",
		"workspace_id", strings.TrimSpace(workspaceID),
		"channel_id", strings.TrimSpace(channelID),
		"bot_slug", botSlug,
		"target_channel_slug", channelSlug,
	)
	if s.repo == nil {
		return BotMessageDTO{}, apperrors.Internal("Order bot repository chưa được cấu hình.")
	}
	if metadata == nil {
		metadata = map[string]any{}
	}
	metadata["generated_at"] = s.now().Format(time.RFC3339)
	rawMetadata, err := json.Marshal(metadata)
	if err != nil {
		return BotMessageDTO{}, err
	}
	slog.Info("Order bot insert message vao database",
		"workspace_id", strings.TrimSpace(workspaceID),
		"channel_id", strings.TrimSpace(channelID),
		"bot_slug", botSlug,
		"target_channel_slug", channelSlug,
		"action", metadata["action"],
	)
	message, err := s.repo.SendBotMessage(ctx, SendBotMessageParams{
		WorkspaceID: strings.TrimSpace(workspaceID),
		BotSlug:     botSlug,
		ChannelID:   strings.TrimSpace(channelID),
		ChannelSlug: channelSlug,
		Body:        body,
		Metadata:    rawMetadata,
	})
	if errors.Is(err, ErrOrderBotTargetNotFound) {
		slog.Warn("Order bot khong tim thay bot hoac bot_installation cho kenh dich",
			"workspace_id", strings.TrimSpace(workspaceID),
			"channel_id", strings.TrimSpace(channelID),
			"bot_slug", botSlug,
			"target_channel_slug", channelSlug,
		)
		return BotMessageDTO{}, apperrors.BadRequest("ORDER_BOT_NOT_INSTALLED", "Bot order chưa được cài vào kênh đích.")
	}
	if err != nil {
		slog.Warn("Order bot insert message vao database that bai",
			"workspace_id", strings.TrimSpace(workspaceID),
			"channel_id", strings.TrimSpace(channelID),
			"bot_slug", botSlug,
			"target_channel_slug", channelSlug,
			"error", err,
		)
		return BotMessageDTO{}, err
	}
	slog.Info("Order bot insert message vao database thanh cong",
		"workspace_id", message.WorkspaceID,
		"channel_id", message.ChannelID,
		"message_id", message.ID,
		"bot_id", message.BotID,
		"bot_slug", botSlug,
	)
	return message, nil
}

func normalizeLookup(email string, userID int) (UserLookupRequest, error) {
	email = normalizeEmail(email)
	if email == "" && userID <= 0 {
		return UserLookupRequest{}, apperrors.BadRequest("VALIDATION_ERROR", "Cần truyền email hoặc user_id khách hàng.")
	}
	if userID < 0 {
		return UserLookupRequest{}, apperrors.BadRequest("VALIDATION_ERROR", "user_id không hợp lệ.")
	}
	return UserLookupRequest{Email: email, UserID: userID}, nil
}

func normalizeEmail(email string) string {
	email = strings.ToLower(strings.TrimSpace(email))
	if strings.ContainsAny(email, "\r\n\t") || len(email) > 254 {
		return ""
	}
	return email
}

func normalizeServiceType(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	switch value {
	case "", "all", "vps", "proxy", "hosting", "s3", "drive", "waf", "domain", "separate":
		return value
	default:
		return ""
	}
}

func inferServiceType(plain string) string {
	for _, serviceType := range []string{"proxy", "hosting", "drive", "domain", "separate", "vps", "s3", "waf"} {
		pattern := regexp.MustCompile(`(^|[^a-z0-9])` + regexp.QuoteMeta(serviceType) + `([^a-z0-9]|$)`)
		if pattern.MatchString(plain) {
			return serviceType
		}
	}
	return "all"
}

func cleanServiceName(value string) string {
	value = strings.TrimSpace(value)
	value = strings.Trim(value, "\"'`.,:;-")
	runes := []rune(value)
	if len(runes) > 160 {
		return string(runes[:160])
	}
	return value
}

func validateOptionalChannelID(channelID string) error {
	channelID = strings.TrimSpace(channelID)
	if channelID == "" {
		return nil
	}
	if !uuidPattern.MatchString(channelID) {
		return apperrors.BadRequest("VALIDATION_ERROR", "channel_id không hợp lệ.")
	}
	return nil
}

func shouldPost(value *bool) bool {
	return value == nil || *value
}

func ensureRemoteOK(ok bool, status string, message string) error {
	if ok || strings.EqualFold(status, "success") {
		return nil
	}
	if strings.TrimSpace(message) == "" {
		message = "Order API trả về lỗi."
	}
	return apperrors.BadRequest("ORDER_API_ERROR", message)
}

func mapOrderClientError(err error) error {
	if err == nil {
		return nil
	}

	var upstream *UpstreamError
	if errors.As(err, &upstream) {
		code := "ORDER_API_UPSTREAM_ERROR"
		message := "Order API không xử lý được yêu cầu."
		responseStatus := http.StatusBadGateway
		switch upstream.StatusCode {
		case http.StatusUnauthorized:
			code = "ORDER_API_UNAUTHORIZED"
			message = "Order API từ chối API key."
		case http.StatusForbidden:
			code = "ORDER_API_FORBIDDEN"
			message = "Order API từ chối truy cập."
		case http.StatusTooManyRequests:
			code = "ORDER_API_RATE_LIMITED"
			message = "Order API đang giới hạn tần suất yêu cầu."
			responseStatus = http.StatusServiceUnavailable
		case http.StatusServiceUnavailable:
			code = "ORDER_API_UNAVAILABLE"
			message = "Order API chưa sẵn sàng."
			responseStatus = http.StatusServiceUnavailable
		}
		if detail := strings.TrimSpace(upstream.Message); detail != "" {
			message += " " + detail
		}
		appErr := apperrors.New(code, message, responseStatus)
		appErr.Details = map[string]any{"upstream_status": upstream.StatusCode}
		return appErr
	}

	return apperrors.New(
		"ORDER_API_UNAVAILABLE",
		"Không kết nối được tới Order API. Vui lòng kiểm tra kết nối và thử lại.",
		http.StatusServiceUnavailable,
	)
}

func mapRenewalClientError(err error) error {
	var upstream *UpstreamError
	if errors.As(err, &upstream) && (upstream.StatusCode == http.StatusNotFound || upstream.StatusCode == http.StatusMethodNotAllowed) {
		return apperrors.New(
			"ORDER_RENEWAL_NOT_SUPPORTED",
			"Order API chưa bật endpoint gia hạn nội bộ an toàn. Không có số dư nào bị trừ. Vui lòng liên hệ Zalo OA VPSTTT hoặc gửi yêu cầu tại #ke-toan để được hỗ trợ.",
			http.StatusServiceUnavailable,
		)
	}
	return mapOrderClientError(err)
}

func formatWalletBalanceMessage(data WalletBalanceData) string {
	var builder strings.Builder
	builder.WriteString("CSKH · SỐ DƯ VÍ\n")
	builder.WriteString("Khách hàng: " + customerLine(data.Name, data.Email, data.UserID) + "\n")
	builder.WriteString("Số dư khả dụng: " + formatVND(balanceAmount(data.BalanceVND, data.Balance, data.Money)) + "\n")
	if data.Agency != "" {
		builder.WriteString("Cấp đại lý: " + data.Agency + "\n")
	}
	if len(data.Services) > 0 {
		builder.WriteString("Dịch vụ đang dùng: " + formatServicesMap(data.Services) + "\n")
	}
	return strings.TrimSpace(builder.String())
}

func formatDepositQRMessage(data WalletDepositQRData) string {
	var builder strings.Builder
	builder.WriteString("THANH TOÁN · QR NẠP VÍ\n")
	builder.WriteString("Khách hàng: " + customerLine(data.Name, data.Email, data.UserID) + "\n")
	builder.WriteString("Số tiền: " + formatVND(data.Amount) + "\n")
	if data.Reference != "" {
		builder.WriteString("Mã tham chiếu: " + data.Reference + "\n")
	}
	transferContent := firstNonEmpty(data.TransferContent, data.Bank.TransferContent)
	if transferContent != "" {
		builder.WriteString("Nội dung chuyển khoản: " + transferContent + "\n")
	}
	if data.Bank.BankCode != "" || data.Bank.AccountNumber != "" {
		builder.WriteString("Ngân hàng: " + strings.TrimSpace(data.Bank.BankCode+" "+data.Bank.AccountNumber) + "\n")
	}
	if data.Bank.AccountName != "" {
		builder.WriteString("Chủ tài khoản: " + data.Bank.AccountName + "\n")
	}
	if data.ExpiresAt != "" {
		builder.WriteString("Hết hạn: " + data.ExpiresAt + "\n")
	}
	builder.WriteString("\nVui lòng quét mã QR bên dưới và giữ nguyên nội dung chuyển khoản.")
	return strings.TrimSpace(builder.String())
}

func formatOrderPaymentQRMessage(data OrderPaymentQRData) string {
	var builder strings.Builder
	builder.WriteString("THANH TOÁN · QR ĐƠN HÀNG\n")
	if data.ExternalOrderID != "" {
		builder.WriteString("Đơn hàng: " + data.ExternalOrderID + "\n")
	} else if data.IntentID > 0 {
		builder.WriteString("Intent ID: " + strconv.Itoa(data.IntentID) + "\n")
	}
	if data.CustomerEmail != "" {
		builder.WriteString("Khách hàng: " + data.CustomerEmail + "\n")
	}
	builder.WriteString("Số tiền: " + formatVND(data.Amount) + "\n")
	if data.Reference != "" {
		builder.WriteString("Mã tham chiếu: " + data.Reference + "\n")
	}
	if data.ExpiresAt != "" {
		builder.WriteString("Hết hạn: " + data.ExpiresAt + "\n")
	}
	builder.WriteString("\nVui lòng quét mã QR bên dưới để hoàn tất thanh toán.")
	return strings.TrimSpace(builder.String())
}

func formatExpiringServicesMessage(data ServicesExpiringData) string {
	var builder strings.Builder
	builder.WriteString("GIA HẠN · DỊCH VỤ SẮP HẾT HẠN\n")
	builder.WriteString("Khách hàng: " + customerLine(data.User.Name, data.User.Email, data.User.UserID) + "\n")
	if data.Days > 0 {
		builder.WriteString("Khoảng kiểm tra: " + strconv.Itoa(data.Days) + " ngày\n")
	}
	builder.WriteString(fmt.Sprintf(
		"Tổng quan: %d dịch vụ · %d đã hết hạn · %d sắp hết hạn · %d tắt tự động gia hạn\n",
		data.Summary.Total,
		data.Summary.Expired,
		data.Summary.Expiring,
		data.Summary.AutoRenewOff,
	))
	if len(data.Summary.ByType) > 0 {
		builder.WriteString("Phân loại: " + formatServicesMap(data.Summary.ByType) + "\n")
	}
	if len(data.Items) == 0 {
		builder.WriteString("\nKhông có dịch vụ cần xử lý trong khoảng thời gian này.")
		return strings.TrimSpace(builder.String())
	}
	builder.WriteString("\nDịch vụ cần xử lý\n")
	limit := len(data.Items)
	if limit > 10 {
		limit = 10
	}
	for index := 0; index < limit; index++ {
		item := data.Items[index]
		builder.WriteString("• " + expiringItemLine(item) + "\n")
	}
	if len(data.Items) > limit {
		builder.WriteString("... và " + strconv.Itoa(len(data.Items)-limit) + " dịch vụ khác.\n")
	}
	return strings.TrimSpace(builder.String())
}

func formatRenewServiceMessage(data RenewServiceData) string {
	var builder strings.Builder
	outcome := normalizeText(strings.TrimSpace(data.Outcome))
	insufficient := data.ShortageAmount > 0 || containsAny(outcome, "insufficient", "not enough", "khong du", "thieu tien")
	if insufficient {
		builder.WriteString("GIA HẠN · SỐ DƯ KHÔNG ĐỦ\n")
	} else if containsAny(outcome, "success", "renewed", "completed", "thanh cong", "da gia han") {
		builder.WriteString("GIA HẠN · ĐÃ HOÀN TẤT\n")
	} else {
		builder.WriteString("GIA HẠN · ĐÃ TIẾP NHẬN\n")
	}
	builder.WriteString("Khách hàng: " + customerLine(data.User.Name, data.User.Email, data.User.UserID) + "\n")
	serviceType := firstNonEmpty(data.ServiceType, "Dịch vụ")
	serviceName := firstNonEmpty(data.ServiceName, "không có tên")
	if data.ServiceID > 0 {
		builder.WriteString("Dịch vụ: " + serviceType + " #" + strconv.Itoa(data.ServiceID) + " - " + serviceName + "\n")
	} else {
		builder.WriteString("Dịch vụ: " + serviceType + " - " + serviceName + "\n")
	}
	if data.Months > 0 {
		builder.WriteString("Thời gian gia hạn: " + strconv.Itoa(data.Months) + " tháng\n")
	}
	if data.Amount > 0 {
		builder.WriteString("Chi phí: " + formatVND(data.Amount) + "\n")
	}
	if data.BalanceBefore > 0 || insufficient {
		builder.WriteString("Số dư ví: " + formatVND(data.BalanceBefore) + "\n")
	}
	if insufficient {
		shortage := data.ShortageAmount
		if shortage <= 0 && data.Amount > data.BalanceBefore {
			shortage = data.Amount - data.BalanceBefore
		}
		builder.WriteString("Số tiền còn thiếu: " + formatVND(shortage) + "\n")
		builder.WriteString("\nVui lòng liên hệ Zalo OA VPSTTT hoặc gửi yêu cầu tại #ke-toan để nạp thêm tiền vào ví.")
		return strings.TrimSpace(builder.String())
	}
	if data.BalanceAfter >= 0 && data.Amount > 0 {
		builder.WriteString("Số dư sau gia hạn: " + formatVND(data.BalanceAfter) + "\n")
	}
	if data.ExpiresAtAfter != "" {
		builder.WriteString("Hạn sử dụng mới: " + data.ExpiresAtAfter + "\n")
	}
	if data.TransactionID != "" {
		builder.WriteString("Mã giao dịch: " + data.TransactionID + "\n")
	}
	return strings.TrimSpace(builder.String())
}

func expiringItemLine(item ServiceExpiringItem) string {
	name := firstNonEmpty(item.Name, item.Meta, "Dịch vụ")
	serviceType := firstNonEmpty(item.ServiceType, strings.ToUpper(item.ServiceTypeKey), "SERVICE")
	parts := []string{fmt.Sprintf("%s #%d %s", serviceType, item.ServiceID, name)}
	if item.DaysRemaining < 0 {
		parts = append(parts, "đã hết hạn "+strconv.Itoa(-item.DaysRemaining)+" ngày")
	} else {
		parts = append(parts, "còn "+strconv.Itoa(item.DaysRemaining)+" ngày")
	}
	if item.ExpiresAt != "" {
		parts = append(parts, "hết hạn "+item.ExpiresAt)
	}
	if item.RenewalTransferContent != "" {
		parts = append(parts, "ND gia hạn: "+item.RenewalTransferContent)
	}
	return strings.Join(parts, " | ")
}

func customerLine(name string, email string, userID int) string {
	parts := make([]string, 0, 3)
	if strings.TrimSpace(name) != "" {
		parts = append(parts, strings.TrimSpace(name))
	}
	if strings.TrimSpace(email) != "" {
		parts = append(parts, strings.TrimSpace(email))
	}
	if userID > 0 {
		parts = append(parts, "#"+strconv.Itoa(userID))
	}
	if len(parts) == 0 {
		return "Không rõ"
	}
	return strings.Join(parts, " - ")
}

func balanceAmount(balanceVND int, balance float64, money float64) int {
	if balanceVND != 0 {
		return balanceVND
	}
	if balance != 0 {
		return int(balance)
	}
	return int(money)
}

func formatVND(amount int) string {
	sign := ""
	if amount < 0 {
		sign = "-"
		amount = -amount
	}
	raw := strconv.Itoa(amount)
	var chunks []string
	for len(raw) > 3 {
		chunks = append([]string{raw[len(raw)-3:]}, chunks...)
		raw = raw[:len(raw)-3]
	}
	chunks = append([]string{raw}, chunks...)
	return sign + strings.Join(chunks, ".") + " VND"
}

func formatServicesMap(values map[string]int) string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, strings.ToUpper(key)+" "+strconv.Itoa(values[key]))
	}
	return strings.Join(parts, " · ")
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
