package application

import (
	"context"
	"encoding/json"
	"sort"
	"strconv"
	"strings"
	"time"

	channelsdomain "github.com/duclamdev/application-chat/backend/internal/modules/channels/domain"
	apperrors "github.com/duclamdev/application-chat/backend/internal/shared/errors"
)

const (
	maxBreakoutRooms      = 20
	maxMeetingTitle       = 160
	maxMeetingDescription = 2000
)

type TalkProductivityRepository interface {
	ListMeetings(ctx context.Context, workspaceID string, channelID string, from *time.Time, to *time.Time) ([]Meeting, error)
	CreateMeeting(ctx context.Context, params CreateMeetingParams) (Meeting, error)
	TransitionMeeting(ctx context.Context, params TransitionMeetingParams) (Meeting, error)
	GetVoiceRoom(ctx context.Context, workspaceID string, channelID string) (VoiceRoom, error)
	SetVoiceRoom(ctx context.Context, params SetVoiceRoomParams) (VoiceRoom, error)
	ReplaceBreakoutRooms(ctx context.Context, params ReplaceBreakoutRoomsParams) ([]channelsdomain.BreakoutRoom, error)
	SetBreakoutRoomsStatus(ctx context.Context, workspaceID string, channelID string, status string) ([]channelsdomain.BreakoutRoom, error)
	JoinBreakoutRoom(ctx context.Context, params JoinBreakoutRoomParams) ([]channelsdomain.BreakoutRoom, error)
	UpdateBreakoutAssignments(ctx context.Context, params UpdateBreakoutAssignmentsParams) ([]channelsdomain.BreakoutRoom, error)
	CreateBreakoutBroadcast(ctx context.Context, params CreateBreakoutBroadcastParams) (BreakoutBroadcast, error)
	GetTalkHome(ctx context.Context, workspaceID string, userID string, now time.Time) (TalkHome, error)
	ListSharedItems(ctx context.Context, workspaceID string, channelID string, kind string, limit int) ([]SharedItem, error)
}

