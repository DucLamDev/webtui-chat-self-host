package application

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	callsdomain "github.com/duclamdev/application-chat/backend/internal/modules/calls/domain"
	apperrors "github.com/duclamdev/application-chat/backend/internal/shared/errors"
)

type fakeCallRepo struct {
	call               callsdomain.Call
	createCalled       bool
	updateStatusCalled bool
	messageCalled      bool
	messageCount       int
	lastMessage        CallMessageParams
	lastSignal         SignalParams
}

func (r *fakeCallRepo) Create(_ context.Context, params CreateParams) (callsdomain.Call, error) {
	r.createCalled = true
	now := time.Now()
	r.call = callsdomain.Call{
		ID:              "call-1",
		WorkspaceID:     params.WorkspaceID,
		ChannelID:       params.ChannelID,
		InitiatorUserID: params.InitiatorID,
		TargetUserID:    params.TargetUserID,
		Mode:            params.Mode,
		Status:          "ringing",
		Metadata:        params.Metadata,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if params.ClientCallID != "" {
		r.call.ClientCallID = &params.ClientCallID
	}
	return r.call, nil
}

func (r *fakeCallRepo) Get(context.Context, string, string) (callsdomain.Call, error) {
	return r.call, nil
}

func (r *fakeCallRepo) FindIncomingRinging(_ context.Context, workspaceID string, targetUserID string) (callsdomain.Call, error) {
	if r.call.WorkspaceID != workspaceID ||
		r.call.TargetUserID != targetUserID ||
		r.call.Status != "ringing" {
		return callsdomain.Call{}, callsdomain.ErrCallNotFound
	}
	return r.call, nil
}

func (r *fakeCallRepo) UpdateStatus(_ context.Context, params StatusParams) (callsdomain.Call, error) {
	r.updateStatusCalled = true
	now := time.Now()
	r.call.Status = params.Status
	r.call.UpdatedAt = now
	if params.Status == "accepted" {
		r.call.StartedAt = &now
	}
	if params.Status == "ended" || params.Status == "missed" || params.Status == "rejected" || params.Status == "cancelled" {
		r.call.EndedAt = &now
	}
	return r.call, nil
}

func (r *fakeCallRepo) ExpireRingingCall(_ context.Context, _ string, _ string, _ time.Time) (callsdomain.Call, error) {
	if r.call.Status != "ringing" {
		return callsdomain.Call{}, callsdomain.ErrCallInvalidTransition
	}
	now := time.Now()
	r.call.Status = "missed"
	r.call.EndedAt = &now
	return r.call, nil
}

func (r *fakeCallRepo) ExpireRinging(_ context.Context, before time.Time, limit int) ([]callsdomain.Call, error) {
	if limit <= 0 || r.call.Status != "ringing" || r.call.CreatedAt.After(before) {
		return []callsdomain.Call{}, nil
	}
	now := time.Now()
	r.call.Status = "missed"
	r.call.EndedAt = &now
	return []callsdomain.Call{r.call}, nil
}

func (r *fakeCallRepo) CreateSignal(_ context.Context, params SignalParams) (callsdomain.Signal, error) {
	r.lastSignal = params
	return callsdomain.Signal{
		ID:           "signal-1",
		WorkspaceID:  params.WorkspaceID,
		CallID:       params.CallID,
		SenderUserID: params.SenderUserID,
		SignalType:   params.SignalType,
		Payload:      params.Payload,
		CreatedAt:    time.Now(),
	}, nil
}

func (r *fakeCallRepo) CreateCallMessage(_ context.Context, params CallMessageParams) error {
	r.messageCalled = true
	r.messageCount++
	r.lastMessage = params
	return nil
}

func (r *fakeCallRepo) WorkspaceZoneID(context.Context, string) (string, error) {
	return "zone-1", nil
}

type fakeCallChecker struct {
	allowed bool
}

func (c fakeCallChecker) HasWorkspacePermission(context.Context, string, string, string) (bool, error) {
	return c.allowed, nil
}

type staticCallBlockChecker struct {
	blocked bool
}

func (c staticCallBlockChecker) IsInteractionBlocked(context.Context, string, string, string) (bool, error) {
	return c.blocked, nil
}

type mutableCallBlockChecker struct {
	blocked bool
}

func (c *mutableCallBlockChecker) IsInteractionBlocked(context.Context, string, string, string) (bool, error) {
	return c.blocked, nil
}

func TestCreateRejectsBlockedInteraction(t *testing.T) {
	service := NewService(nil, fakeCallChecker{allowed: true}, nil)
	service.SetBlockChecker(staticCallBlockChecker{blocked: true})

	_, err := service.Create(context.Background(), CreateInput{
		ActorUserID: "user-a", WorkspaceID: "workspace-1", TargetUserID: "user-b", Mode: "audio",
	})
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) || appErr.Code != "INTERACTION_BLOCKED" {
		t.Fatalf("Create() error = %#v, want INTERACTION_BLOCKED", err)
	}
}

