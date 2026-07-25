package application

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	callsdomain "github.com/duclamdev/application-chat/backend/internal/modules/calls/domain"
	apperrors "github.com/duclamdev/application-chat/backend/internal/shared/errors"
)

type PermissionChecker interface {
	HasWorkspacePermission(ctx context.Context, userID string, workspaceID string, permissionCode string) (bool, error)
}

type Repository interface {
	Create(ctx context.Context, params CreateParams) (callsdomain.Call, error)
	Get(ctx context.Context, workspaceID string, callID string) (callsdomain.Call, error)
	UpdateStatus(ctx context.Context, params StatusParams) (callsdomain.Call, error)
	ExpireRingingCall(ctx context.Context, workspaceID string, callID string, before time.Time) (callsdomain.Call, error)
	ExpireRinging(ctx context.Context, before time.Time, limit int) ([]callsdomain.Call, error)
	CreateSignal(ctx context.Context, params SignalParams) (callsdomain.Signal, error)
	CreateCallMessage(ctx context.Context, params CallMessageParams) error
	WorkspaceZoneID(ctx context.Context, workspaceID string) (string, error)
}

type RealtimePublisher interface {
	Publish(ctx context.Context, event RealtimeEvent) error
}

type Service struct {
	repo          Repository
	checker       PermissionChecker
	realtime      RealtimePublisher
	notifications NotificationPublisher
	ringTimeout   time.Duration
}

type NotificationPublisher interface {
	NotifyIncomingCall(ctx context.Context, call CallNotification) error
	NotifyCallTerminal(ctx context.Context, call CallNotification) error
}

type CallNotification struct {
	ID              string
	WorkspaceID     string
	ChannelID       string
	InitiatorUserID string
	TargetUserID    string
	Mode            string
	Status          string
}

type CreateInput struct {
	ActorUserID  string
	WorkspaceID  string
	ChannelID    string
	TargetUserID string
	ClientCallID string
	Mode         string
	Metadata     json.RawMessage
}

type CreateParams struct {
	WorkspaceID  string
	ChannelID    string
	InitiatorID  string
	TargetUserID string
	ClientCallID string
	Mode         string
	Metadata     []byte
}

type StatusInput struct {
	ActorUserID string
	WorkspaceID string
	CallID      string
	Action      string
	Reason      string
}

type StatusParams struct {
	WorkspaceID    string
	CallID         string
	Status         string
	ExpectedStatus string
	ActorUserID    string
}

type SignalInput struct {
	ActorUserID string
	WorkspaceID string
	CallID      string
	SignalType  string
	Payload     json.RawMessage
}

type SignalParams struct {
	WorkspaceID  string
	CallID       string
	SenderUserID string
	SignalType   string
	Payload      []byte
}

type CallMessageParams struct {
	WorkspaceID string
	ChannelID   string
	CallID      string
	SenderID    string
	Body        string
	Metadata    []byte
}

type CallDTO struct {
	ID              string          `json:"id"`
	WorkspaceID     string          `json:"workspace_id"`
	ChannelID       string          `json:"channel_id"`
	InitiatorUserID string          `json:"initiator_user_id"`
	TargetUserID    string          `json:"target_user_id"`
	ClientCallID    *string         `json:"client_call_id,omitempty"`
	Mode            string          `json:"mode"`
	Status          string          `json:"status"`
	Metadata        json.RawMessage `json:"metadata"`
	StartedAt       *string         `json:"started_at,omitempty"`
	EndedAt         *string         `json:"ended_at,omitempty"`
	CreatedAt       string          `json:"created_at"`
	UpdatedAt       string          `json:"updated_at"`
}

type SignalDTO struct {
	ID           string          `json:"id"`
	WorkspaceID  string          `json:"workspace_id"`
	CallID       string          `json:"call_id"`
	SenderUserID string          `json:"sender_user_id"`
	SignalType   string          `json:"signal_type"`
	Payload      json.RawMessage `json:"payload"`
	CreatedAt    string          `json:"created_at"`
}

type RealtimeEvent struct {
	Type         string
	ZoneID       string
	WorkspaceID  string
	ChannelID    string
	ActorUserID  string
	TargetUserID string
	Payload      map[string]any
}

