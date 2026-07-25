package application

import (
	"context"
	"reflect"
	"testing"
	"time"

	channelsdomain "github.com/duclamdev/application-chat/backend/internal/modules/channels/domain"
	apperrors "github.com/duclamdev/application-chat/backend/internal/shared/errors"
)

type staticPermissionChecker struct {
	allowed bool
}

func (c staticPermissionChecker) HasWorkspacePermission(context.Context, string, string, string) (bool, error) {
	return c.allowed, nil
}

type directConversationRepo struct {
	params         CreateDirectParams
	privateSource  channelsdomain.Channel
	privateSession channelsdomain.Channel
}

func (r *directConversationRepo) CreateChannel(context.Context, CreateChannelParams) (channelsdomain.Channel, error) {
	panic("không được gọi")
}

func (r *directConversationRepo) FindChannel(_ context.Context, _ string, channelID string) (channelsdomain.Channel, error) {
	if r.privateSource.ID == channelID {
		return r.privateSource, nil
	}
	panic("không được gọi")
}

func (r *directConversationRepo) ListChannels(context.Context, string) ([]channelsdomain.Channel, error) {
	panic("không được gọi")
}

func (r *directConversationRepo) UpdateChannel(context.Context, UpdateChannelParams) (channelsdomain.Channel, error) {
	panic("không được gọi")
}

func (r *directConversationRepo) ArchiveChannel(context.Context, string, string) error {
	panic("không được gọi")
}

func (r *directConversationRepo) CountMembers(context.Context, string, string) (int, error) {
	return 1, nil
}

func (r *directConversationRepo) ListMembers(context.Context, string, string) ([]channelsdomain.Member, error) {
	panic("không được gọi")
}

func (r *directConversationRepo) FindMember(context.Context, string, string, string) (channelsdomain.Member, error) {
	panic("không được gọi")
}

func (r *directConversationRepo) AddMember(context.Context, AddMemberParams) (channelsdomain.Member, error) {
	panic("không được gọi")
}

func (r *directConversationRepo) RequestJoin(context.Context, AddMemberParams) (channelsdomain.Member, error) {
	panic("không được gọi")
}

func (r *directConversationRepo) UpdateMemberStatus(context.Context, UpdateMemberStatusParams) (channelsdomain.Member, error) {
	panic("không được gọi")
}

func (r *directConversationRepo) UpdateReadState(context.Context, UpdateReadStateParams) (channelsdomain.Member, error) {
	panic("không được gọi")
}

func (r *directConversationRepo) CreateOrGetPrivateSession(context.Context, PrivateSessionParams) (channelsdomain.Channel, error) {
	return r.privateSession, nil
}

func (r *directConversationRepo) CreateOrGetDirectConversation(_ context.Context, params CreateDirectParams) (channelsdomain.DirectConversation, error) {
	r.params = params
	now := time.Date(2026, 7, 6, 10, 0, 0, 0, time.UTC)
	return channelsdomain.DirectConversation{
		ID:               "direct-1",
		WorkspaceID:      params.WorkspaceID,
		ChannelID:        "channel-1",
		ParticipantKey:   params.ParticipantKey,
		ConversationType: params.ConversationType,
		ParticipantIDs:   params.ParticipantIDs,
		CreatedAt:        now,
		UpdatedAt:        now,
	}, nil
}

func (r *directConversationRepo) HasAcceptedContact(context.Context, string, string) (bool, error) {
	return true, nil
}

func (r *directConversationRepo) ListDirectConversations(context.Context, string, string) ([]channelsdomain.DirectConversation, error) {
	panic("không được gọi")
}

func (r *directConversationRepo) RecordAudit(context.Context, AuditEvent) error {
	return nil
}

func TestCreateDirectNormalizesParticipants(t *testing.T) {
	repo := &directConversationRepo{}
	service := NewService(repo, staticPermissionChecker{allowed: true})

	dto, err := service.CreateDirect(context.Background(), CreateDirectInput{
		ActorUserID:    "user-b",
		WorkspaceID:    "workspace-1",
		ParticipantIDs: []string{" user-a ", "user-b", "user-a", ""},
	})
	if err != nil {
		t.Fatalf("CreateDirect() trả lỗi: %v", err)
	}

	wantParticipants := []string{"user-a", "user-b"}
	if !reflect.DeepEqual(repo.params.ParticipantIDs, wantParticipants) {
		t.Fatalf("ParticipantIDs = %#v, muốn %#v", repo.params.ParticipantIDs, wantParticipants)
	}
	if repo.params.ParticipantKey != "user-a:user-b" {
		t.Fatalf("ParticipantKey = %q", repo.params.ParticipantKey)
	}
	if repo.params.ConversationType != "one_to_one" {
		t.Fatalf("ConversationType = %q", repo.params.ConversationType)
	}
	if dto.ParticipantKey != repo.params.ParticipantKey {
		t.Fatalf("DTO ParticipantKey = %q", dto.ParticipantKey)
	}
}

