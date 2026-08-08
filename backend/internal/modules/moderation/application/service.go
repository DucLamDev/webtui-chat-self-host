package application

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"regexp"
	"strings"
	"time"

	moderationdomain "github.com/duclamdev/application-chat/backend/internal/modules/moderation/domain"
	apperrors "github.com/duclamdev/application-chat/backend/internal/shared/errors"
	"github.com/duclamdev/application-chat/backend/internal/shared/requestcontext"
)

const (
	PermissionModerationManage = "moderation.manage"
	MaxReportsPer24Hours       = 50
)

var uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

var reportReasons = map[string]struct{}{
	"spam": {}, "harassment": {}, "hate_speech": {}, "sexual_content": {},
	"violence": {}, "illegal_content": {}, "privacy": {}, "impersonation": {}, "other": {},
}

var reportStatuses = map[string]struct{}{
	"pending": {}, "reviewing": {}, "resolved": {}, "dismissed": {},
}

type PermissionChecker interface {
	HasWorkspacePermission(ctx context.Context, userID string, workspaceID string, permissionCode string) (bool, error)
}

type Repository interface {
	IsActiveWorkspaceMember(ctx context.Context, workspaceID string, userID string) (bool, error)
	ResolveReportTarget(ctx context.Context, workspaceID string, reporterUserID string, targetType string, targetID string) (moderationdomain.ReportTarget, error)
	CreateReport(ctx context.Context, params CreateReportParams) (moderationdomain.Report, error)
	ListReports(ctx context.Context, params ListReportsParams) ([]moderationdomain.Report, error)
	UpdateReport(ctx context.Context, params UpdateReportParams) (moderationdomain.Report, error)
	CreateBlock(ctx context.Context, params CreateBlockParams) (moderationdomain.UserBlock, error)
	DeleteBlock(ctx context.Context, workspaceID string, blockerUserID string, blockedUserID string) error
	ListBlocks(ctx context.Context, workspaceID string, blockerUserID string) ([]moderationdomain.UserBlock, error)
	IsInteractionBlocked(ctx context.Context, workspaceID string, firstUserID string, secondUserID string) (bool, error)
	IsDirectChannelBlocked(ctx context.Context, workspaceID string, channelID string, actorUserID string) (bool, error)
	RecordAudit(ctx context.Context, event AuditEvent) error
}

type Service struct {
	repo    Repository
	checker PermissionChecker
	now     func() time.Time
}

type CreateReportInput struct {
	ActorUserID string
	WorkspaceID string
	TargetType  string
	TargetID    string
	Reason      string
	Details     string
}

type CreateReportParams struct {
	WorkspaceID    string
	ReporterUserID string
	TargetType     string
	TargetID       string
	TargetUserID   string
	TargetSnapshot json.RawMessage
	Reason         string
	Details        string
	RateLimitSince time.Time
	MaxReports     int
}

type ListReportsInput struct {
	ActorUserID string
	WorkspaceID string
	Status      string
	TargetType  string
	Limit       int
	Offset      int
}

type ListReportsParams struct {
	WorkspaceID string
	Status      string
	TargetType  string
	Limit       int
	Offset      int
}

type UpdateReportInput struct {
	ActorUserID string
	WorkspaceID string
	ReportID    string
	Status      string
	Resolution  string
}

type UpdateReportParams struct {
	WorkspaceID string
	ReportID    string
	Status      string
	Resolution  string
	ResolvedBy  string
}

type CreateBlockInput struct {
	ActorUserID   string
	WorkspaceID   string
	BlockedUserID string
	Reason        string
}

type CreateBlockParams struct {
	WorkspaceID   string
	BlockerUserID string
	BlockedUserID string
	Reason        string
}

type AuditEvent struct {
	ActorUserID string
	WorkspaceID string
	Action      string
	EntityType  string
	EntityID    string
	Metadata    map[string]any
}

type ReportDTO struct {
	ID                    string          `json:"id"`
	WorkspaceID           string          `json:"workspace_id"`
	ReporterUserID        *string         `json:"reporter_user_id,omitempty"`
	ReporterDisplayName   *string         `json:"reporter_display_name,omitempty"`
	TargetType            string          `json:"target_type"`
	TargetID              string          `json:"target_id"`
	TargetUserID          *string         `json:"target_user_id,omitempty"`
	TargetUserDisplayName *string         `json:"target_user_display_name,omitempty"`
	TargetSnapshot        json.RawMessage `json:"target_snapshot,omitempty"`
	Reason                string          `json:"reason"`
	Details               *string         `json:"details,omitempty"`
	Status                string          `json:"status"`
	ResolutionNote        *string         `json:"resolution_note,omitempty"`
	ResolvedBy            *string         `json:"resolved_by,omitempty"`
	ResolvedAt            *string         `json:"resolved_at,omitempty"`
	CreatedAt             string          `json:"created_at"`
	UpdatedAt             string          `json:"updated_at"`
}