func NewService(repo Repository, checker PermissionChecker, realtime RealtimePublisher, notifications ...NotificationPublisher) *Service {
	service := &Service{
		repo:        repo,
		checker:     checker,
		realtime:    realtime,
		ringTimeout: 30 * time.Second,
	}
	if len(notifications) > 0 {
		service.notifications = notifications[0]
	}
	return service
}

func (s *Service) SetRingTimeout(timeout time.Duration) {
	if timeout > 0 {
		s.ringTimeout = timeout
	}
}

func (s *Service) Create(ctx context.Context, input CreateInput) (CallDTO, error) {
	userID := strings.TrimSpace(input.ActorUserID)
	workspaceID := strings.TrimSpace(input.WorkspaceID)
	if err := s.ensureWorkspaceMember(ctx, userID, workspaceID); err != nil {
		return CallDTO{}, err
	}
	mode := normalizeMode(input.Mode)
	if mode == "" {
		return CallDTO{}, apperrors.BadRequest("VALIDATION_ERROR", "Kiểu cuộc gọi phải là audio hoặc video.")
	}
	metadata, err := normalizeJSON(input.Metadata, "Metadata cuộc gọi không phải JSON hợp lệ.")
	if err != nil {
		return CallDTO{}, err
	}
	call, err := s.repo.Create(ctx, CreateParams{
		WorkspaceID:  workspaceID,
		ChannelID:    strings.TrimSpace(input.ChannelID),
		InitiatorID:  userID,
		TargetUserID: strings.TrimSpace(input.TargetUserID),
		ClientCallID: strings.TrimSpace(input.ClientCallID),
		Mode:         mode,
		Metadata:     metadata,
	})
	if err != nil {
		return CallDTO{}, mapCallError(err)
	}
	_ = s.publish(ctx, "CallInvited", call, userID, call.TargetUserID, map[string]any{"reason": "created"})
	_ = s.notifyIncomingCall(ctx, call)
	s.scheduleRingTimeout(call)
	return toCallDTO(call), nil
}

func (s *Service) Get(ctx context.Context, actorUserID string, workspaceID string, callID string) (CallDTO, error) {
	call, err := s.repo.Get(ctx, strings.TrimSpace(workspaceID), strings.TrimSpace(callID))
	if err != nil {
		return CallDTO{}, mapCallError(err)
	}
	if !isParticipant(call, strings.TrimSpace(actorUserID)) {
		return CallDTO{}, apperrors.Forbidden("Bạn không thuộc cuộc gọi này.")
	}
	return toCallDTO(call), nil
}

func (s *Service) ChangeStatus(ctx context.Context, input StatusInput) (CallDTO, error) {
	call, err := s.repo.Get(ctx, strings.TrimSpace(input.WorkspaceID), strings.TrimSpace(input.CallID))
	if err != nil {
		return CallDTO{}, mapCallError(err)
	}
	actorID := strings.TrimSpace(input.ActorUserID)
	nextStatus, eventType, err := nextStatusForAction(call, actorID, strings.TrimSpace(input.Action))
	if err != nil {
		return CallDTO{}, err
	}
	updated, err := s.repo.UpdateStatus(ctx, StatusParams{
		WorkspaceID:    call.WorkspaceID,
		CallID:         call.ID,
		Status:         nextStatus,
		ExpectedStatus: call.Status,
		ActorUserID:    actorID,
	})
	if err != nil {
		return CallDTO{}, mapCallError(err)
	}
	if isTerminalStatus(nextStatus) {
		_ = s.createCallMessage(ctx, updated)
		_ = s.notifyCallTerminal(ctx, updated)
	}
	targetUserID := updated.TargetUserID
	if actorID == updated.TargetUserID {
		targetUserID = updated.InitiatorUserID
	}
	_ = s.publish(ctx, eventType, updated, actorID, targetUserID, map[string]any{"reason": strings.TrimSpace(input.Reason)})
	return toCallDTO(updated), nil
}