type fakeRealtime struct {
	events []RealtimeEvent
	err    error
}

func (p *fakeRealtime) Publish(_ context.Context, event RealtimeEvent) error {
	p.events = append(p.events, event)
	return p.err
}

type fakeCallNotifications struct {
	incoming []CallNotification
	terminal []CallNotification
}

func (p *fakeCallNotifications) NotifyIncomingCall(_ context.Context, call CallNotification) error {
	p.incoming = append(p.incoming, call)
	return nil
}

func (p *fakeCallNotifications) NotifyCallTerminal(_ context.Context, call CallNotification) error {
	p.terminal = append(p.terminal, call)
	return nil
}

func TestCreateRejectsInvalidModeBeforeRepository(t *testing.T) {
	repo := &fakeCallRepo{}
	service := NewService(repo, fakeCallChecker{allowed: true}, nil)

	_, err := service.Create(context.Background(), CreateInput{
		ActorUserID:  "user-1",
		WorkspaceID:  "workspace-1",
		ChannelID:    "channel-1",
		TargetUserID: "user-2",
		Mode:         "screen-share",
	})
	if err == nil {
		t.Fatal("Create() phải trả lỗi khi mode không hợp lệ")
	}
	if repo.createCalled {
		t.Fatal("Create() không được gọi repository khi validation lỗi")
	}
}