type BlockDTO struct {
	ID                 string  `json:"id"`
	WorkspaceID        string  `json:"workspace_id"`
	BlockedUserID      string  `json:"blocked_user_id"`
	BlockedUsername    string  `json:"blocked_username"`
	BlockedDisplayName string  `json:"blocked_display_name"`
	BlockedAvatarURL   *string `json:"blocked_avatar_url,omitempty"`
	Reason             *string `json:"reason,omitempty"`
	CreatedAt          string  `json:"created_at"`
	UpdatedAt          string  `json:"updated_at"`
}

func NewService(repo Repository, checker PermissionChecker) *Service {
	return &Service{repo: repo, checker: checker, now: time.Now}
}

func (s *Service) CreateReport(ctx context.Context, input CreateReportInput) (ReportDTO, error) {
	input.ActorUserID = strings.TrimSpace(input.ActorUserID)
	input.WorkspaceID = strings.TrimSpace(input.WorkspaceID)
	input.TargetType = strings.ToLower(strings.TrimSpace(input.TargetType))
	input.TargetID = strings.TrimSpace(input.TargetID)
	input.Reason = strings.ToLower(strings.TrimSpace(input.Reason))
	input.Details = strings.TrimSpace(input.Details)

	if !validUUID(input.ActorUserID) || !validUUID(input.WorkspaceID) || !validUUID(input.TargetID) {
		return ReportDTO{}, apperrors.BadRequest("VALIDATION_ERROR", "workspace, actor, and target identifiers must be valid UUIDs.")
	}
	if input.TargetType != "message" && input.TargetType != "user" {
		return ReportDTO{}, apperrors.BadRequest("INVALID_REPORT_TARGET", "target_type must be message or user.")
	}
	if _, ok := reportReasons[input.Reason]; !ok {
		return ReportDTO{}, apperrors.BadRequest("INVALID_REPORT_REASON", "The report reason is not supported.")
	}
	if len([]rune(input.Details)) > 2000 {
		return ReportDTO{}, apperrors.BadRequest("VALIDATION_ERROR", "Report details must not exceed 2000 characters.")
	}
	member, err := s.repo.IsActiveWorkspaceMember(ctx, input.WorkspaceID, input.ActorUserID)
	if err != nil {
		return ReportDTO{}, err
	}
	if !member {
		return ReportDTO{}, apperrors.Forbidden("You must be an active workspace member to submit a report.")
	}
	target, err := s.repo.ResolveReportTarget(ctx, input.WorkspaceID, input.ActorUserID, input.TargetType, input.TargetID)
	if err != nil {
		return ReportDTO{}, mapModerationError(err)
	}
	if target.UserID == input.ActorUserID {
		return ReportDTO{}, apperrors.BadRequest("SELF_REPORT_NOT_ALLOWED", "You cannot report yourself or your own message.")
	}
	report, err := s.repo.CreateReport(ctx, CreateReportParams{
		WorkspaceID: input.WorkspaceID, ReporterUserID: input.ActorUserID,
		TargetType: input.TargetType, TargetID: input.TargetID, TargetUserID: target.UserID,
		TargetSnapshot: target.Snapshot,
		Reason:         input.Reason, Details: input.Details,
		RateLimitSince: s.now().UTC().Add(-24 * time.Hour), MaxReports: MaxReportsPer24Hours,
	})
	if err != nil {
		return ReportDTO{}, mapModerationError(err)
	}
	s.recordAudit(ctx, AuditEvent{
		ActorUserID: input.ActorUserID, WorkspaceID: input.WorkspaceID,
		Action: "moderation.report.create", EntityType: "moderation_report", EntityID: report.ID,
		Metadata: map[string]any{"target_type": input.TargetType, "target_id": input.TargetID, "reason": input.Reason},
	})
	dto := toReportDTO(report)
	// The immutable evidence snapshot is for the permissioned moderation queue,
	// not part of the reporter's creation receipt.
	dto.TargetSnapshot = nil
	return dto, nil
}