func (s *Service) ExpireUnanswered(ctx context.Context, limit int) (int, error) {
	if limit <= 0 {
		limit = 50
	}
	calls, err := s.repo.ExpireRinging(ctx, time.Now().UTC().Add(-s.ringTimeout), limit)
	if err != nil {
		return 0, err
	}
	for _, call := range calls {
		_ = s.createCallMessage(ctx, call)
		_ = s.notifyCallTerminal(ctx, call)
		_ = s.publish(ctx, "CallMissed", call, "system", call.TargetUserID, map[string]any{"reason": "no_answer"})
	}
	return len(calls), nil
}

func (s *Service) scheduleRingTimeout(call callsdomain.Call) {
	timeout := s.ringTimeout
	if timeout <= 0 {
		return
	}
	time.AfterFunc(timeout, func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		expired, err := s.repo.ExpireRingingCall(ctx, call.WorkspaceID, call.ID, time.Now().UTC())
		if err != nil {
			return
		}
		_ = s.createCallMessage(ctx, expired)
		_ = s.notifyCallTerminal(ctx, expired)
		_ = s.publish(ctx, "CallMissed", expired, "system", expired.TargetUserID, map[string]any{"reason": "no_answer"})
	})
}

func (s *Service) SendSignal(ctx context.Context, input SignalInput) (SignalDTO, error) {
	call, err := s.repo.Get(ctx, strings.TrimSpace(input.WorkspaceID), strings.TrimSpace(input.CallID))
	if err != nil {
		return SignalDTO{}, mapCallError(err)
	}
	actorID := strings.TrimSpace(input.ActorUserID)
	if !isParticipant(call, actorID) {
		return SignalDTO{}, apperrors.Forbidden("Bạn không thuộc cuộc gọi này.")
	}
	signalType := normalizeSignalType(input.SignalType)
	if signalType == "" {
		return SignalDTO{}, apperrors.BadRequest("VALIDATION_ERROR", "Loại tín hiệu cuộc gọi không hợp lệ.")
	}
	if call.Status != "accepted" {
		return SignalDTO{}, apperrors.Conflict(
			"CALL_INVALID_TRANSITION",
			"Chỉ gửi tín hiệu media sau khi cuộc gọi được chấp nhận.",
		)
	}
	if signalType == "offer" && actorID != call.InitiatorUserID {
		return SignalDTO{}, apperrors.Forbidden("Chỉ người khởi tạo được gửi SDP offer.")
	}
	if signalType == "answer" && actorID != call.TargetUserID {
		return SignalDTO{}, apperrors.Forbidden("Chỉ người nhận được gửi SDP answer.")
	}
	if signalType == "ready" && actorID != call.TargetUserID {
		return SignalDTO{}, apperrors.Forbidden("Chỉ người nhận được báo media đã sẵn sàng.")
	}
	payload, err := normalizeJSON(input.Payload, "Payload tín hiệu cuộc gọi không phải JSON hợp lệ.")
	if err != nil {
		return SignalDTO{}, err
	}
	signal, err := s.repo.CreateSignal(ctx, SignalParams{
		WorkspaceID:  call.WorkspaceID,
		CallID:       call.ID,
		SenderUserID: actorID,
		SignalType:   signalType,
		Payload:      payload,
	})
	if err != nil {
		return SignalDTO{}, err
	}
	targetUserID := call.TargetUserID
	if actorID == call.TargetUserID {
		targetUserID = call.InitiatorUserID
	}
	if err := s.publish(ctx, signalEventType(signalType), call, actorID, targetUserID, map[string]any{
		"signal": toSignalDTO(signal).Payload,
	}); err != nil {
		return SignalDTO{}, err
	}
	return toSignalDTO(signal), nil
}

func (s *Service) ensureWorkspaceMember(ctx context.Context, userID string, workspaceID string) error {
	if workspaceID == "" {
		return apperrors.BadRequest("WORKSPACE_REQUIRED", "workspace_id là bắt buộc.")
	}
	if s.checker == nil {
		return nil
	}
	allowed, err := s.checker.HasWorkspacePermission(ctx, userID, workspaceID, "workspace.view_members")
	if err != nil {
		return err
	}
	if !allowed {
		return apperrors.Forbidden("Bạn không thuộc workspace này.")
	}
	return nil
}

