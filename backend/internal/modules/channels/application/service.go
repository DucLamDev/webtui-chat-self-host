package application

import (
	"context"
	"errors"
	"regexp"
	"sort"
	"strings"
	"time"

	channelsdomain "github.com/duclamdev/application-chat/backend/internal/modules/channels/domain"
	apperrors "github.com/duclamdev/application-chat/backend/internal/shared/errors"
)

var channelSlugPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,61}[a-z0-9]$`)

type PermissionChecker interface {
	HasWorkspacePermission(ctx context.Context, userID string, workspaceID string, permissionCode string) (bool, error)
}

type Repository interface {
	CreateChannel(ctx context.Context, params CreateChannelParams) (channelsdomain.Channel, error)
	FindChannel(ctx context.Context, workspaceID string, channelID string) (channelsdomain.Channel, error)
	ListChannels(ctx context.Context, workspaceID string) ([]channelsdomain.Channel, error)
	UpdateChannel(ctx context.Context, params UpdateChannelParams) (channelsdomain.Channel, error)
	ArchiveChannel(ctx context.Context, workspaceID string, channelID string) error
	CountMembers(ctx context.Context, workspaceID string, channelID string) (int, error)
	ListMembers(ctx context.Context, workspaceID string, channelID string) ([]channelsdomain.Member, error)
	FindMember(ctx context.Context, workspaceID string, channelID string, userID string) (channelsdomain.Member, error)
	AddMember(ctx context.Context, params AddMemberParams) (channelsdomain.Member, error)
	RequestJoin(ctx context.Context, params AddMemberParams) (channelsdomain.Member, error)
	UpdateMemberStatus(ctx context.Context, params UpdateMemberStatusParams) (channelsdomain.Member, error)
	UpdateReadState(ctx context.Context, params UpdateReadStateParams) (channelsdomain.Member, error)
	CreateOrGetPrivateSession(ctx context.Context, params PrivateSessionParams) (channelsdomain.Channel, error)
	CreateOrGetDirectConversation(ctx context.Context, params CreateDirectParams) (channelsdomain.DirectConversation, error)
	HasAcceptedContact(ctx context.Context, actorUserID string, participantUserID string) (bool, error)
	ListDirectConversations(ctx context.Context, workspaceID string, userID string) ([]channelsdomain.DirectConversation, error)
	RecordAudit(ctx context.Context, event AuditEvent) error
}

type Service struct {
	repo    Repository
	checker PermissionChecker
}

type CreateChannelInput struct {
	ActorUserID  string
	WorkspaceID  string
	DepartmentID string
	Slug         string
	Name         string
	Description  string
	Type         string
}

type CreateChannelParams struct {
	WorkspaceID  string
	DepartmentID string
	Slug         string
	Name         string
	Description  string
	Type         string
	CreatedBy    string
}

type UpdateChannelInput struct {
	ActorUserID  string
	WorkspaceID  string
	ChannelID    string
	DepartmentID *string
	Name         *string
	Description  *string
}

type UpdateChannelParams struct {
	WorkspaceID  string
	ChannelID    string
	DepartmentID *string
	Name         *string
	Description  *string
}

type AddMemberInput struct {
	ActorUserID string
	WorkspaceID string
	ChannelID   string
	UserID      string
}

type AddMemberParams struct {
	WorkspaceID string
	ChannelID   string
	UserID      string
}

type UpdateMemberStatusInput struct {
	ActorUserID string
	WorkspaceID string
	ChannelID   string
	UserID      string
	Status      string
}

type UpdateMemberStatusParams struct {
	WorkspaceID string
	ChannelID   string
	UserID      string
	Status      string
}

type UpdateReadStateInput struct {
	ActorUserID       string
	WorkspaceID       string
	ChannelID         string
	LastReadMessageID string
}

type UpdateReadStateParams struct {
	WorkspaceID       string
	ChannelID         string
	UserID            string
	LastReadMessageID string
}

type CreateDirectInput struct {
	ActorUserID    string
	WorkspaceID    string
	ParticipantIDs []string
}

type CreateDirectParams struct {
	WorkspaceID      string
	ParticipantKey   string
	ParticipantIDs   []string
	ConversationType string
	CreatedBy        string
}

type PrivateSessionParams struct {
	WorkspaceID     string
	SourceChannelID string
	UserID          string
}

type AuditEvent struct {
	ActorUserID string
	WorkspaceID string
	Action      string
	EntityType  string
	EntityID    string
	Metadata    map[string]any
}

type ChannelDTO struct {
	ID                 string  `json:"id"`
	WorkspaceID        string  `json:"workspace_id"`
	DepartmentID       *string `json:"department_id,omitempty"`
	Slug               *string `json:"slug,omitempty"`
	Name               string  `json:"name"`
	Description        *string `json:"description,omitempty"`
	Type               string  `json:"type"`
	Status             string  `json:"status"`
	CreatedBy          *string `json:"created_by,omitempty"`
	CreatedAt          string  `json:"created_at"`
	UpdatedAt          string  `json:"updated_at"`
	ArchivedAt         *string `json:"archived_at,omitempty"`
	MembershipStatus   string  `json:"membership_status"`
	IsMember           bool    `json:"is_member"`
	CanManage          bool    `json:"can_manage"`
	MemberCount        int     `json:"member_count"`
	PrivateSessionMode bool    `json:"private_session_mode"`
}

type MemberDTO struct {
	ChannelID         string  `json:"channel_id"`
	UserID            string  `json:"user_id"`
	Email             string  `json:"email"`
	Username          string  `json:"username"`
	DisplayName       string  `json:"display_name"`
	AvatarURL         *string `json:"avatar_url,omitempty"`
	Status            string  `json:"status"`
	LastReadAt        *string `json:"last_read_at,omitempty"`
	LastReadMessageID *string `json:"last_read_message_id,omitempty"`
	JoinedAt          string  `json:"joined_at"`
	CreatedAt         string  `json:"created_at"`
	UpdatedAt         string  `json:"updated_at"`
}

type DirectConversationDTO struct {
	ID               string             `json:"id"`
	WorkspaceID      string             `json:"workspace_id"`
	ChannelID        string             `json:"channel_id"`
	ParticipantKey   string             `json:"participant_key"`
	ConversationType string             `json:"conversation_type"`
	ParticipantIDs   []string           `json:"participant_ids"`
	Participants     []MemberDTO        `json:"participants"`
	User             *MemberDTO         `json:"user,omitempty"`
	LastMessage      *MessageSummaryDTO `json:"last_message,omitempty"`
	UnreadCount      int                `json:"unread_count"`
	CreatedAt        string             `json:"created_at"`
	UpdatedAt        string             `json:"updated_at"`
}

type MessageSummaryDTO struct {
	ID          string  `json:"id"`
	WorkspaceID string  `json:"workspace_id"`
	ChannelID   string  `json:"channel_id"`
	SenderID    *string `json:"sender_id,omitempty"`
	Kind        string  `json:"kind"`
	Body        string  `json:"body"`
	CreatedAt   string  `json:"created_at"`
	UpdatedAt   string  `json:"updated_at"`
}

func NewService(repo Repository, checker PermissionChecker) *Service {
	return &Service{repo: repo, checker: checker}
}

func (s *Service) Create(ctx context.Context, input CreateChannelInput) (ChannelDTO, error) {
	if err := s.ensurePermission(ctx, input.ActorUserID, input.WorkspaceID, "channel.create"); err != nil {
		return ChannelDTO{}, err
	}
	input.Type = strings.TrimSpace(input.Type)
	if input.Type == "" {
		input.Type = "public"
	}
	if input.Type != "public" && input.Type != "private" {
		return ChannelDTO{}, apperrors.BadRequest("VALIDATION_ERROR", "Loại kênh không hợp lệ.")
	}
	input.Slug = strings.ToLower(strings.TrimSpace(input.Slug))
	input.Name = strings.TrimSpace(input.Name)
	if !channelSlugPattern.MatchString(input.Slug) {
		return ChannelDTO{}, apperrors.BadRequest("VALIDATION_ERROR", "Slug kênh không hợp lệ.")
	}
	if input.Name == "" {
		return ChannelDTO{}, apperrors.BadRequest("VALIDATION_ERROR", "Tên kênh không được để trống.")
	}
	channel, err := s.repo.CreateChannel(ctx, CreateChannelParams{
		WorkspaceID:  strings.TrimSpace(input.WorkspaceID),
		DepartmentID: strings.TrimSpace(input.DepartmentID),
		Slug:         input.Slug,
		Name:         input.Name,
		Description:  strings.TrimSpace(input.Description),
		Type:         input.Type,
		CreatedBy:    strings.TrimSpace(input.ActorUserID),
	})
	if err != nil {
		if errors.Is(err, channelsdomain.ErrChannelConflict) {
			return ChannelDTO{}, apperrors.Conflict("CHANNEL_ALREADY_EXISTS", "Slug kênh đã tồn tại.")
		}
		return ChannelDTO{}, err
	}
	_ = s.repo.RecordAudit(ctx, AuditEvent{ActorUserID: input.ActorUserID, WorkspaceID: input.WorkspaceID, Action: "channel.create", EntityType: "channel", EntityID: channel.ID})
	dto := toChannelDTO(channel)
	dto.MembershipStatus = "active"
	dto.IsMember = true
	dto.CanManage = true
	dto.MemberCount = 1
	return dto, nil
}

func (s *Service) Get(ctx context.Context, actorUserID string, workspaceID string, channelID string) (ChannelDTO, error) {
	if err := s.ensureWorkspaceAccess(ctx, actorUserID, workspaceID); err != nil {
		return ChannelDTO{}, err
	}
	channel, err := s.repo.FindChannel(ctx, strings.TrimSpace(workspaceID), strings.TrimSpace(channelID))
	if err != nil {
		return ChannelDTO{}, mapChannelError(err)
	}
	dto, err := s.decorateChannelDTO(ctx, actorUserID, workspaceID, channel)
	if err != nil {
		return ChannelDTO{}, err
	}
	memberCount, countErr := s.repo.CountMembers(ctx, strings.TrimSpace(workspaceID), channel.ID)
	if countErr != nil {
		return ChannelDTO{}, countErr
	}
	dto.MemberCount = memberCount
	if !dto.IsMember {
		return ChannelDTO{}, apperrors.Forbidden("Bạn chưa phải là thành viên của kênh này.")
	}
	return dto, nil
}

func (s *Service) List(ctx context.Context, actorUserID string, workspaceID string) ([]ChannelDTO, error) {
	if err := s.ensureWorkspaceAccess(ctx, actorUserID, workspaceID); err != nil {
		return nil, err
	}
	channels, err := s.repo.ListChannels(ctx, strings.TrimSpace(workspaceID))
	if err != nil {
		return nil, err
	}
	dtos := make([]ChannelDTO, 0, len(channels))
	for _, channel := range channels {
		dto, err := s.decorateChannelDTO(ctx, actorUserID, workspaceID, channel)
		if err != nil {
			return nil, err
		}
		if dto.IsMember || dto.PrivateSessionMode || dto.Type == "public" {
			dtos = append(dtos, dto)
		}
	}
	return dtos, nil
}

func (s *Service) OpenPrivateSession(ctx context.Context, actorUserID string, workspaceID string, sourceChannelID string) (ChannelDTO, error) {
	if err := s.ensureWorkspaceAccess(ctx, actorUserID, workspaceID); err != nil {
		return ChannelDTO{}, err
	}
	source, err := s.repo.FindChannel(ctx, strings.TrimSpace(workspaceID), strings.TrimSpace(sourceChannelID))
	if err != nil {
		return ChannelDTO{}, mapChannelError(err)
	}
	if !source.PrivateSessionMode {
		return ChannelDTO{}, apperrors.BadRequest("CHANNEL_NOT_PRIVATE_SESSION", "Kênh này không sử dụng phiên làm việc riêng tư.")
	}
	channel, err := s.repo.CreateOrGetPrivateSession(ctx, PrivateSessionParams{
		WorkspaceID:     strings.TrimSpace(workspaceID),
		SourceChannelID: strings.TrimSpace(sourceChannelID),
		UserID:          strings.TrimSpace(actorUserID),
	})
	if err != nil {
		return ChannelDTO{}, mapChannelError(err)
	}
	dto := toChannelDTO(channel)
	dto.MembershipStatus = "active"
	dto.IsMember = true
	dto.CanManage = false
	dto.MemberCount = 1
	return dto, nil
}

func (s *Service) Update(ctx context.Context, input UpdateChannelInput) (ChannelDTO, error) {
	if err := s.ensurePermission(ctx, input.ActorUserID, input.WorkspaceID, "channel.manage"); err != nil {
		return ChannelDTO{}, err
	}
	channel, err := s.repo.UpdateChannel(ctx, UpdateChannelParams{
		WorkspaceID:  strings.TrimSpace(input.WorkspaceID),
		ChannelID:    strings.TrimSpace(input.ChannelID),
		DepartmentID: cleanOptional(input.DepartmentID),
		Name:         cleanOptional(input.Name),
		Description:  cleanOptional(input.Description),
	})
	if err != nil {
		return ChannelDTO{}, mapChannelError(err)
	}
	return toChannelDTO(channel), nil
}

func (s *Service) Archive(ctx context.Context, actorUserID string, workspaceID string, channelID string) error {
	if err := s.ensurePermission(ctx, actorUserID, workspaceID, "channel.delete"); err != nil {
		return err
	}
	if err := s.repo.ArchiveChannel(ctx, strings.TrimSpace(workspaceID), strings.TrimSpace(channelID)); err != nil {
		return mapChannelError(err)
	}
	return nil
}

func (s *Service) ListMembers(ctx context.Context, actorUserID string, workspaceID string, channelID string) ([]MemberDTO, error) {
	if err := s.ensureWorkspaceAccess(ctx, actorUserID, workspaceID); err != nil {
		return nil, err
	}
	member, err := s.repo.FindMember(ctx, strings.TrimSpace(workspaceID), strings.TrimSpace(channelID), strings.TrimSpace(actorUserID))
	if err != nil || (member.Status != "active" && member.Status != "muted") {
		if err != nil && !errors.Is(err, channelsdomain.ErrMemberNotFound) {
			return nil, err
		}
		return nil, apperrors.Forbidden("Bạn chưa phải là thành viên của kênh này.")
	}
	members, err := s.repo.ListMembers(ctx, strings.TrimSpace(workspaceID), strings.TrimSpace(channelID))
	if err != nil {
		return nil, err
	}
	return toMemberDTOs(members), nil
}

func (s *Service) AddMember(ctx context.Context, input AddMemberInput) (MemberDTO, error) {
	if err := s.ensureCanManageChannel(ctx, input.ActorUserID, input.WorkspaceID, input.ChannelID); err != nil {
		return MemberDTO{}, err
	}
	channel, err := s.repo.FindChannel(ctx, strings.TrimSpace(input.WorkspaceID), strings.TrimSpace(input.ChannelID))
	if err != nil {
		return MemberDTO{}, mapChannelError(err)
	}
	if channel.PrivateSessionMode {
		return MemberDTO{}, apperrors.BadRequest("PRIVATE_SESSION_SOURCE", "Kênh này tự tạo phiên riêng cho từng người dùng và không cho thêm thành viên trực tiếp.")
	}
	member, err := s.repo.AddMember(ctx, AddMemberParams{
		WorkspaceID: strings.TrimSpace(input.WorkspaceID),
		ChannelID:   strings.TrimSpace(input.ChannelID),
		UserID:      strings.TrimSpace(input.UserID),
	})
	if err != nil {
		return MemberDTO{}, mapMemberError(err)
	}
	return toMemberDTO(member), nil
}

func (s *Service) UpdateMemberStatus(ctx context.Context, input UpdateMemberStatusInput) (MemberDTO, error) {
	if err := s.ensureCanManageChannel(ctx, input.ActorUserID, input.WorkspaceID, input.ChannelID); err != nil {
		return MemberDTO{}, err
	}
	status := strings.TrimSpace(input.Status)
	if status != "active" && status != "muted" && status != "left" && status != "removed" {
		return MemberDTO{}, apperrors.BadRequest("VALIDATION_ERROR", "Trạng thái thành viên kênh không hợp lệ.")
	}
	member, err := s.repo.UpdateMemberStatus(ctx, UpdateMemberStatusParams{
		WorkspaceID: strings.TrimSpace(input.WorkspaceID),
		ChannelID:   strings.TrimSpace(input.ChannelID),
		UserID:      strings.TrimSpace(input.UserID),
		Status:      status,
	})
	if err != nil {
		return MemberDTO{}, mapMemberError(err)
	}
	return toMemberDTO(member), nil
}

func (s *Service) RequestJoin(ctx context.Context, actorUserID string, workspaceID string, channelID string) (MemberDTO, error) {
	if err := s.ensureWorkspaceAccess(ctx, actorUserID, workspaceID); err != nil {
		return MemberDTO{}, err
	}
	channel, err := s.repo.FindChannel(ctx, strings.TrimSpace(workspaceID), strings.TrimSpace(channelID))
	if err != nil {
		return MemberDTO{}, mapChannelError(err)
	}
	if channel.Type == "direct" {
		return MemberDTO{}, apperrors.BadRequest("VALIDATION_ERROR", "Không thể yêu cầu tham gia hội thoại riêng.")
	}
	if channel.PrivateSessionMode {
		return MemberDTO{}, apperrors.BadRequest("PRIVATE_SESSION_SOURCE", "Hãy mở kênh để hệ thống tạo phiên làm việc riêng tư.")
	}
	member, err := s.repo.FindMember(ctx, strings.TrimSpace(workspaceID), strings.TrimSpace(channelID), strings.TrimSpace(actorUserID))
	if err == nil && (member.Status == "active" || member.Status == "muted") {
		return MemberDTO{}, apperrors.Conflict("CHANNEL_ALREADY_JOINED", "Bạn đã là thành viên của kênh.")
	}
	if err == nil && member.Status == "invited" {
		return toMemberDTO(member), nil
	}
	if err != nil && !errors.Is(err, channelsdomain.ErrMemberNotFound) {
		return MemberDTO{}, err
	}
	member, err = s.repo.RequestJoin(ctx, AddMemberParams{
		WorkspaceID: strings.TrimSpace(workspaceID),
		ChannelID:   strings.TrimSpace(channelID),
		UserID:      strings.TrimSpace(actorUserID),
	})
	if err != nil {
		return MemberDTO{}, mapMemberError(err)
	}
	return toMemberDTO(member), nil
}

func (s *Service) ListJoinRequests(ctx context.Context, actorUserID string, workspaceID string, channelID string) ([]MemberDTO, error) {
	if err := s.ensureCanManageChannel(ctx, actorUserID, workspaceID, channelID); err != nil {
		return nil, err
	}
	members, err := s.repo.ListMembers(ctx, strings.TrimSpace(workspaceID), strings.TrimSpace(channelID))
	if err != nil {
		return nil, err
	}
	requests := make([]channelsdomain.Member, 0)
	for _, member := range members {
		if member.Status == "invited" {
			requests = append(requests, member)
		}
	}
	return toMemberDTOs(requests), nil
}

func (s *Service) ApproveJoinRequest(ctx context.Context, actorUserID string, workspaceID string, channelID string, userID string) (MemberDTO, error) {
	return s.UpdateMemberStatus(ctx, UpdateMemberStatusInput{ActorUserID: actorUserID, WorkspaceID: workspaceID, ChannelID: channelID, UserID: userID, Status: "active"})
}

func (s *Service) RejectJoinRequest(ctx context.Context, actorUserID string, workspaceID string, channelID string, userID string) error {
	_, err := s.UpdateMemberStatus(ctx, UpdateMemberStatusInput{ActorUserID: actorUserID, WorkspaceID: workspaceID, ChannelID: channelID, UserID: userID, Status: "removed"})
	return err
}

func (s *Service) UpdateReadState(ctx context.Context, input UpdateReadStateInput) (MemberDTO, error) {
	if err := s.ensureWorkspaceAccess(ctx, input.ActorUserID, input.WorkspaceID); err != nil {
		return MemberDTO{}, err
	}
	member, err := s.repo.UpdateReadState(ctx, UpdateReadStateParams{
		WorkspaceID:       strings.TrimSpace(input.WorkspaceID),
		ChannelID:         strings.TrimSpace(input.ChannelID),
		UserID:            strings.TrimSpace(input.ActorUserID),
		LastReadMessageID: strings.TrimSpace(input.LastReadMessageID),
	})
	if err != nil {
		return MemberDTO{}, mapMemberError(err)
	}
	return toMemberDTO(member), nil
}

func (s *Service) CreateDirect(ctx context.Context, input CreateDirectInput) (DirectConversationDTO, error) {
	if err := s.ensurePermission(ctx, input.ActorUserID, input.WorkspaceID, "message.send"); err != nil {
		return DirectConversationDTO{}, err
	}
	participantIDs := normalizeParticipants(input.ActorUserID, input.ParticipantIDs)
	if len(participantIDs) < 2 {
		return DirectConversationDTO{}, apperrors.BadRequest("VALIDATION_ERROR", "Direct message cần ít nhất 2 thành viên.")
	}
	if err := s.ensureDirectContacts(ctx, input.ActorUserID, participantIDs); err != nil {
		return DirectConversationDTO{}, err
	}
	conversationType := "group"
	if len(participantIDs) == 2 {
		conversationType = "one_to_one"
	}
	conversation, err := s.repo.CreateOrGetDirectConversation(ctx, CreateDirectParams{
		WorkspaceID:      strings.TrimSpace(input.WorkspaceID),
		ParticipantKey:   strings.Join(participantIDs, ":"),
		ParticipantIDs:   participantIDs,
		ConversationType: conversationType,
		CreatedBy:        strings.TrimSpace(input.ActorUserID),
	})
	if err != nil {
		if errors.Is(err, channelsdomain.ErrMemberNotFound) {
			return DirectConversationDTO{}, apperrors.BadRequest("INVALID_PARTICIPANTS", "Một hoặc nhiều thành viên chưa thuộc workspace.")
		}
		return DirectConversationDTO{}, err
	}
	return toDirectDTO(conversation, input.ActorUserID), nil
}

func (s *Service) ensureDirectContacts(ctx context.Context, actorUserID string, participantIDs []string) error {
	actorUserID = strings.TrimSpace(actorUserID)
	for _, participantID := range participantIDs {
		participantID = strings.TrimSpace(participantID)
		if participantID == "" || participantID == actorUserID {
			continue
		}
		accepted, err := s.repo.HasAcceptedContact(ctx, actorUserID, participantID)
		if err != nil {
			return err
		}
		if !accepted {
			return apperrors.Forbidden("Hai tài khoản cần là bạn bè trước khi mở hội thoại riêng.")
		}
	}
	return nil
}

func (s *Service) ListDirects(ctx context.Context, actorUserID string, workspaceID string) ([]DirectConversationDTO, error) {
	if err := s.ensureWorkspaceAccess(ctx, actorUserID, workspaceID); err != nil {
		return nil, err
	}
	conversations, err := s.repo.ListDirectConversations(ctx, strings.TrimSpace(workspaceID), strings.TrimSpace(actorUserID))
	if err != nil {
		return nil, err
	}
	dtos := make([]DirectConversationDTO, 0, len(conversations))
	for _, conversation := range conversations {
		dtos = append(dtos, toDirectDTO(conversation, actorUserID))
	}
	return dtos, nil
}

func (s *Service) ensureWorkspaceAccess(ctx context.Context, userID string, workspaceID string) error {
	allowed, err := s.checker.HasWorkspacePermission(ctx, strings.TrimSpace(userID), strings.TrimSpace(workspaceID), "message.send")
	if err != nil {
		return err
	}
	if !allowed {
		return apperrors.Forbidden("Bạn không thuộc workspace này.")
	}
	return nil
}

func (s *Service) CanJoinRoom(ctx context.Context, userID string, room string) bool {
	parts := strings.Split(strings.TrimSpace(room), ":")
	if len(parts) != 4 || parts[0] != "workspace" || parts[2] != "channel" {
		return false
	}
	member, err := s.repo.FindMember(ctx, parts[1], parts[3], strings.TrimSpace(userID))
	return err == nil && (member.Status == "active" || member.Status == "muted")
}

func (s *Service) ensureCanManageChannel(ctx context.Context, userID string, workspaceID string, channelID string) error {
	channel, err := s.repo.FindChannel(ctx, strings.TrimSpace(workspaceID), strings.TrimSpace(channelID))
	if err != nil {
		return mapChannelError(err)
	}
	if channel.Type == "direct" {
		return apperrors.Forbidden("Không thể thêm hoặc thay đổi thành viên của hội thoại riêng.")
	}
	if channel.CreatedBy != nil && *channel.CreatedBy == strings.TrimSpace(userID) {
		return nil
	}
	return s.ensurePermission(ctx, userID, workspaceID, "channel.manage")
}

func (s *Service) decorateChannelDTO(ctx context.Context, actorUserID string, workspaceID string, channel channelsdomain.Channel) (ChannelDTO, error) {
	dto := toChannelDTO(channel)
	dto.MembershipStatus = "none"
	member, err := s.repo.FindMember(ctx, strings.TrimSpace(workspaceID), channel.ID, strings.TrimSpace(actorUserID))
	if err == nil {
		dto.MembershipStatus = member.Status
		dto.IsMember = member.Status == "active" || member.Status == "muted"
	} else if !errors.Is(err, channelsdomain.ErrMemberNotFound) {
		return ChannelDTO{}, err
	}
	dto.CanManage = channel.Type != "direct" && channel.CreatedBy != nil && *channel.CreatedBy == strings.TrimSpace(actorUserID)
	if !dto.CanManage && channel.Type != "direct" {
		allowed, permissionErr := s.checker.HasWorkspacePermission(ctx, strings.TrimSpace(actorUserID), strings.TrimSpace(workspaceID), "channel.manage")
		if permissionErr != nil {
			return ChannelDTO{}, permissionErr
		}
		dto.CanManage = allowed
	}
	return dto, nil
}

func (s *Service) ensurePermission(ctx context.Context, userID string, workspaceID string, permission string) error {
	allowed, err := s.checker.HasWorkspacePermission(ctx, strings.TrimSpace(userID), strings.TrimSpace(workspaceID), permission)
	if err != nil {
		return err
	}
	if !allowed {
		return apperrors.Forbidden("Bạn không có quyền thực hiện thao tác này.")
	}
	return nil
}

func normalizeParticipants(actorUserID string, ids []string) []string {
	seen := map[string]bool{}
	result := make([]string, 0, len(ids)+1)
	for _, id := range append(ids, actorUserID) {
		id = strings.TrimSpace(id)
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		result = append(result, id)
	}
	sort.Strings(result)
	return result
}

func cleanOptional(value *string) *string {
	if value == nil {
		return nil
	}
	cleaned := strings.TrimSpace(*value)
	return &cleaned
}

func mapChannelError(err error) error {
	if errors.Is(err, channelsdomain.ErrChannelNotFound) {
		return apperrors.NotFound("CHANNEL_NOT_FOUND", "Không tìm thấy kênh.")
	}
	return err
}

func mapMemberError(err error) error {
	if errors.Is(err, channelsdomain.ErrMemberNotFound) {
		return apperrors.NotFound("CHANNEL_MEMBER_NOT_FOUND", "Không tìm thấy thành viên kênh.")
	}
	return err
}

func toChannelDTO(channel channelsdomain.Channel) ChannelDTO {
	return ChannelDTO{
		ID:                 channel.ID,
		WorkspaceID:        channel.WorkspaceID,
		DepartmentID:       channel.DepartmentID,
		Slug:               channel.Slug,
		Name:               channel.Name,
		Description:        channel.Description,
		Type:               channel.Type,
		Status:             channel.Status,
		CreatedBy:          channel.CreatedBy,
		CreatedAt:          formatTime(channel.CreatedAt),
		UpdatedAt:          formatTime(channel.UpdatedAt),
		ArchivedAt:         formatOptionalTime(channel.ArchivedAt),
		MemberCount:        channel.MemberCount,
		PrivateSessionMode: channel.PrivateSessionMode,
	}
}

func toMemberDTOs(members []channelsdomain.Member) []MemberDTO {
	dtos := make([]MemberDTO, 0, len(members))
	for _, member := range members {
		dtos = append(dtos, toMemberDTO(member))
	}
	return dtos
}

func toMemberDTO(member channelsdomain.Member) MemberDTO {
	return MemberDTO{
		ChannelID:         member.ChannelID,
		UserID:            member.UserID,
		Email:             member.Email,
		Username:          member.Username,
		DisplayName:       member.DisplayName,
		AvatarURL:         member.AvatarURL,
		Status:            member.Status,
		LastReadAt:        formatOptionalTime(member.LastReadAt),
		LastReadMessageID: member.LastReadMessageID,
		JoinedAt:          formatTime(member.JoinedAt),
		CreatedAt:         formatTime(member.CreatedAt),
		UpdatedAt:         formatTime(member.UpdatedAt),
	}
}

func toDirectDTO(conversation channelsdomain.DirectConversation, actorUserID string) DirectConversationDTO {
	participants := toMemberDTOs(conversation.Participants)
	var peer *MemberDTO
	for index := range participants {
		if participants[index].UserID != strings.TrimSpace(actorUserID) {
			current := participants[index]
			peer = &current
			break
		}
	}
	return DirectConversationDTO{
		ID:               conversation.ID,
		WorkspaceID:      conversation.WorkspaceID,
		ChannelID:        conversation.ChannelID,
		ParticipantKey:   conversation.ParticipantKey,
		ConversationType: conversation.ConversationType,
		ParticipantIDs:   conversation.ParticipantIDs,
		Participants:     participants,
		User:             peer,
		LastMessage:      toMessageSummaryDTO(conversation.LastMessage),
		UnreadCount:      conversation.UnreadCount,
		CreatedAt:        formatTime(conversation.CreatedAt),
		UpdatedAt:        formatTime(conversation.UpdatedAt),
	}
}

func toMessageSummaryDTO(message *channelsdomain.MessageSummary) *MessageSummaryDTO {
	if message == nil {
		return nil
	}
	return &MessageSummaryDTO{
		ID:          message.ID,
		WorkspaceID: message.WorkspaceID,
		ChannelID:   message.ChannelID,
		SenderID:    message.SenderID,
		Kind:        message.Kind,
		Body:        message.Body,
		CreatedAt:   formatTime(message.CreatedAt),
		UpdatedAt:   formatTime(message.UpdatedAt),
	}
}

func formatOptionalTime(value *time.Time) *string {
	if value == nil {
		return nil
	}
	formatted := formatTime(*value)
	return &formatted
}

func formatTime(value time.Time) string {
	return value.UTC().Format(time.RFC3339)
}