func (s *Service) ListReports(ctx context.Context, input ListReportsInput) ([]ReportDTO, error) {
	input.ActorUserID = strings.TrimSpace(input.ActorUserID)
	input.WorkspaceID = strings.TrimSpace(input.WorkspaceID)
	input.Status = strings.ToLower(strings.TrimSpace(input.Status))
	input.TargetType = strings.ToLower(strings.TrimSpace(input.TargetType))
	if err := s.ensureModerator(ctx, input.ActorUserID, input.WorkspaceID); err != nil {
		return nil, err
	}
	if input.Status != "" {
		if _, ok := reportStatuses[input.Status]; !ok {
			return nil, apperrors.BadRequest("INVALID_REPORT_STATUS", "The report status filter is invalid.")
		}
	}
	if input.TargetType != "" && input.TargetType != "message" && input.TargetType != "user" {
		return nil, apperrors.BadRequest("INVALID_REPORT_TARGET", "target_type must be message or user.")
	}
	if input.Limit <= 0 {
		input.Limit = 50
	}
	if input.Limit > 100 {
		input.Limit = 100
	}
	if input.Offset < 0 {
		input.Offset = 0
	}
	reports, err := s.repo.ListReports(ctx, ListReportsParams{
		WorkspaceID: input.WorkspaceID, Status: input.Status, TargetType: input.TargetType,
		Limit: input.Limit, Offset: input.Offset,
	})
	if err != nil {
		return nil, err
	}
	result := make([]ReportDTO, 0, len(reports))
	for _, report := range reports {
		result = append(result, toReportDTO(report))
	}
	return result, nil
}

func (s *Service) UpdateReport(ctx context.Context, input UpdateReportInput) (ReportDTO, error) {
	input.ActorUserID = strings.TrimSpace(input.ActorUserID)
	input.WorkspaceID = strings.TrimSpace(input.WorkspaceID)
	input.ReportID = strings.TrimSpace(input.ReportID)
	input.Status = strings.ToLower(strings.TrimSpace(input.Status))
	input.Resolution = strings.TrimSpace(input.Resolution)
	if err := s.ensureModerator(ctx, input.ActorUserID, input.WorkspaceID); err != nil {
		return ReportDTO{}, err
	}
	if !validUUID(input.ReportID) {
		return ReportDTO{}, apperrors.BadRequest("VALIDATION_ERROR", "report_id must be a valid UUID.")
	}
	if _, ok := reportStatuses[input.Status]; !ok {
		return ReportDTO{}, apperrors.BadRequest("INVALID_REPORT_STATUS", "status must be pending, reviewing, resolved, or dismissed.")
	}
	if len([]rune(input.Resolution)) > 2000 {
		return ReportDTO{}, apperrors.BadRequest("VALIDATION_ERROR", "resolution_note must not exceed 2000 characters.")
	}
	if (input.Status == "resolved" || input.Status == "dismissed") && input.Resolution == "" {
		return ReportDTO{}, apperrors.BadRequest("RESOLUTION_REQUIRED", "A resolution note is required to close a report.")
	}
	report, err := s.repo.UpdateReport(ctx, UpdateReportParams{
		WorkspaceID: input.WorkspaceID, ReportID: input.ReportID, Status: input.Status,
		Resolution: input.Resolution, ResolvedBy: input.ActorUserID,
	})
	if err != nil {
		return ReportDTO{}, mapModerationError(err)
	}
	s.recordAudit(ctx, AuditEvent{
		ActorUserID: input.ActorUserID, WorkspaceID: input.WorkspaceID,
		Action: "moderation.report.update", EntityType: "moderation_report", EntityID: report.ID,
		Metadata: map[string]any{"status": input.Status},
	})
	return toReportDTO(report), nil
}

