package application

import (
	"context"
	"testing"
	"time"

	contactsdomain "github.com/duclamdev/application-chat/backend/internal/modules/contacts/domain"
)

func TestAcceptRequestPublishesRecipientScopedContactDTOs(t *testing.T) {
	now := time.Date(2026, 9, 3, 8, 0, 0, 0, time.UTC)
	repo := &contactRepositoryStub{
		request: contactsdomain.ContactRequest{
			ID:          "request-1",
			RequesterID: "requester",
			ReceiverID:  "receiver",
			Status:      "accepted",
			RequestedAt: now,
			CreatedAt:   now,
			UpdatedAt:   now,
		},
		users: map[string]contactsdomain.UserSummary{
			"requester": {ID: "requester", Email: "requester@example.test", Username: "requester", DisplayName: "Requester", Status: "active"},
			"receiver":  {ID: "receiver", Email: "receiver@example.test", Username: "receiver", DisplayName: "Receiver", Status: "active"},
		},
	}
	publisher := &contactRealtimeRecorder{}
	service := NewService(repo, publisher)

	dto, err := service.AcceptRequest(context.Background(), "zone-1", "receiver", "request-1")
	if err != nil {
		t.Fatalf("AcceptRequest() error = %v", err)
	}
	if dto.Direction != "incoming" || dto.User.ID != "requester" {
		t.Fatalf("returned dto = %#v, want receiver-scoped requester", dto)
	}
	if len(publisher.events) != 2 {
		t.Fatalf("published events = %d, want 2", len(publisher.events))
	}

	byUser := map[string]ContactRequestDTO{}
	for _, event := range publisher.events {
		payload, ok := event.Payload["contact_request"].(ContactRequestDTO)
		if !ok {
			t.Fatalf("payload contact_request = %T, want ContactRequestDTO", event.Payload["contact_request"])
		}
		byUser[event.UserID] = payload
	}

	if got := byUser["requester"]; got.Direction != "outgoing" || got.User.ID != "receiver" {
		t.Fatalf("requester payload = %#v, want outgoing receiver", got)
	}
	if got := byUser["receiver"]; got.Direction != "incoming" || got.User.ID != "requester" {
		t.Fatalf("receiver payload = %#v, want incoming requester", got)
	}
}

type contactRepositoryStub struct {
	request contactsdomain.ContactRequest
	users   map[string]contactsdomain.UserSummary
}

func (r *contactRepositoryStub) AcceptRequest(_ context.Context, _ string, actorUserID string, _ string) (contactsdomain.ContactRequest, error) {
	return r.requestFor(actorUserID), nil
}

func (r *contactRepositoryStub) CancelRequest(_ context.Context, _ string, actorUserID string, _ string) (contactsdomain.ContactRequest, error) {
	return r.requestFor(actorUserID), nil
}

func (r *contactRepositoryStub) CreateRequest(_ context.Context, _ string, actorUserID string, receiverID string) (contactsdomain.ContactRequest, error) {
	request := r.request
	request.RequesterID = actorUserID
	request.ReceiverID = receiverID
	return r.withUserFor(request, actorUserID), nil
}

func (r *contactRepositoryStub) GetRequest(_ context.Context, _ string, actorUserID string, _ string) (contactsdomain.ContactRequest, error) {
	return r.requestFor(actorUserID), nil
}

func (r *contactRepositoryStub) ListContacts(context.Context, string, string) ([]contactsdomain.ContactRequest, error) {
	return nil, nil
}

func (r *contactRepositoryStub) ListRequests(context.Context, string, string, string) ([]contactsdomain.ContactRequest, error) {
	return nil, nil
}

func (r *contactRepositoryStub) RejectRequest(_ context.Context, _ string, actorUserID string, _ string) (contactsdomain.ContactRequest, error) {
	return r.requestFor(actorUserID), nil
}

func (r *contactRepositoryStub) requestFor(actorUserID string) contactsdomain.ContactRequest {
	return r.withUserFor(r.request, actorUserID)
}

func (r *contactRepositoryStub) withUserFor(request contactsdomain.ContactRequest, actorUserID string) contactsdomain.ContactRequest {
	otherUserID := request.RequesterID
	if actorUserID == request.RequesterID {
		otherUserID = request.ReceiverID
	}
	request.User = r.users[otherUserID]
	return request
}

type contactRealtimeRecorder struct {
	events []RealtimeEvent
}

func (r *contactRealtimeRecorder) Publish(_ context.Context, event RealtimeEvent) error {
	r.events = append(r.events, event)
	return nil
}