func nextStatusForAction(call callsdomain.Call, actorID string, action string) (string, string, error) {
	if !isParticipant(call, actorID) {
		return "", "", apperrors.Forbidden("Bạn không thuộc cuộc gọi này.")
	}
	switch action {
	case "accept":
		if actorID != call.TargetUserID || call.Status != "ringing" {
			return "", "", apperrors.Conflict("CALL_INVALID_TRANSITION", "Không thể chấp nhận cuộc gọi ở trạng thái hiện tại.")
		}
		return "accepted", "CallAccepted", nil
	case "reject":
		if actorID != call.TargetUserID || call.Status != "ringing" {
			return "", "", apperrors.Conflict("CALL_INVALID_TRANSITION", "Không thể từ chối cuộc gọi ở trạng thái hiện tại.")
		}
		return "rejected", "CallRejected", nil
	case "cancel":
		if actorID != call.InitiatorUserID || call.Status != "ringing" {
			return "", "", apperrors.Conflict("CALL_INVALID_TRANSITION", "Không thể hủy cuộc gọi ở trạng thái hiện tại.")
		}
		return "cancelled", "CallCancelled", nil
	case "hangup":
		if call.Status != "accepted" {
			return "", "", apperrors.Conflict("CALL_INVALID_TRANSITION", "Chỉ có thể kết thúc cuộc gọi đã được chấp nhận.")
		}
		return "ended", "CallEnded", nil
	case "miss":
		if call.Status != "ringing" {
			return "", "", apperrors.Conflict("CALL_INVALID_TRANSITION", "Chỉ có thể đánh dấu nhỡ khi cuộc gọi đang đổ chuông.")
		}
		return "missed", "CallMissed", nil
	default:
		return "", "", apperrors.BadRequest("VALIDATION_ERROR", "Hành động cuộc gọi không hợp lệ.")
	}
}

func (s *Service) createCallMessage(ctx context.Context, call callsdomain.Call) error {
	status := "missed"
	if call.Status == "ended" {
		status = "completed"
	}
	duration := 0
	if call.StartedAt != nil && call.EndedAt != nil && call.EndedAt.After(*call.StartedAt) {
		duration = int(call.EndedAt.Sub(*call.StartedAt).Seconds())
	}
	metadata, err := json.Marshal(map[string]any{
		"message_type":      "call",
		"call_id":           call.ID,
		"call_mode":         call.Mode,
		"call_status":       status,
		"initiator_user_id": call.InitiatorUserID,
		"target_user_id":    call.TargetUserID,
		"duration_seconds":  duration,
	})
	if err != nil {
		return err
	}
	body := "Cuộc gọi nhỡ"
	if status == "completed" {
		body = "Cuộc gọi " + callModeLabel(call.Mode) + " đã kết thúc"
	}
	return s.repo.CreateCallMessage(ctx, CallMessageParams{
		WorkspaceID: call.WorkspaceID,
		ChannelID:   call.ChannelID,
		CallID:      call.ID,
		SenderID:    call.InitiatorUserID,
		Body:        body,
		Metadata:    metadata,
	})
}

func (s *Service) notifyIncomingCall(ctx context.Context, call callsdomain.Call) error {
	if s.notifications == nil {
		return nil
	}
	return s.notifications.NotifyIncomingCall(ctx, callNotification(call))
}

func (s *Service) notifyCallTerminal(ctx context.Context, call callsdomain.Call) error {
	if s.notifications == nil {
		return nil
	}
	return s.notifications.NotifyCallTerminal(ctx, callNotification(call))
}

func callNotification(call callsdomain.Call) CallNotification {
	return CallNotification{
		ID:              call.ID,
		WorkspaceID:     call.WorkspaceID,
		ChannelID:       call.ChannelID,
		InitiatorUserID: call.InitiatorUserID,
		TargetUserID:    call.TargetUserID,
		Mode:            call.Mode,
		Status:          call.Status,
	}
}

func (s *Service) publish(ctx context.Context, eventType string, call callsdomain.Call, actorUserID string, targetUserID string, extra map[string]any) error {
	if s.realtime == nil {
		return nil
	}
	zoneID, err := s.repo.WorkspaceZoneID(ctx, call.WorkspaceID)
	if err != nil {
		return err
	}
	payload := callPayload(call)
	payload["actor_user_id"] = strings.TrimSpace(actorUserID)
	for key, value := range extra {
		if value != nil {
			payload[key] = value
		}
	}
	return s.realtime.Publish(ctx, RealtimeEvent{
		Type:         eventType,
		ZoneID:       zoneID,
		WorkspaceID:  call.WorkspaceID,
		ChannelID:    call.ChannelID,
		ActorUserID:  strings.TrimSpace(actorUserID),
		TargetUserID: targetUserID,
		Payload:      payload,
	})
}