func (s *Service) CreateBlock(ctx context.Context, input CreateBlockInput) (BlockDTO, error) {
	input.ActorUserID = strings.TrimSpace(input.ActorUserID)
	input.WorkspaceID = strings.TrimSpace(input.WorkspaceID)
	input.BlockedUserID = strings.TrimSpace(input.BlockedUserID)
	input.Reason = strings.TrimSpace(input.Reason)
	if !validUUID(input.ActorUserID) || !validUUID(input.WorkspaceID) || !validUUID(input.BlockedUserID) {
		return BlockDTO{}, apperrors.BadRequest("VALIDATION_ERROR", "workspace and user identifiers must be valid UUIDs.")
	}
	if input.ActorUserID == input.BlockedUserID {
		return BlockDTO{}, apperrors.BadRequest("SELF_BLOCK_NOT_ALLOWED", "You cannot block yourself.")
	}
	if len([]rune(input.Reason)) > 500 {
		return BlockDTO{}, apperrors.BadRequest("VALIDATION_ERROR", "Block reason must not exceed 500 characters.")
	}
	for _, userID := range []string{input.ActorUserID, input.BlockedUserID} {
		member, err := s.repo.IsActiveWorkspaceMember(ctx, input.WorkspaceID, userID)
		if err != nil {
			return BlockDTO{}, err
		}
		if !member {
			return BlockDTO{}, apperrors.NotFound("WORKSPACE_MEMBER_NOT_FOUND", "The selected user is not an active workspace member.")
		}
	}
	block, err := s.repo.CreateBlock(ctx, CreateBlockParams{
		WorkspaceID: input.WorkspaceID, BlockerUserID: input.ActorUserID,
		BlockedUserID: input.BlockedUserID, Reason: input.Reason,
	})
	if err != nil {
		return BlockDTO{}, mapModerationError(err)
	}
	s.recordAudit(ctx, AuditEvent{
		ActorUserID: input.ActorUserID, WorkspaceID: input.WorkspaceID,
		Action: "user.block", EntityType: "user", EntityID: input.BlockedUserID,
	})
	return toBlockDTO(block), nil
}

func (s *Service) DeleteBlock(ctx context.Context, actorUserID string, workspaceID string, blockedUserID string) error {
	actorUserID = strings.TrimSpace(actorUserID)
	workspaceID = strings.TrimSpace(workspaceID)
	blockedUserID = strings.TrimSpace(blockedUserID)
	if !validUUID(actorUserID) || !validUUID(workspaceID) || !validUUID(blockedUserID) {
		return apperrors.BadRequest("VALIDATION_ERROR", "workspace and user identifiers must be valid UUIDs.")
	}
	member, err := s.repo.IsActiveWorkspaceMember(ctx, workspaceID, actorUserID)
	if err != nil {
		return err
	}
	if !member {
		return apperrors.Forbidden("You must be an active workspace member to manage blocks.")
	}
	if err := s.repo.DeleteBlock(ctx, workspaceID, actorUserID, blockedUserID); err != nil {
		return mapModerationError(err)
	}
	s.recordAudit(ctx, AuditEvent{
		ActorUserID: actorUserID, WorkspaceID: workspaceID,
		Action: "user.unblock", EntityType: "user", EntityID: blockedUserID,
	})
	return nil
}

func (s *Service) ListBlocks(ctx context.Context, actorUserID string, workspaceID string) ([]BlockDTO, error) {
	actorUserID = strings.TrimSpace(actorUserID)
	workspaceID = strings.TrimSpace(workspaceID)
	if !validUUID(actorUserID) || !validUUID(workspaceID) {
		return nil, apperrors.BadRequest("VALIDATION_ERROR", "workspace and actor identifiers must be valid UUIDs.")
	}
	member, err := s.repo.IsActiveWorkspaceMember(ctx, workspaceID, actorUserID)
	if err != nil {
		return nil, err
	}
	if !member {
		return nil, apperrors.Forbidden("You must be an active workspace member to list blocks.")
	}
	blocks, err := s.repo.ListBlocks(ctx, workspaceID, actorUserID)
	if err != nil {
		return nil, err
	}
	result := make([]BlockDTO, 0, len(blocks))
	for _, block := range blocks {
		result = append(result, toBlockDTO(block))
	}
	return result, nil
}

// IsInteractionBlocked is implemented on Service so the chat/call modules can
// depend on a narrow policy interface instead of importing persistence code.
func (s *Service) IsInteractionBlocked(ctx context.Context, workspaceID string, firstUserID string, secondUserID string) (bool, error) {
	return s.repo.IsInteractionBlocked(ctx, strings.TrimSpace(workspaceID), strings.TrimSpace(firstUserID), strings.TrimSpace(secondUserID))
}

func (s *Service) IsDirectChannelBlocked(ctx context.Context, workspaceID string, channelID string, actorUserID string) (bool, error) {
	return s.repo.IsDirectChannelBlocked(ctx, strings.TrimSpace(workspaceID), strings.TrimSpace(channelID), strings.TrimSpace(actorUserID))
}