func TestCreatePublishesCallInvited(t *testing.T) {
	repo := &fakeCallRepo{}
	realtime := &fakeRealtime{}
	service := NewService(repo, fakeCallChecker{allowed: true}, realtime)

	call, err := service.Create(context.Background(), CreateInput{
		ActorUserID:  "user-1",
		WorkspaceID:  "workspace-1",
		ChannelID:    "channel-1",
		TargetUserID: "user-2",
		ClientCallID: "client-call-1",
		Mode:         "video",
		Metadata:     json.RawMessage(`{"source":"mobile"}`),
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if call.Status != "ringing" || call.Mode != "video" {
		t.Fatalf("call không đúng: %#v", call)
	}
	if len(realtime.events) != 1 || realtime.events[0].Type != "CallInvited" || realtime.events[0].TargetUserID != "user-2" {
		t.Fatalf("realtime event không đúng: %#v", realtime.events)
	}
}

func TestSendSignalRequiresAcceptedCallAndCorrectSDPRole(t *testing.T) {
	call := callsdomain.Call{
		ID:              "call-1",
		WorkspaceID:     "workspace-1",
		ChannelID:       "channel-1",
		InitiatorUserID: "user-1",
		TargetUserID:    "user-2",
		Mode:            "video",
		Status:          "ringing",
	}
	repo := &fakeCallRepo{call: call}
	service := NewService(repo, fakeCallChecker{allowed: true}, nil)

	_, err := service.SendSignal(context.Background(), SignalInput{
		ActorUserID: "user-1",
		WorkspaceID: "workspace-1",
		CallID:      "call-1",
		SignalType:  "offer",
		Payload:     json.RawMessage(`{"sdp":{"type":"offer","sdp":"v=0"}}`),
	})
	if err == nil || repo.lastSignal.CallID != "" {
		t.Fatalf("ringing call signal = %#v, error = %v; want rejected", repo.lastSignal, err)
	}

	repo.call.Status = "accepted"
	_, err = service.SendSignal(context.Background(), SignalInput{
		ActorUserID: "user-2",
		WorkspaceID: "workspace-1",
		CallID:      "call-1",
		SignalType:  "offer",
		Payload:     json.RawMessage(`{"sdp":{"type":"offer","sdp":"v=0"}}`),
	})
	if err == nil || repo.lastSignal.CallID != "" {
		t.Fatalf("target offer = %#v, error = %v; want rejected", repo.lastSignal, err)
	}
}

func TestSendSignalPublishesValidatedOffer(t *testing.T) {
	repo := &fakeCallRepo{call: callsdomain.Call{
		ID:              "call-1",
		WorkspaceID:     "workspace-1",
		ChannelID:       "channel-1",
		InitiatorUserID: "user-1",
		TargetUserID:    "user-2",
		Mode:            "video",
		Status:          "accepted",
	}}
	realtime := &fakeRealtime{}
	service := NewService(repo, fakeCallChecker{allowed: true}, realtime)

	_, err := service.SendSignal(context.Background(), SignalInput{
		ActorUserID: "user-1",
		WorkspaceID: "workspace-1",
		CallID:      "call-1",
		SignalType:  "offer",
		Payload:     json.RawMessage(`{"sdp":{"type":"offer","sdp":"v=0"}}`),
	})
	if err != nil {
		t.Fatalf("SendSignal() error = %v", err)
	}
	if repo.lastSignal.SignalType != "offer" || len(realtime.events) != 1 {
		t.Fatalf("signal = %#v, events = %#v", repo.lastSignal, realtime.events)
	}
	if realtime.events[0].Type != "CallOffer" || realtime.events[0].TargetUserID != "user-2" {
		t.Fatalf("unexpected realtime event: %#v", realtime.events[0])
	}

	_, err = service.SendSignal(context.Background(), SignalInput{
		ActorUserID: "user-2",
		WorkspaceID: "workspace-1",
		CallID:      "call-1",
		SignalType:  "ready",
		Payload:     json.RawMessage(`{}`),
	})
	if err != nil {
		t.Fatalf("SendSignal(ready) error = %v", err)
	}
	if len(realtime.events) != 2 || realtime.events[1].Type != "CallReady" ||
		realtime.events[1].TargetUserID != "user-1" {
		t.Fatalf("unexpected ready event: %#v", realtime.events)
	}
}

func TestSendSignalReportsRealtimeDeliveryFailure(t *testing.T) {
	repo := &fakeCallRepo{call: callsdomain.Call{
		ID:              "call-1",
		WorkspaceID:     "workspace-1",
		ChannelID:       "channel-1",
		InitiatorUserID: "user-1",
		TargetUserID:    "user-2",
		Mode:            "video",
		Status:          "accepted",
	}}
	realtime := &fakeRealtime{err: errors.New("realtime unavailable")}
	service := NewService(repo, fakeCallChecker{allowed: true}, realtime)

	_, err := service.SendSignal(context.Background(), SignalInput{
		ActorUserID: "user-1",
		WorkspaceID: "workspace-1",
		CallID:      "call-1",
		SignalType:  "offer",
		Payload:     json.RawMessage(`{"sdp":{"type":"offer","sdp":"v=0"}}`),
	})
	if err == nil {
		t.Fatal("SendSignal() must report realtime delivery failure")
	}
}

func TestAcceptRequiresTargetUser(t *testing.T) {
	repo := &fakeCallRepo{
		call: callsdomain.Call{
			ID:              "call-1",
			WorkspaceID:     "workspace-1",
			ChannelID:       "channel-1",
			InitiatorUserID: "user-1",
			TargetUserID:    "user-2",
			Mode:            "audio",
			Status:          "ringing",
			CreatedAt:       time.Now(),
			UpdatedAt:       time.Now(),
		},
	}
	service := NewService(repo, fakeCallChecker{allowed: true}, nil)

	_, err := service.ChangeStatus(context.Background(), StatusInput{
		ActorUserID: "user-1",
		WorkspaceID: "workspace-1",
		CallID:      "call-1",
		Action:      "accept",
	})
	if err == nil {
		t.Fatal("ChangeStatus(accept) phải trả lỗi khi người gọi tự accept")
	}
	if repo.updateStatusCalled {
		t.Fatal("ChangeStatus(accept) không được update khi transition sai")
	}
}

func TestMissCreatesCallMessageAndRealtimeEvent(t *testing.T) {
	repo := &fakeCallRepo{
		call: callsdomain.Call{
			ID:              "call-1",
			WorkspaceID:     "workspace-1",
			ChannelID:       "channel-1",
			InitiatorUserID: "user-1",
			TargetUserID:    "user-2",
			Mode:            "audio",
			Status:          "ringing",
			CreatedAt:       time.Now(),
			UpdatedAt:       time.Now(),
		},
	}
	realtime := &fakeRealtime{}
	service := NewService(repo, fakeCallChecker{allowed: true}, realtime)

	call, err := service.ChangeStatus(context.Background(), StatusInput{
		ActorUserID: "user-2",
		WorkspaceID: "workspace-1",
		CallID:      "call-1",
		Action:      "miss",
	})
	if err != nil {
		t.Fatalf("ChangeStatus(miss) error = %v", err)
	}
	if call.Status != "missed" || !repo.messageCalled {
		t.Fatalf("missed call không đúng: call=%#v messageCalled=%v", call, repo.messageCalled)
	}
	if repo.lastMessage.Body != "Cuộc gọi nhỡ" {
		t.Fatalf("body message cuộc gọi = %q", repo.lastMessage.Body)
	}
	if len(realtime.events) != 1 || realtime.events[0].Type != "CallMissed" || realtime.events[0].TargetUserID != "user-1" {
		t.Fatalf("realtime event không đúng: %#v", realtime.events)
	}
}

func TestAcceptedCallEndsAsCompleted(t *testing.T) {
	repo := &fakeCallRepo{
		call: callsdomain.Call{
			ID:              "call-1",
			WorkspaceID:     "workspace-1",
			ChannelID:       "channel-1",
			InitiatorUserID: "user-1",
			TargetUserID:    "user-2",
			Mode:            "audio",
			Status:          "ringing",
			CreatedAt:       time.Now(),
			UpdatedAt:       time.Now(),
		},
	}
	realtime := &fakeRealtime{}
	notifications := &fakeCallNotifications{}
	service := NewService(repo, fakeCallChecker{allowed: true}, realtime, notifications)

	if _, err := service.ChangeStatus(context.Background(), StatusInput{
		ActorUserID: "user-2",
		WorkspaceID: "workspace-1",
		CallID:      "call-1",
		Action:      "accept",
	}); err != nil {
		t.Fatalf("ChangeStatus(accept) error = %v", err)
	}
	call, err := service.ChangeStatus(context.Background(), StatusInput{
		ActorUserID: "user-1",
		WorkspaceID: "workspace-1",
		CallID:      "call-1",
		Action:      "hangup",
	})
	if err != nil {
		t.Fatalf("ChangeStatus(hangup) error = %v", err)
	}
	if call.Status != "ended" || !repo.messageCalled {
		t.Fatalf("ended call = %#v, messageCalled = %v", call, repo.messageCalled)
	}
	var metadata map[string]any
	if err := json.Unmarshal(repo.lastMessage.Metadata, &metadata); err != nil {
		t.Fatalf("decode call message metadata: %v", err)
	}
	if metadata["call_status"] != "completed" {
		t.Fatalf("call_status = %#v", metadata["call_status"])
	}
	if len(notifications.terminal) != 1 || notifications.terminal[0].Status != "ended" {
		t.Fatalf("terminal notifications = %#v", notifications.terminal)
	}
	if realtime.events[len(realtime.events)-1].Type != "CallEnded" {
		t.Fatalf("realtime events = %#v", realtime.events)
	}
}

func TestFindIncomingRingingReturnsOnlyTargetCall(t *testing.T) {
	repo := &fakeCallRepo{call: callsdomain.Call{
		ID:              "call-1",
		WorkspaceID:     "workspace-1",
		ChannelID:       "channel-1",
		InitiatorUserID: "user-1",
		TargetUserID:    "user-2",
		Mode:            "video",
		Status:          "ringing",
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}}
	service := NewService(repo, fakeCallChecker{allowed: true}, nil)

	call, err := service.FindIncomingRinging(context.Background(), "user-2", "workspace-1")
	if err != nil {
		t.Fatalf("FindIncomingRinging() error = %v", err)
	}
	if call == nil || call.ID != "call-1" || call.TargetUserID != "user-2" {
		t.Fatalf("incoming call = %#v", call)
	}

	missing, err := service.FindIncomingRinging(context.Background(), "user-3", "workspace-1")
	if err != nil {
		t.Fatalf("FindIncomingRinging(missing) error = %v", err)
	}
	if missing != nil {
		t.Fatalf("incoming call for unrelated user = %#v, want nil", missing)
	}
}

func TestBlockAfterInviteHidesAndTerminatesIncomingCall(t *testing.T) {
	repo := &fakeCallRepo{}
	realtime := &fakeRealtime{}
	notifications := &fakeCallNotifications{}
	blocks := &mutableCallBlockChecker{}
	service := NewService(repo, fakeCallChecker{allowed: true}, realtime, notifications)
	service.SetBlockChecker(blocks)

	created, err := service.Create(context.Background(), CreateInput{
		ActorUserID:  "user-1",
		WorkspaceID:  "workspace-1",
		ChannelID:    "channel-1",
		TargetUserID: "user-2",
		Mode:         "audio",
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	blocks.blocked = true

	incoming, err := service.FindIncomingRinging(context.Background(), "user-2", "workspace-1")
	if err != nil {
		t.Fatalf("FindIncomingRinging() error = %v", err)
	}
	if incoming != nil {
		t.Fatalf("incoming = %#v, want hidden after block", incoming)
	}
	if repo.call.ID != created.ID || repo.call.Status != "rejected" {
		t.Fatalf("persisted call = %#v, want rejected", repo.call)
	}
	if repo.messageCalled {
		t.Fatal("automatic block rejection must not create new call content")
	}
	if len(notifications.terminal) != 1 || notifications.terminal[0].Status != "rejected" {
		t.Fatalf("terminal notifications = %#v", notifications.terminal)
	}
	if len(realtime.events) != 2 || realtime.events[1].Type != "CallRejected" ||
		realtime.events[1].Payload["reason"] != "interaction_blocked" {
		t.Fatalf("realtime events = %#v", realtime.events)
	}
}

func TestBlockAfterInviteDeniesAcceptAndSignaling(t *testing.T) {
	blocks := &mutableCallBlockChecker{blocked: true}
	repo := &fakeCallRepo{call: callsdomain.Call{
		ID:              "call-1",
		WorkspaceID:     "workspace-1",
		ChannelID:       "channel-1",
		InitiatorUserID: "user-1",
		TargetUserID:    "user-2",
		Mode:            "audio",
		Status:          "ringing",
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}}
	service := NewService(repo, fakeCallChecker{allowed: true}, nil)
	service.SetBlockChecker(blocks)

	_, err := service.ChangeStatus(context.Background(), StatusInput{
		ActorUserID: "user-2",
		WorkspaceID: "workspace-1",
		CallID:      "call-1",
		Action:      "accept",
	})
	assertCallInteractionBlocked(t, err)
	if repo.updateStatusCalled {
		t.Fatal("blocked accept must not update the call")
	}

	repo.call.Status = "accepted"
	_, err = service.SendSignal(context.Background(), SignalInput{
		ActorUserID: "user-1",
		WorkspaceID: "workspace-1",
		CallID:      "call-1",
		SignalType:  "offer",
		Payload:     json.RawMessage(`{"sdp":{"type":"offer","sdp":"v=0"}}`),
	})
	assertCallInteractionBlocked(t, err)
	if repo.lastSignal.CallID != "" {
		t.Fatalf("blocked signal was persisted: %#v", repo.lastSignal)
	}
}

func TestBlockAllowsTerminalCallCleanup(t *testing.T) {
	tests := []struct {
		name   string
		status string
		actor  string
		action string
		want   string
	}{
		{name: "recipient rejects ringing", status: "ringing", actor: "user-2", action: "reject", want: "rejected"},
		{name: "caller cancels ringing", status: "ringing", actor: "user-1", action: "cancel", want: "cancelled"},
		{name: "participant ends accepted", status: "accepted", actor: "user-2", action: "hangup", want: "ended"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repo := &fakeCallRepo{call: callsdomain.Call{
				ID:              "call-1",
				WorkspaceID:     "workspace-1",
				ChannelID:       "channel-1",
				InitiatorUserID: "user-1",
				TargetUserID:    "user-2",
				Mode:            "audio",
				Status:          test.status,
				CreatedAt:       time.Now(),
				UpdatedAt:       time.Now(),
			}}
			service := NewService(repo, fakeCallChecker{allowed: true}, nil)
			service.SetBlockChecker(staticCallBlockChecker{blocked: true})

			updated, err := service.ChangeStatus(context.Background(), StatusInput{
				ActorUserID: test.actor,
				WorkspaceID: "workspace-1",
				CallID:      "call-1",
				Action:      test.action,
			})
			if err != nil {
				t.Fatalf("ChangeStatus(%s) error = %v", test.action, err)
			}
			if updated.Status != test.want {
				t.Fatalf("status = %q, want %q", updated.Status, test.want)
			}
		})
	}
}

func assertCallInteractionBlocked(t *testing.T, err error) {
	t.Helper()
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) || appErr.Code != "INTERACTION_BLOCKED" || appErr.Status != 403 {
		t.Fatalf("error = %#v, want 403 INTERACTION_BLOCKED", err)
	}
}

func TestTwoDeviceVideoCallControlPlaneEndToEnd(t *testing.T) {
	repo := &fakeCallRepo{}
	realtime := &fakeRealtime{}
	notifications := &fakeCallNotifications{}
	service := NewService(
		repo,
		fakeCallChecker{allowed: true},
		realtime,
		notifications,
	)

	call, err := service.Create(context.Background(), CreateInput{
		ActorUserID:  "device-a-user",
		WorkspaceID:  "workspace-1",
		ChannelID:    "direct-channel-1",
		TargetUserID: "device-b-user",
		ClientCallID: "device-a-call-1",
		Mode:         "video",
		Metadata:     json.RawMessage(`{"client":"two-device-e2e"}`),
	})
	if err != nil {
		t.Fatalf("device A create call: %v", err)
	}
	if len(notifications.incoming) != 1 {
		t.Fatalf("incoming notifications = %d, want 1", len(notifications.incoming))
	}

	if _, err := service.ChangeStatus(context.Background(), StatusInput{
		ActorUserID: "device-b-user",
		WorkspaceID: "workspace-1",
		CallID:      call.ID,
		Action:      "accept",
	}); err != nil {
		t.Fatalf("device B accept call: %v", err)
	}

	signals := []SignalInput{
		{
			ActorUserID: "device-b-user",
			WorkspaceID: "workspace-1",
			CallID:      call.ID,
			SignalType:  "ready",
			Payload:     json.RawMessage(`{}`),
		},
		{
			ActorUserID: "device-a-user",
			WorkspaceID: "workspace-1",
			CallID:      call.ID,
			SignalType:  "offer",
			Payload:     json.RawMessage(`{"sdp":{"type":"offer","sdp":"v=0"}}`),
		},
		{
			ActorUserID: "device-b-user",
			WorkspaceID: "workspace-1",
			CallID:      call.ID,
			SignalType:  "answer",
			Payload:     json.RawMessage(`{"sdp":{"type":"answer","sdp":"v=0"}}`),
		},
		{
			ActorUserID: "device-a-user",
			WorkspaceID: "workspace-1",
			CallID:      call.ID,
			SignalType:  "ice_candidate",
			Payload:     json.RawMessage(`{"candidate":{"candidate":"candidate:a","sdpMid":"0","sdpMLineIndex":0}}`),
		},
		{
			ActorUserID: "device-b-user",
			WorkspaceID: "workspace-1",
			CallID:      call.ID,
			SignalType:  "ice_candidate",
			Payload:     json.RawMessage(`{"candidate":{"candidate":"candidate:b","sdpMid":"0","sdpMLineIndex":0}}`),
		},
	}
	for index, signal := range signals {
		if _, err := service.SendSignal(context.Background(), signal); err != nil {
			t.Fatalf("signal %d (%s): %v", index, signal.SignalType, err)
		}
	}

	ended, err := service.ChangeStatus(context.Background(), StatusInput{
		ActorUserID: "device-b-user",
		WorkspaceID: "workspace-1",
		CallID:      call.ID,
		Action:      "hangup",
	})
	if err != nil {
		t.Fatalf("device B hangup: %v", err)
	}
	if ended.Status != "ended" {
		t.Fatalf("final status = %q, want ended", ended.Status)
	}
	if repo.messageCount != 1 {
		t.Fatalf("call history messages = %d, want exactly 1", repo.messageCount)
	}
	if len(notifications.terminal) != 1 {
		t.Fatalf("terminal notifications = %d, want 1", len(notifications.terminal))
	}

	eventTypes := make([]string, 0, len(realtime.events))
	for _, event := range realtime.events {
		eventTypes = append(eventTypes, event.Type)
	}
	wantEvents := []string{
		"CallInvited",
		"CallAccepted",
		"CallReady",
		"CallOffer",
		"CallAnswer",
		"CallIceCandidate",
		"CallIceCandidate",
		"CallEnded",
	}
	if len(eventTypes) != len(wantEvents) {
		t.Fatalf("event types = %#v, want %#v", eventTypes, wantEvents)
	}
	for index := range wantEvents {
		if eventTypes[index] != wantEvents[index] {
			t.Fatalf("event %d = %q, want %q", index, eventTypes[index], wantEvents[index])
		}
	}
}

func TestExpireUnansweredMarksMissedWithSystemEvent(t *testing.T) {
	repo := &fakeCallRepo{
		call: callsdomain.Call{
			ID:              "call-1",
			WorkspaceID:     "workspace-1",
			ChannelID:       "channel-1",
			InitiatorUserID: "user-1",
			TargetUserID:    "user-2",
			Mode:            "video",
			Status:          "ringing",
			CreatedAt:       time.Now().Add(-31 * time.Second),
			UpdatedAt:       time.Now().Add(-31 * time.Second),
		},
	}
	realtime := &fakeRealtime{}
	service := NewService(repo, fakeCallChecker{allowed: true}, realtime)
	service.SetRingTimeout(30 * time.Second)

	count, err := service.ExpireUnanswered(context.Background(), 10)
	if err != nil {
		t.Fatalf("ExpireUnanswered() error = %v", err)
	}
	if count != 1 || repo.call.Status != "missed" || !repo.messageCalled {
		t.Fatalf("count = %d, call = %#v, messageCalled = %v", count, repo.call, repo.messageCalled)
	}
	if len(realtime.events) != 1 || realtime.events[0].Type != "CallMissed" || realtime.events[0].ActorUserID != "system" {
		t.Fatalf("realtime events = %#v", realtime.events)
	}
}
