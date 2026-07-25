package application

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	callsdomain "github.com/duclamdev/application-chat/backend/internal/modules/calls/domain"
)

type fakeCallRepo struct {
	call               callsdomain.Call
	createCalled       bool
	updateStatusCalled bool
	messageCalled      bool
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