// recordAudit deliberately does not fail an already committed user mutation:
// returning a retryable error here could make clients duplicate reports or
// other moderation actions. A structured error keeps audit degradation visible
// to operators and correlatable with the action/entity in application logs.
func (s *Service) recordAudit(ctx context.Context, event AuditEvent) {
	if err := s.repo.RecordAudit(ctx, event); err != nil {
		slog.ErrorContext(ctx, "moderation audit write failed after mutation",
			"error", err,
			"action", event.Action,
			"workspace_id", event.WorkspaceID,
			"entity_type", event.EntityType,
			"entity_id", event.EntityID,
			"actor_user_id", event.ActorUserID,
			"request_id", requestcontext.RequestID(ctx),
		)
	}
}

func (s *Service) ensureModerator(ctx context.Context, actorUserID string, workspaceID string) error {
	if !validUUID(actorUserID) || !validUUID(workspaceID) {
		return apperrors.BadRequest("VALIDATION_ERROR", "workspace and actor identifiers must be valid UUIDs.")
	}
	member, err := s.repo.IsActiveWorkspaceMember(ctx, workspaceID, actorUserID)
	if err != nil {
		return err
	}
	if !member {
		return apperrors.Forbidden("You must be an active workspace member to manage moderation reports.")
	}
	allowed, err := s.checker.HasWorkspacePermission(ctx, actorUserID, workspaceID, PermissionModerationManage)
	if err != nil {
		return err
	}
	if !allowed {
		return apperrors.Forbidden("You do not have permission to manage moderation reports.")
	}
	return nil
}

func mapModerationError(err error) error {
	switch {
	case errors.Is(err, moderationdomain.ErrReportNotFound):
		return apperrors.NotFound("REPORT_NOT_FOUND", "The moderation report was not found.")
	case errors.Is(err, moderationdomain.ErrReportDuplicate):
		return apperrors.Conflict("REPORT_ALREADY_OPEN", "You already have an open report for this target.")
	case errors.Is(err, moderationdomain.ErrReportRateLimit):
		return apperrors.New("REPORT_RATE_LIMITED", "Too many reports were submitted. Try again later.", 429)
	case errors.Is(err, moderationdomain.ErrReportTarget):
		return apperrors.NotFound("REPORT_TARGET_NOT_FOUND", "The report target was not found or is not visible to you.")
	case errors.Is(err, moderationdomain.ErrWorkspaceMember):
		return apperrors.NotFound("WORKSPACE_MEMBER_NOT_FOUND", "The selected user is not an active workspace member.")
	case errors.Is(err, moderationdomain.ErrInvalidBlockPair):
		return apperrors.BadRequest("INVALID_BLOCK", "The selected user cannot be blocked.")
	default:
		return err
	}
}

func toReportDTO(report moderationdomain.Report) ReportDTO {
	return ReportDTO{
		ID: report.ID, WorkspaceID: report.WorkspaceID,
		ReporterUserID: report.ReporterUserID, ReporterDisplayName: report.ReporterDisplayName,
		TargetType: report.TargetType, TargetID: report.TargetID,
		TargetUserID: report.TargetUserID, TargetUserDisplayName: report.TargetUserDisplayName,
		TargetSnapshot: report.TargetSnapshot,
		Reason:         report.Reason, Details: report.Details, Status: report.Status,
		ResolutionNote: report.ResolutionNote, ResolvedBy: report.ResolvedBy,
		ResolvedAt: formatOptionalTime(report.ResolvedAt),
		CreatedAt:  report.CreatedAt.UTC().Format(time.RFC3339Nano),
		UpdatedAt:  report.UpdatedAt.UTC().Format(time.RFC3339Nano),
	}
}

func toBlockDTO(block moderationdomain.UserBlock) BlockDTO {
	return BlockDTO{
		ID: block.ID, WorkspaceID: block.WorkspaceID, BlockedUserID: block.BlockedUserID,
		BlockedUsername: block.BlockedUsername, BlockedDisplayName: block.BlockedDisplayName,
		BlockedAvatarURL: block.BlockedAvatarURL, Reason: block.Reason,
		CreatedAt: block.CreatedAt.UTC().Format(time.RFC3339Nano),
		UpdatedAt: block.UpdatedAt.UTC().Format(time.RFC3339Nano),
	}
}

func formatOptionalTime(value *time.Time) *string {
	if value == nil {
		return nil
	}
	formatted := value.UTC().Format(time.RFC3339Nano)
	return &formatted
}

func validUUID(value string) bool {
	return uuidPattern.MatchString(strings.TrimSpace(value))
}