func callPayload(call callsdomain.Call) map[string]any {
	return map[string]any{
		"call_id":           call.ID,
		"workspace_id":      call.WorkspaceID,
		"channel_id":        call.ChannelID,
		"initiator_user_id": call.InitiatorUserID,
		"target_user_id":    call.TargetUserID,
		"mode":              call.Mode,
		"status":            call.Status,
	}
}

func normalizeMode(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "audio", "video":
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return ""
	}
}

func normalizeSignalType(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "offer", "answer", "ice_candidate", "ready":
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return ""
	}
}

func signalEventType(signalType string) string {
	switch signalType {
	case "offer":
		return "CallOffer"
	case "answer":
		return "CallAnswer"
	case "ice_candidate":
		return "CallIceCandidate"
	case "ready":
		return "CallReady"
	default:
		return ""
	}
}

func normalizeJSON(value json.RawMessage, message string) ([]byte, error) {
	if len(value) == 0 || strings.TrimSpace(string(value)) == "" || strings.TrimSpace(string(value)) == "null" {
		return []byte(`{}`), nil
	}
	if !json.Valid(value) {
		return nil, apperrors.BadRequest("VALIDATION_ERROR", message)
	}
	return []byte(value), nil
}

func isParticipant(call callsdomain.Call, userID string) bool {
	return userID != "" && (userID == call.InitiatorUserID || userID == call.TargetUserID)
}

func isTerminalStatus(status string) bool {
	return status == "rejected" || status == "cancelled" || status == "ended" || status == "missed"
}

func callModeLabel(mode string) string {
	if mode == "video" {
		return "video"
	}
	return "thoại"
}

func mapCallError(err error) error {
	if errors.Is(err, callsdomain.ErrCallNotFound) {
		return apperrors.NotFound("CALL_NOT_FOUND", "Không tìm thấy cuộc gọi.")
	}
	if errors.Is(err, callsdomain.ErrCallParticipantDenied) {
		return apperrors.Forbidden("Bạn không thuộc cuộc gọi này.")
	}
	if errors.Is(err, callsdomain.ErrCallInvalidTransition) {
		return apperrors.Conflict("CALL_INVALID_TRANSITION", "Trạng thái cuộc gọi không hợp lệ.")
	}
	return err
}

func toCallDTO(call callsdomain.Call) CallDTO {
	metadata := call.Metadata
	if len(metadata) == 0 || !json.Valid(metadata) {
		metadata = json.RawMessage(`{}`)
	}
	return CallDTO{
		ID:              call.ID,
		WorkspaceID:     call.WorkspaceID,
		ChannelID:       call.ChannelID,
		InitiatorUserID: call.InitiatorUserID,
		TargetUserID:    call.TargetUserID,
		ClientCallID:    call.ClientCallID,
		Mode:            call.Mode,
		Status:          call.Status,
		Metadata:        metadata,
		StartedAt:       formatOptionalTime(call.StartedAt),
		EndedAt:         formatOptionalTime(call.EndedAt),
		CreatedAt:       call.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:       call.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

func toSignalDTO(signal callsdomain.Signal) SignalDTO {
	payload := signal.Payload
	if len(payload) == 0 || !json.Valid(payload) {
		payload = json.RawMessage(`{}`)
	}
	return SignalDTO{
		ID:           signal.ID,
		WorkspaceID:  signal.WorkspaceID,
		CallID:       signal.CallID,
		SenderUserID: signal.SenderUserID,
		SignalType:   signal.SignalType,
		Payload:      payload,
		CreatedAt:    signal.CreatedAt.UTC().Format(time.RFC3339),
	}
}

func formatOptionalTime(value *time.Time) *string {
	if value == nil {
		return nil
	}
	formatted := value.UTC().Format(time.RFC3339)
	return &formatted
}