func TestOpenPrivateSessionReturnsUserOnlyChannel(t *testing.T) {
	now := time.Date(2026, 7, 12, 12, 0, 0, 0, time.UTC)
	repo := &directConversationRepo{
		privateSource:  channelsdomain.Channel{ID: "source-1", WorkspaceID: "workspace-1", Name: "Kế toán", PrivateSessionMode: true},
		privateSession: channelsdomain.Channel{ID: "session-1", WorkspaceID: "workspace-1", Name: "Kế toán", Type: "direct", CreatedAt: now, UpdatedAt: now},
	}
	service := NewService(repo, staticPermissionChecker{allowed: true})

	dto, err := service.OpenPrivateSession(context.Background(), "user-1", "workspace-1", "source-1")
	if err != nil {
		t.Fatalf("OpenPrivateSession() error = %v", err)
	}
	if dto.ID != "session-1" || dto.Type != "direct" || !dto.IsMember {
		t.Fatalf("OpenPrivateSession() = %#v", dto)
	}
}

type channelListRepo struct {
	directConversationRepo
	channel channelsdomain.Channel
}

func (r *channelListRepo) ListChannels(context.Context, string) ([]channelsdomain.Channel, error) {
	return []channelsdomain.Channel{r.channel}, nil
}

func (r *channelListRepo) FindMember(context.Context, string, string, string) (channelsdomain.Member, error) {
	return channelsdomain.Member{Status: "active"}, nil
}

func TestListIncludesActiveMemberCount(t *testing.T) {
	now := time.Date(2026, 7, 13, 8, 0, 0, 0, time.UTC)
	repo := &channelListRepo{
		channel: channelsdomain.Channel{
			ID: "channel-1", WorkspaceID: "workspace-1", Name: "Kỹ thuật", Type: "private", MemberCount: 3, CreatedAt: now, UpdatedAt: now,
		},
	}
	service := NewService(repo, staticPermissionChecker{allowed: true})

	channels, err := service.List(context.Background(), "user-1", "workspace-1")
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(channels) != 1 || channels[0].MemberCount != 3 {
		t.Fatalf("List() member_count = %#v, want 3", channels)
	}
}

type publicChannelListRepo struct {
	channelListRepo
}

func (r *publicChannelListRepo) FindMember(context.Context, string, string, string) (channelsdomain.Member, error) {
	return channelsdomain.Member{}, channelsdomain.ErrMemberNotFound
}

func TestListIncludesPublicChannelForNonMember(t *testing.T) {
	now := time.Date(2026, 7, 15, 8, 0, 0, 0, time.UTC)
	repo := &publicChannelListRepo{
		channelListRepo: channelListRepo{
			channel: channelsdomain.Channel{
				ID: "channel-public", WorkspaceID: "workspace-1", Name: "Thông báo", Type: "public", CreatedAt: now, UpdatedAt: now,
			},
		},
	}
	service := NewService(repo, staticPermissionChecker{allowed: true})

	channels, err := service.List(context.Background(), "user-1", "workspace-1")
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(channels) != 1 || channels[0].IsMember {
		t.Fatalf("List() public non-member = %#v, want visible and not joined", channels)
	}
}

func TestCreateDirectRejectsSingleParticipant(t *testing.T) {
	service := NewService(&directConversationRepo{}, staticPermissionChecker{allowed: true})

	_, err := service.CreateDirect(context.Background(), CreateDirectInput{
		ActorUserID:    "user-a",
		WorkspaceID:    "workspace-1",
		ParticipantIDs: []string{" user-a ", ""},
	})
	if err == nil {
		t.Fatal("CreateDirect() phải trả lỗi khi chỉ có một thành viên")
	}

	appErr, ok := err.(*apperrors.AppError)
	if !ok {
		t.Fatalf("lỗi = %T, muốn AppError", err)
	}
	if appErr.Code != "VALIDATION_ERROR" {
		t.Fatalf("mã lỗi = %q", appErr.Code)
	}
}