type Meeting struct {
	ID           string
	WorkspaceID  string
	ChannelID    string
	Title        string
	Description  string
	StartsAt     time.Time
	EndsAt       *time.Time
	LobbyOpensAt *time.Time
	Status       string
	RoomPolicy   string
	CleanupAfter *time.Time
	CreatedBy    *string
	StartedAt    *time.Time
	EndedAt      *time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type MeetingDTO struct {
	ID           string  `json:"id"`
	WorkspaceID  string  `json:"workspace_id"`
	ChannelID    string  `json:"channel_id"`
	Title        string  `json:"title"`
	Description  string  `json:"description"`
	StartsAt     string  `json:"starts_at"`
	EndsAt       *string `json:"ends_at,omitempty"`
	LobbyOpensAt *string `json:"lobby_opens_at,omitempty"`
	Status       string  `json:"status"`
	RoomPolicy   string  `json:"room_policy"`
	CleanupAfter *string `json:"cleanup_after,omitempty"`
	CreatedBy    *string `json:"created_by,omitempty"`
	StartedAt    *string `json:"started_at,omitempty"`
	EndedAt      *string `json:"ended_at,omitempty"`
	CreatedAt    string  `json:"created_at"`
	UpdatedAt    string  `json:"updated_at"`
}

type CreateMeetingInput struct {
	ActorUserID  string
	WorkspaceID  string
	ChannelID    string
	Title        string
	Description  string
	StartsAt     string
	EndsAt       string
	LobbyOpensAt string
	RoomPolicy   string
	CleanupAfter string
}

type CreateMeetingParams struct {
	WorkspaceID  string
	ChannelID    string
	Title        string
	Description  string
	StartsAt     time.Time
	EndsAt       *time.Time
	LobbyOpensAt *time.Time
	RoomPolicy   string
	CleanupAfter *time.Time
	ActorUserID  string
}

type TransitionMeetingParams struct {
	WorkspaceID string
	ChannelID   string
	MeetingID   string
	Action      string
	ActorUserID string
}

type VoiceRoom struct {
	ChannelID   string
	WorkspaceID string
	Status      string
	StartedBy   *string
	StartedAt   *time.Time
	EndedAt     *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type VoiceRoomDTO struct {
	ChannelID   string  `json:"channel_id"`
	WorkspaceID string  `json:"workspace_id"`
	Status      string  `json:"status"`
	StartedBy   *string `json:"started_by,omitempty"`
	StartedAt   *string `json:"started_at,omitempty"`
	EndedAt     *string `json:"ended_at,omitempty"`
	CreatedAt   string  `json:"created_at"`
	UpdatedAt   string  `json:"updated_at"`
}

type SetVoiceRoomParams struct {
	WorkspaceID string
	ChannelID   string
	Status      string
	ActorUserID string
}

type BreakoutRoomSpec struct {
	Name            string
	AssignedUserIDs []string
	Sequence        int
}

type SetupBreakoutsInput struct {
	ActorUserID     string
	WorkspaceID     string
	ChannelID       string
	AssignmentMode  string
	RoomCount       int
	AllowSelfSelect bool
	Rooms           []BreakoutRoomSpec
}

type ReplaceBreakoutRoomsParams struct {
	WorkspaceID     string
	ChannelID       string
	AssignmentMode  string
	AllowSelfSelect bool
	Rooms           []BreakoutRoomSpec
	ActorUserID     string
}

type JoinBreakoutRoomParams struct {
	WorkspaceID string
	ChannelID   string
	RoomID      string
	UserID      string
}

type UpdateBreakoutAssignmentsParams struct {
	WorkspaceID     string
	ChannelID       string
	RoomID          string
	AssignedUserIDs []string
}

type CreateBreakoutBroadcastParams struct {
	WorkspaceID string
	ChannelID   string
	Body        string
	ActorUserID string
}

type BreakoutBroadcast struct {
	ID        string
	ChannelID string
	Body      string
	CreatedBy *string
	CreatedAt time.Time
}

type BreakoutBroadcastDTO struct {
	ID        string  `json:"id"`
	ChannelID string  `json:"channel_id"`
	Body      string  `json:"body"`
	CreatedBy *string `json:"created_by,omitempty"`
	CreatedAt string  `json:"created_at"`
}

type TalkHome struct {
	UpcomingMeetings []Meeting
	ActiveVoiceRooms []VoiceRoom
	OpenTasks        []channelsdomain.ChannelTask
	UnreadMentions   int
	PendingReminders int
	MissedCalls      int
}

type TalkHomeDTO struct {
	UpcomingMeetings []MeetingDTO     `json:"upcoming_meetings"`
	ActiveVoiceRooms []VoiceRoomDTO   `json:"active_voice_rooms"`
	OpenTasks        []ChannelTaskDTO `json:"open_tasks"`
	UnreadMentions   int              `json:"unread_mentions"`
	PendingReminders int              `json:"pending_reminders"`
	MissedCalls      int              `json:"missed_calls"`
}

type SharedItem struct {
	ID        string
	Kind      string
	Title     string
	Subtitle  string
	URL       string
	Metadata  json.RawMessage
	CreatedBy *string
	CreatedAt time.Time
}

type SharedItemDTO struct {
	ID        string          `json:"id"`
	Kind      string          `json:"kind"`
	Title     string          `json:"title"`
	Subtitle  string          `json:"subtitle,omitempty"`
	URL       string          `json:"url,omitempty"`
	Metadata  json.RawMessage `json:"metadata"`
	CreatedBy *string         `json:"created_by,omitempty"`
	CreatedAt string          `json:"created_at"`
}

func (s *Service) ListMeetings(ctx context.Context, actorUserID string, workspaceID string, channelID string, fromValue string, toValue string) ([]MeetingDTO, error) {
	if err := s.ensureCollaborationMember(ctx, actorUserID, workspaceID, channelID); err != nil {
		return nil, err
	}
	from, err := parseDateFilter(fromValue)
	if err != nil {
		return nil, apperrors.BadRequest("VALIDATION_ERROR", "from must be an RFC3339 timestamp.")
	}
	to, err := parseDateFilter(toValue)
	if err != nil {
		return nil, apperrors.BadRequest("VALIDATION_ERROR", "to must be an RFC3339 timestamp.")
	}
	meetings, err := s.talkProductivityRepository().ListMeetings(ctx, strings.TrimSpace(workspaceID), strings.TrimSpace(channelID), from, to)
	if err != nil {
		return nil, err
	}
	return toMeetingDTOs(meetings), nil
}

func (s *Service) CreateMeeting(ctx context.Context, input CreateMeetingInput) (MeetingDTO, error) {
	if err := s.ensureCanManageCollaboration(ctx, input.ActorUserID, input.WorkspaceID, input.ChannelID); err != nil {
		return MeetingDTO{}, err
	}
	title := strings.TrimSpace(input.Title)
	if title == "" || len([]rune(title)) > maxMeetingTitle {
		return MeetingDTO{}, apperrors.BadRequest("VALIDATION_ERROR", "Meeting title must contain 1 to 160 characters.")
	}
	description := strings.TrimSpace(input.Description)
	if len([]rune(description)) > maxMeetingDescription {
		return MeetingDTO{}, apperrors.BadRequest("VALIDATION_ERROR", "Meeting description must not exceed 2000 characters.")
	}
	startsAt, err := time.Parse(time.RFC3339, strings.TrimSpace(input.StartsAt))
	if err != nil {
		return MeetingDTO{}, apperrors.BadRequest("VALIDATION_ERROR", "starts_at must be an RFC3339 timestamp.")
	}
	if startsAt.Before(time.Now().UTC().Add(-time.Minute)) {
		return MeetingDTO{}, apperrors.BadRequest("VALIDATION_ERROR", "starts_at must not be in the past.")
	}
	endsAt, err := parseDateFilter(input.EndsAt)
	if err != nil || (endsAt != nil && !endsAt.After(startsAt)) {
		return MeetingDTO{}, apperrors.BadRequest("VALIDATION_ERROR", "ends_at must be after starts_at.")
	}
	lobbyOpensAt, err := parseDateFilter(input.LobbyOpensAt)
	if err != nil || (lobbyOpensAt != nil && !lobbyOpensAt.Before(startsAt)) {
		return MeetingDTO{}, apperrors.BadRequest("VALIDATION_ERROR", "lobby_opens_at must be before starts_at.")
	}
	roomPolicy := strings.TrimSpace(input.RoomPolicy)
	if roomPolicy == "" {
		roomPolicy = "keep"
	}
	if roomPolicy != "keep" && roomPolicy != "archive" && roomPolicy != "delete" {
		return MeetingDTO{}, apperrors.BadRequest("VALIDATION_ERROR", "room_policy must be keep, archive or delete.")
	}
	cleanupAfter, err := parseDateFilter(input.CleanupAfter)
	if err != nil {
		return MeetingDTO{}, apperrors.BadRequest("VALIDATION_ERROR", "cleanup_after must be an RFC3339 timestamp.")
	}
	cleanupReference := startsAt
	if endsAt != nil {
		cleanupReference = *endsAt
	}
	if cleanupAfter != nil && !cleanupAfter.After(cleanupReference) {
		return MeetingDTO{}, apperrors.BadRequest("VALIDATION_ERROR", "cleanup_after must be after the meeting ends.")
	}
	if roomPolicy == "keep" {
		cleanupAfter = nil
	} else if cleanupAfter == nil {
		defaultCleanup := cleanupReference.Add(24 * time.Hour)
		cleanupAfter = &defaultCleanup
	}
	meeting, err := s.talkProductivityRepository().CreateMeeting(ctx, CreateMeetingParams{
		WorkspaceID: strings.TrimSpace(input.WorkspaceID), ChannelID: strings.TrimSpace(input.ChannelID),
		Title: title, Description: description, StartsAt: startsAt.UTC(),
		EndsAt: endsAt, LobbyOpensAt: lobbyOpensAt, RoomPolicy: roomPolicy,
		CleanupAfter: cleanupAfter, ActorUserID: strings.TrimSpace(input.ActorUserID),
	})
	if err != nil {
		return MeetingDTO{}, err
	}
	return toMeetingDTO(meeting), nil
}

func (s *Service) TransitionMeeting(ctx context.Context, actorUserID string, workspaceID string, channelID string, meetingID string, action string) (MeetingDTO, error) {
	if err := s.ensureCanManageCollaboration(ctx, actorUserID, workspaceID, channelID); err != nil {
		return MeetingDTO{}, err
	}
	action = strings.TrimSpace(action)
	if action != "start" && action != "end" && action != "cancel" {
		return MeetingDTO{}, apperrors.BadRequest("VALIDATION_ERROR", "Meeting action must be start, end or cancel.")
	}
	meeting, err := s.talkProductivityRepository().TransitionMeeting(ctx, TransitionMeetingParams{
		WorkspaceID: strings.TrimSpace(workspaceID), ChannelID: strings.TrimSpace(channelID),
		MeetingID: strings.TrimSpace(meetingID), Action: action, ActorUserID: strings.TrimSpace(actorUserID),
	})
	if err != nil {
		return MeetingDTO{}, err
	}
	return toMeetingDTO(meeting), nil
}

func (s *Service) GetVoiceRoom(ctx context.Context, actorUserID string, workspaceID string, channelID string) (VoiceRoomDTO, error) {
	if err := s.ensureCollaborationMember(ctx, actorUserID, workspaceID, channelID); err != nil {
		return VoiceRoomDTO{}, err
	}
	room, err := s.talkProductivityRepository().GetVoiceRoom(ctx, strings.TrimSpace(workspaceID), strings.TrimSpace(channelID))
	if err != nil {
		return VoiceRoomDTO{}, err
	}
	return toVoiceRoomDTO(room), nil
}

func (s *Service) StartVoiceRoom(ctx context.Context, actorUserID string, workspaceID string, channelID string) (VoiceRoomDTO, error) {
	if err := s.ensureCollaborationMember(ctx, actorUserID, workspaceID, channelID); err != nil {
		return VoiceRoomDTO{}, err
	}
	room, err := s.talkProductivityRepository().SetVoiceRoom(ctx, SetVoiceRoomParams{
		WorkspaceID: strings.TrimSpace(workspaceID), ChannelID: strings.TrimSpace(channelID),
		Status: "active", ActorUserID: strings.TrimSpace(actorUserID),
	})
	if err != nil {
		return VoiceRoomDTO{}, err
	}
	return toVoiceRoomDTO(room), nil
}

func (s *Service) StopVoiceRoom(ctx context.Context, actorUserID string, workspaceID string, channelID string) (VoiceRoomDTO, error) {
	if err := s.ensureCollaborationMember(ctx, actorUserID, workspaceID, channelID); err != nil {
		return VoiceRoomDTO{}, err
	}
	current, err := s.talkProductivityRepository().GetVoiceRoom(ctx, strings.TrimSpace(workspaceID), strings.TrimSpace(channelID))
	if err != nil {
		return VoiceRoomDTO{}, err
	}
	if current.StartedBy == nil || *current.StartedBy != strings.TrimSpace(actorUserID) {
		if err := s.ensureCanManageCollaboration(ctx, actorUserID, workspaceID, channelID); err != nil {
			return VoiceRoomDTO{}, err
		}
	}
	room, err := s.talkProductivityRepository().SetVoiceRoom(ctx, SetVoiceRoomParams{
		WorkspaceID: strings.TrimSpace(workspaceID), ChannelID: strings.TrimSpace(channelID),
		Status: "inactive", ActorUserID: strings.TrimSpace(actorUserID),
	})
	if err != nil {
		return VoiceRoomDTO{}, err
	}
	return toVoiceRoomDTO(room), nil
}

func (s *Service) SetupBreakoutRooms(ctx context.Context, input SetupBreakoutsInput) ([]BreakoutRoomDTO, error) {
	if err := s.ensureCanManageCollaboration(ctx, input.ActorUserID, input.WorkspaceID, input.ChannelID); err != nil {
		return nil, err
	}
	if err := s.ensureInternalBreakouts(ctx, input.WorkspaceID, input.ChannelID); err != nil {
		return nil, err
	}
	mode := strings.TrimSpace(input.AssignmentMode)
	if mode == "" {
		mode = "automatic"
	}
	if mode != "automatic" && mode != "manual" && mode != "self_select" {
		return nil, apperrors.BadRequest("VALIDATION_ERROR", "assignment_mode must be automatic, manual or self_select.")
	}
	members, err := s.repo.ListMembers(ctx, strings.TrimSpace(input.WorkspaceID), strings.TrimSpace(input.ChannelID))
	if err != nil {
		return nil, err
	}
	memberSet := make(map[string]struct{})
	activeIDs := make([]string, 0, len(members))
	for _, member := range members {
		if member.Status == "active" || member.Status == "muted" {
			memberSet[member.UserID] = struct{}{}
			activeIDs = append(activeIDs, member.UserID)
		}
	}
	sort.Strings(activeIDs)
	specs := input.Rooms
	if mode == "automatic" || mode == "self_select" {
		count := input.RoomCount
		if count < 2 {
			count = 2
		}
		if count > maxBreakoutRooms {
			return nil, apperrors.BadRequest("VALIDATION_ERROR", "room_count cannot exceed 20.")
		}
		specs = make([]BreakoutRoomSpec, count)
		for index := range specs {
			specs[index] = BreakoutRoomSpec{Name: "Room " + integerString(index+1), Sequence: index}
		}
		if mode == "automatic" {
			for index, userID := range activeIDs {
				target := index % count
				specs[target].AssignedUserIDs = append(specs[target].AssignedUserIDs, userID)
			}
		}
	}
	if len(specs) < 2 || len(specs) > maxBreakoutRooms {
		return nil, apperrors.BadRequest("VALIDATION_ERROR", "Create between 2 and 20 breakout rooms.")
	}
	seen := make(map[string]struct{})
	for index := range specs {
		specs[index].Name = strings.TrimSpace(specs[index].Name)
		if specs[index].Name == "" || len([]rune(specs[index].Name)) > 80 {
			return nil, apperrors.BadRequest("VALIDATION_ERROR", "Each breakout room needs a name of at most 80 characters.")
		}
		specs[index].Sequence = index
		specs[index].AssignedUserIDs = normalizeStringIDs(specs[index].AssignedUserIDs)
		for _, userID := range specs[index].AssignedUserIDs {
			if _, ok := memberSet[userID]; !ok {
				return nil, apperrors.BadRequest("INVALID_PARTICIPANTS", "A breakout participant is not an active channel member.")
			}
			if _, duplicate := seen[userID]; duplicate {
				return nil, apperrors.BadRequest("DUPLICATE_PARTICIPANT", "A participant can be assigned to only one breakout room.")
			}
			seen[userID] = struct{}{}
		}
	}
	rooms, err := s.talkProductivityRepository().ReplaceBreakoutRooms(ctx, ReplaceBreakoutRoomsParams{
		WorkspaceID: strings.TrimSpace(input.WorkspaceID), ChannelID: strings.TrimSpace(input.ChannelID),
		AssignmentMode: mode, AllowSelfSelect: input.AllowSelfSelect || mode == "self_select",
		Rooms: specs, ActorUserID: strings.TrimSpace(input.ActorUserID),
	})
	if err != nil {
		return nil, err
	}
	return toBreakoutRoomDTOs(rooms), nil
}

func (s *Service) StartBreakoutRooms(ctx context.Context, actorUserID string, workspaceID string, channelID string) ([]BreakoutRoomDTO, error) {
	if err := s.ensureCanManageCollaboration(ctx, actorUserID, workspaceID, channelID); err != nil {
		return nil, err
	}
	if err := s.ensureInternalBreakouts(ctx, workspaceID, channelID); err != nil {
		return nil, err
	}
	rooms, err := s.talkProductivityRepository().SetBreakoutRoomsStatus(ctx, strings.TrimSpace(workspaceID), strings.TrimSpace(channelID), "active")
	if err != nil {
		return nil, err
	}
	return toBreakoutRoomDTOs(rooms), nil
}

func (s *Service) JoinBreakoutRoom(ctx context.Context, actorUserID string, workspaceID string, channelID string, roomID string) ([]BreakoutRoomDTO, error) {
	if err := s.ensureCollaborationMember(ctx, actorUserID, workspaceID, channelID); err != nil {
		return nil, err
	}
	rooms, err := s.talkProductivityRepository().JoinBreakoutRoom(ctx, JoinBreakoutRoomParams{
		WorkspaceID: strings.TrimSpace(workspaceID), ChannelID: strings.TrimSpace(channelID),
		RoomID: strings.TrimSpace(roomID), UserID: strings.TrimSpace(actorUserID),
	})
	if err != nil {
		return nil, err
	}
	return toBreakoutRoomDTOs(rooms), nil
}

func (s *Service) UpdateBreakoutAssignments(ctx context.Context, actorUserID string, workspaceID string, channelID string, roomID string, userIDs []string) ([]BreakoutRoomDTO, error) {
	if err := s.ensureCanManageCollaboration(ctx, actorUserID, workspaceID, channelID); err != nil {
		return nil, err
	}
	userIDs = normalizeStringIDs(userIDs)
	for _, userID := range userIDs {
		member, err := s.repo.FindMember(ctx, strings.TrimSpace(workspaceID), strings.TrimSpace(channelID), userID)
		if err != nil || (member.Status != "active" && member.Status != "muted") {
			return nil, apperrors.BadRequest("INVALID_PARTICIPANTS", "A breakout participant is not an active channel member.")
		}
	}
	rooms, err := s.talkProductivityRepository().UpdateBreakoutAssignments(ctx, UpdateBreakoutAssignmentsParams{
		WorkspaceID: strings.TrimSpace(workspaceID), ChannelID: strings.TrimSpace(channelID),
		RoomID: strings.TrimSpace(roomID), AssignedUserIDs: userIDs,
	})
	if err != nil {
		return nil, err
	}
	return toBreakoutRoomDTOs(rooms), nil
}

func (s *Service) BroadcastToBreakouts(ctx context.Context, actorUserID string, workspaceID string, channelID string, body string) (BreakoutBroadcastDTO, error) {
	if err := s.ensureCanManageCollaboration(ctx, actorUserID, workspaceID, channelID); err != nil {
		return BreakoutBroadcastDTO{}, err
	}
	body = strings.TrimSpace(body)
	if body == "" || len([]rune(body)) > 2000 {
		return BreakoutBroadcastDTO{}, apperrors.BadRequest("VALIDATION_ERROR", "Broadcast must contain 1 to 2000 characters.")
	}
	broadcast, err := s.talkProductivityRepository().CreateBreakoutBroadcast(ctx, CreateBreakoutBroadcastParams{
		WorkspaceID: strings.TrimSpace(workspaceID), ChannelID: strings.TrimSpace(channelID),
		Body: body, ActorUserID: strings.TrimSpace(actorUserID),
	})
	if err != nil {
		return BreakoutBroadcastDTO{}, err
	}
	return BreakoutBroadcastDTO{
		ID: broadcast.ID, ChannelID: broadcast.ChannelID, Body: broadcast.Body,
		CreatedBy: broadcast.CreatedBy, CreatedAt: formatTime(broadcast.CreatedAt),
	}, nil
}

func (s *Service) GetTalkHome(ctx context.Context, actorUserID string, workspaceID string) (TalkHomeDTO, error) {
	if err := s.ensureWorkspaceAccess(ctx, actorUserID, workspaceID); err != nil {
		return TalkHomeDTO{}, err
	}
	home, err := s.talkProductivityRepository().GetTalkHome(ctx, strings.TrimSpace(workspaceID), strings.TrimSpace(actorUserID), time.Now().UTC())
	if err != nil {
		return TalkHomeDTO{}, err
	}
	tasks := make([]ChannelTaskDTO, 0, len(home.OpenTasks))
	for _, task := range home.OpenTasks {
		tasks = append(tasks, toChannelTaskDTO(task))
	}
	voices := make([]VoiceRoomDTO, 0, len(home.ActiveVoiceRooms))
	for _, room := range home.ActiveVoiceRooms {
		voices = append(voices, toVoiceRoomDTO(room))
	}
	return TalkHomeDTO{
		UpcomingMeetings: toMeetingDTOs(home.UpcomingMeetings), ActiveVoiceRooms: voices,
		OpenTasks: tasks, UnreadMentions: home.UnreadMentions,
		PendingReminders: home.PendingReminders, MissedCalls: home.MissedCalls,
	}, nil
}

func (s *Service) ListSharedItems(ctx context.Context, actorUserID string, workspaceID string, channelID string, kind string, limit int) ([]SharedItemDTO, error) {
	if err := s.ensureCollaborationMember(ctx, actorUserID, workspaceID, channelID); err != nil {
		return nil, err
	}
	kind = strings.TrimSpace(kind)
	switch kind {
	case "", "all", "file", "pin", "poll", "task", "recording":
	default:
		return nil, apperrors.BadRequest("VALIDATION_ERROR", "Unsupported shared item kind.")
	}
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	items, err := s.talkProductivityRepository().ListSharedItems(ctx, strings.TrimSpace(workspaceID), strings.TrimSpace(channelID), kind, limit)
	if err != nil {
		return nil, err
	}
	result := make([]SharedItemDTO, 0, len(items))
	for _, item := range items {
		result = append(result, SharedItemDTO{
			ID: item.ID, Kind: item.Kind, Title: item.Title, Subtitle: item.Subtitle,
			URL: item.URL, Metadata: item.Metadata, CreatedBy: item.CreatedBy,
			CreatedAt: formatTime(item.CreatedAt),
		})
	}
	return result, nil
}

func (s *Service) ensureInternalBreakouts(ctx context.Context, workspaceID string, channelID string) error {
	channel, err := s.repo.FindChannel(ctx, strings.TrimSpace(workspaceID), strings.TrimSpace(channelID))
	if err != nil {
		return mapChannelError(err)
	}
	if channel.Type == "direct" {
		return apperrors.Conflict("PROMOTION_REQUIRED", "Promote the direct conversation to a group before creating breakout rooms.")
	}
	settings, err := s.collaborationRepository().GetCollaborationSettings(ctx, strings.TrimSpace(workspaceID), strings.TrimSpace(channelID))
	if err != nil {
		return err
	}
	if settings.PublicAccessEnabled || settings.RoomMode == "public" || settings.RoomMode == "webinar" {
		return apperrors.Conflict("BREAKOUT_INTERNAL_ONLY", "Breakout rooms are available only in internal group conversations.")
	}
	return nil
}

func (s *Service) talkProductivityRepository() TalkProductivityRepository {
	repository, ok := s.collab.(TalkProductivityRepository)
	if !ok {
		return unavailableTalkRepository{}
	}
	return repository
}

func parseDateFilter(value string) (*time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return nil, err
	}
	parsed = parsed.UTC()
	return &parsed, nil
}

func toMeetingDTOs(meetings []Meeting) []MeetingDTO {
	result := make([]MeetingDTO, 0, len(meetings))
	for _, meeting := range meetings {
		result = append(result, toMeetingDTO(meeting))
	}
	return result
}

func toMeetingDTO(meeting Meeting) MeetingDTO {
	return MeetingDTO{
		ID: meeting.ID, WorkspaceID: meeting.WorkspaceID, ChannelID: meeting.ChannelID,
		Title: meeting.Title, Description: meeting.Description, StartsAt: formatTime(meeting.StartsAt),
		EndsAt: formatOptionalTime(meeting.EndsAt), LobbyOpensAt: formatOptionalTime(meeting.LobbyOpensAt),
		Status: meeting.Status, RoomPolicy: meeting.RoomPolicy, CleanupAfter: formatOptionalTime(meeting.CleanupAfter),
		CreatedBy: meeting.CreatedBy, StartedAt: formatOptionalTime(meeting.StartedAt),
		EndedAt: formatOptionalTime(meeting.EndedAt), CreatedAt: formatTime(meeting.CreatedAt),
		UpdatedAt: formatTime(meeting.UpdatedAt),
	}
}

func toVoiceRoomDTO(room VoiceRoom) VoiceRoomDTO {
	return VoiceRoomDTO{
		ChannelID: room.ChannelID, WorkspaceID: room.WorkspaceID, Status: room.Status,
		StartedBy: room.StartedBy, StartedAt: formatOptionalTime(room.StartedAt),
		EndedAt: formatOptionalTime(room.EndedAt), CreatedAt: formatTime(room.CreatedAt),
		UpdatedAt: formatTime(room.UpdatedAt),
	}
}

func integerString(value int) string {
	return strconv.Itoa(value)
}
