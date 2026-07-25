package application

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/mail"
	"regexp"
	"strings"
	"time"

	workspacesdomain "github.com/duclamdev/application-chat/backend/internal/modules/workspaces/domain"
	apperrors "github.com/duclamdev/application-chat/backend/internal/shared/errors"
)

var slugPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,61}[a-z0-9]$`)

// DefaultChannelDefinition describes a public channel provisioned for every
// VPSTTT workspace. Keeping the catalogue in application code gives the
// create-workspace transaction a single, testable source of truth.
type DefaultChannelDefinition struct {
	Slug           string
	Name           string
	Description    string
	Type           string
	PrivateSession bool
}

type DefaultBotDefinition struct {
	Slug        string
	Name        string
	Description string
	ChannelSlug string
}

type WorkspaceTemplate struct {
	Key      string
	Channels []DefaultChannelDefinition
	Bots     []DefaultBotDefinition
}

func DefaultWorkspaceChannels() []DefaultChannelDefinition {
	return []DefaultChannelDefinition{
		{Slug: "thong-bao", Name: "Thông báo", Description: "Thông báo chung của workspace", Type: "public"},
		{Slug: "ban-giam-doc", Name: "Ban giám đốc", Description: "Trao đổi dành cho quản lý cấp cao", Type: "private"},
		{Slug: "ky-thuat", Name: "Kỹ thuật", Description: "Kỹ thuật và vận hành hệ thống", Type: "public"},
		{Slug: "sale", Name: "Sale", Description: "Kinh doanh và chăm sóc khách hàng", Type: "public"},
		{Slug: "ke-toan", Name: "Kế toán", Description: "Hóa đơn và thanh toán", Type: "private", PrivateSession: true},
		{Slug: "ticket", Name: "Ticket", Description: "Tiếp nhận ticket khách hàng", Type: "public", PrivateSession: true},
		{Slug: "server-alert", Name: "Server Alert", Description: "Cảnh báo server và dịch vụ", Type: "public"},
		{Slug: "gia-han", Name: "Gia hạn", Description: "Theo dõi gia hạn dịch vụ", Type: "public", PrivateSession: true},
		{Slug: "ban-giao-ca", Name: "Bàn giao ca", Description: "Bàn giao ca trực vận hành", Type: "public"},
	}
}

func DefaultWorkspaceBots() []DefaultBotDefinition {
	return []DefaultBotDefinition{
		{Slug: "cskh-bot", Name: "CSKH Bot", Description: "Tra cứu ví, dịch vụ và hỗ trợ khách hàng từ hệ thống order VPSTTT", ChannelSlug: "ticket"},
		{Slug: "ticket-bot", Name: "Ticket Bot", Description: "Báo ticket mới từ web bán hàng/billing", ChannelSlug: "ticket"},
		{Slug: "server-alert-bot", Name: "Server Alert Bot", Description: "Báo server mất ping, port lỗi, dịch vụ down", ChannelSlug: "server-alert"},
		{Slug: "gia-han-bot", Name: "Gia Hạn Bot", Description: "Báo khách hoặc dịch vụ sắp hết hạn", ChannelSlug: "gia-han"},
		{Slug: "thanh-toan-bot", Name: "Thanh Toán Bot", Description: "Tạo QR nạp ví và hỗ trợ kiểm tra thanh toán", ChannelSlug: "ke-toan"},
	}
}

func CustomerWorkspaceChannels() []DefaultChannelDefinition {
	return []DefaultChannelDefinition{
		{Slug: "general", Name: "General", Description: "Trao doi chung trong workspace", Type: "public"},
		{Slug: "announcements", Name: "Announcements", Description: "Thong bao noi bo cua workspace", Type: "public"},
	}
}

func WorkspaceTemplateForZone(zoneKind string) WorkspaceTemplate {
	if strings.TrimSpace(zoneKind) == "vpsttt_internal" {
		return WorkspaceTemplate{
			Key:      "vpsttt_services",
			Channels: DefaultWorkspaceChannels(),
			Bots:     DefaultWorkspaceBots(),
		}
	}
	return WorkspaceTemplate{
		Key:      "customer_standard",
		Channels: CustomerWorkspaceChannels(),
		Bots:     []DefaultBotDefinition{},
	}
}

type PermissionChecker interface {
	HasWorkspacePermission(ctx context.Context, userID string, workspaceID string, permissionCode string) (bool, error)
	HasAnyWorkspacePermission(ctx context.Context, userID string, permissionCode string) (bool, error)
	HasAnyZonePermission(ctx context.Context, userID string, zoneID string, permissionCode string) (bool, error)
}

type Repository interface {
	CreateWorkspace(ctx context.Context, params CreateWorkspaceParams) (workspacesdomain.Workspace, error)
	FindWorkspace(ctx context.Context, workspaceID string) (workspacesdomain.Workspace, error)
	ListWorkspacesForUser(ctx context.Context, userID string, zoneID string) ([]workspacesdomain.Workspace, error)
	UpdateWorkspace(ctx context.Context, params UpdateWorkspaceParams) (workspacesdomain.Workspace, error)
	ArchiveWorkspace(ctx context.Context, workspaceID string) error
	ListMembers(ctx context.Context, workspaceID string) ([]workspacesdomain.Member, error)
	AddMember(ctx context.Context, params AddMemberParams) (workspacesdomain.Member, error)
	UpdateMemberStatus(ctx context.Context, params UpdateMemberStatusParams) (workspacesdomain.Member, error)
	UpsertSetting(ctx context.Context, params UpsertSettingParams) (workspacesdomain.Setting, error)
	ListSettings(ctx context.Context, workspaceID string) ([]workspacesdomain.Setting, error)
	CreateInvite(ctx context.Context, params CreateInviteParams) (workspacesdomain.Invite, error)
	ListInvites(ctx context.Context, workspaceID string) ([]workspacesdomain.Invite, error)
	RecordAudit(ctx context.Context, event AuditEvent) error
}

type Service struct {
	repo    Repository
	checker PermissionChecker
	now     func() time.Time
}

type CreateWorkspaceInput struct {
	ActorUserID string
	ZoneID      string
	Slug        string
	Name        string
	Description string
}

type CreateWorkspaceParams struct {
	OwnerID     string
	ZoneID      string
	Slug        string
	Name        string
	Description string
}

type UpdateWorkspaceInput struct {
	ActorUserID string
	WorkspaceID string
	Name        *string
	Description *string
}

type UpdateWorkspaceParams struct {
	WorkspaceID string
	Name        *string
	Description *string
}

type AddMemberInput struct {
	ActorUserID string
	WorkspaceID string
	UserID      string
	Title       string
	RoleCode    string
}

type AddMemberParams struct {
	WorkspaceID string
	UserID      string
	Title       string
	RoleCode    string
	AssignedBy  string
}

type UpdateMemberStatusInput struct {
	ActorUserID string
	WorkspaceID string
	UserID      string
	Status      string
}

type UpdateMemberStatusParams struct {
	WorkspaceID string
	UserID      string
	Status      string
}

type UpsertSettingInput struct {
	ActorUserID string
	WorkspaceID string
	Key         string
	Value       json.RawMessage
	ValueType   string
	Description string
}

type UpsertSettingParams struct {
	WorkspaceID string
	Key         string
	Value       []byte
	ValueType   string
	Description string
	UpdatedBy   string
}

type CreateInviteInput struct {
	ActorUserID string
	WorkspaceID string
	Email       string
	RoleCode    string
	ExpiresDays int
}

type CreateInviteParams struct {
	WorkspaceID string
	Email       string
	RoleCode    string
	TokenHash   string
	InvitedBy   string
	ExpiresAt   time.Time
}

type AuditEvent struct {
	ActorUserID string
	WorkspaceID string
	Action      string
	EntityType  string
	EntityID    string
	Metadata    map[string]any
}

type WorkspaceDTO struct {
	ID          string  `json:"id"`
	ZoneID      string  `json:"zone_id"`
	Slug        string  `json:"slug"`
	Name        string  `json:"name"`
	Description *string `json:"description,omitempty"`
	OwnerID     *string `json:"owner_id,omitempty"`
	Plan        string  `json:"plan"`
	Status      string  `json:"status"`
	CreatedAt   string  `json:"created_at"`
	UpdatedAt   string  `json:"updated_at"`
}

type MemberDTO struct {
	ID          string  `json:"id"`
	WorkspaceID string  `json:"workspace_id"`
	UserID      string  `json:"user_id"`
	Email       string  `json:"email"`
	Username    string  `json:"username"`
	DisplayName string  `json:"display_name"`
	AvatarURL   *string `json:"avatar_url,omitempty"`
	PhoneNumber *string `json:"phone_number,omitempty"`
	Status      string  `json:"status"`
	Title       *string `json:"title,omitempty"`
	JoinedAt    *string `json:"joined_at,omitempty"`
	CreatedAt   string  `json:"created_at"`
	UpdatedAt   string  `json:"updated_at"`
}

type SettingDTO struct {
	WorkspaceID string          `json:"workspace_id"`
	Key         string          `json:"key"`
	Value       json.RawMessage `json:"value"`
	ValueType   string          `json:"value_type"`
	Description *string         `json:"description,omitempty"`
	UpdatedBy   *string         `json:"updated_by,omitempty"`
	CreatedAt   string          `json:"created_at"`
	UpdatedAt   string          `json:"updated_at"`
}

type InviteDTO struct {
	ID          string  `json:"id"`
	WorkspaceID string  `json:"workspace_id"`
	Email       string  `json:"email"`
	RoleID      *string `json:"role_id,omitempty"`
	InvitedBy   *string `json:"invited_by,omitempty"`
	Token       string  `json:"token,omitempty"`
	ExpiresAt   string  `json:"expires_at"`
	AcceptedAt  *string `json:"accepted_at,omitempty"`
	RevokedAt   *string `json:"revoked_at,omitempty"`
	CreatedAt   string  `json:"created_at"`
}

func NewService(repo Repository, checker PermissionChecker) *Service {
	return &Service{repo: repo, checker: checker, now: time.Now}
}

func (s *Service) Create(ctx context.Context, input CreateWorkspaceInput) (WorkspaceDTO, error) {
	input.ActorUserID = strings.TrimSpace(input.ActorUserID)
	input.ZoneID = strings.TrimSpace(input.ZoneID)
	if input.ZoneID == "" {
		return WorkspaceDTO{}, apperrors.BadRequest("ZONE_REQUIRED", "Khong xac dinh duoc zone hien tai.")
	}
	allowed, err := s.checker.HasAnyZonePermission(ctx, input.ActorUserID, input.ZoneID, "workspace.manage")
	if err != nil {
		return WorkspaceDTO{}, err
	}
	if !allowed {
		return WorkspaceDTO{}, apperrors.Forbidden("Chỉ quản trị viên workspace mới có thể tạo workspace mới.")
	}
	input.Slug = strings.ToLower(strings.TrimSpace(input.Slug))
	input.Name = strings.TrimSpace(input.Name)
	input.Description = strings.TrimSpace(input.Description)
	if !slugPattern.MatchString(input.Slug) {
		return WorkspaceDTO{}, apperrors.BadRequest("VALIDATION_ERROR", "Slug workspace phải dài 3-63 ký tự và chỉ gồm chữ thường, số hoặc dấu gạch ngang.")
	}
	if input.Name == "" || len([]rune(input.Name)) > 120 {
		return WorkspaceDTO{}, apperrors.BadRequest("VALIDATION_ERROR", "Tên workspace phải dài từ 1 đến 120 ký tự.")
	}

	workspace, err := s.repo.CreateWorkspace(ctx, CreateWorkspaceParams{
		OwnerID:     input.ActorUserID,
		ZoneID:      input.ZoneID,
		Slug:        input.Slug,
		Name:        input.Name,
		Description: input.Description,
	})
	if err != nil {
		if errors.Is(err, workspacesdomain.ErrWorkspaceQuotaExceeded) {
			return WorkspaceDTO{}, apperrors.Conflict("ZONE_QUOTA_EXCEEDED", "Zone da dat gioi han workspace.")
		}
		if errors.Is(err, workspacesdomain.ErrWorkspaceConflict) {
			return WorkspaceDTO{}, apperrors.Conflict("WORKSPACE_ALREADY_EXISTS", "Slug workspace đã tồn tại.")
		}
		return WorkspaceDTO{}, err
	}
	_ = s.repo.RecordAudit(ctx, AuditEvent{
		ActorUserID: input.ActorUserID,
		WorkspaceID: workspace.ID,
		Action:      "workspace.create",
		EntityType:  "workspace",
		EntityID:    workspace.ID,
	})
	return toWorkspaceDTO(workspace), nil
}

func (s *Service) Get(ctx context.Context, userID string, workspaceID string) (WorkspaceDTO, error) {
	if err := s.ensureMember(ctx, userID, workspaceID); err != nil {
		return WorkspaceDTO{}, err
	}
	workspace, err := s.repo.FindWorkspace(ctx, strings.TrimSpace(workspaceID))
	if err != nil {
		return WorkspaceDTO{}, mapWorkspaceError(err)
	}
	return toWorkspaceDTO(workspace), nil
}

func (s *Service) ListMine(ctx context.Context, userID string, zoneID string) ([]WorkspaceDTO, error) {
	workspaces, err := s.repo.ListWorkspacesForUser(ctx, strings.TrimSpace(userID), strings.TrimSpace(zoneID))
	if err != nil {
		return nil, err
	}
	dtos := make([]WorkspaceDTO, 0, len(workspaces))
	for _, workspace := range workspaces {
		dtos = append(dtos, toWorkspaceDTO(workspace))
	}
	return dtos, nil
}

func (s *Service) Update(ctx context.Context, input UpdateWorkspaceInput) (WorkspaceDTO, error) {
	if err := s.ensurePermission(ctx, input.ActorUserID, input.WorkspaceID, "workspace.manage"); err != nil {
		return WorkspaceDTO{}, err
	}
	workspace, err := s.repo.UpdateWorkspace(ctx, UpdateWorkspaceParams{
		WorkspaceID: strings.TrimSpace(input.WorkspaceID),
		Name:        cleanOptional(input.Name),
		Description: cleanOptional(input.Description),
	})
	if err != nil {
		return WorkspaceDTO{}, mapWorkspaceError(err)
	}
	_ = s.repo.RecordAudit(ctx, AuditEvent{
		ActorUserID: input.ActorUserID,
		WorkspaceID: input.WorkspaceID,
		Action:      "workspace.update",
		EntityType:  "workspace",
		EntityID:    input.WorkspaceID,
	})
	return toWorkspaceDTO(workspace), nil
}

func (s *Service) Archive(ctx context.Context, actorUserID string, workspaceID string) error {
	if err := s.ensurePermission(ctx, actorUserID, workspaceID, "workspace.manage"); err != nil {
		return err
	}
	if err := s.repo.ArchiveWorkspace(ctx, strings.TrimSpace(workspaceID)); err != nil {
		return mapWorkspaceError(err)
	}
	_ = s.repo.RecordAudit(ctx, AuditEvent{
		ActorUserID: actorUserID,
		WorkspaceID: workspaceID,
		Action:      "workspace.archive",
		EntityType:  "workspace",
		EntityID:    workspaceID,
	})
	return nil
}

func (s *Service) ListMembers(ctx context.Context, actorUserID string, workspaceID string) ([]MemberDTO, error) {
	if err := s.ensurePermission(ctx, actorUserID, workspaceID, "workspace.view_members"); err != nil {
		return nil, err
	}
	members, err := s.repo.ListMembers(ctx, strings.TrimSpace(workspaceID))
	if err != nil {
		return nil, err
	}
	return toMemberDTOs(members), nil
}

func (s *Service) AddMember(ctx context.Context, input AddMemberInput) (MemberDTO, error) {
	if err := s.ensurePermission(ctx, input.ActorUserID, input.WorkspaceID, "workspace.invite_user"); err != nil {
		return MemberDTO{}, err
	}
	input.RoleCode = strings.TrimSpace(input.RoleCode)
	if input.RoleCode == "" {
		input.RoleCode = "workspace_member"
	}
	member, err := s.repo.AddMember(ctx, AddMemberParams{
		WorkspaceID: strings.TrimSpace(input.WorkspaceID),
		UserID:      strings.TrimSpace(input.UserID),
		Title:       strings.TrimSpace(input.Title),
		RoleCode:    input.RoleCode,
		AssignedBy:  strings.TrimSpace(input.ActorUserID),
	})
	if err != nil {
		return MemberDTO{}, mapMemberError(err)
	}
	_ = s.repo.RecordAudit(ctx, AuditEvent{
		ActorUserID: input.ActorUserID,
		WorkspaceID: input.WorkspaceID,
		Action:      "workspace.member.add",
		EntityType:  "workspace_member",
		EntityID:    input.UserID,
		Metadata: map[string]any{
			"role_code": input.RoleCode,
		},
	})
	return toMemberDTO(member), nil
}

func (s *Service) UpdateMemberStatus(ctx context.Context, input UpdateMemberStatusInput) (MemberDTO, error) {
	if err := s.ensurePermission(ctx, input.ActorUserID, input.WorkspaceID, "workspace.manage"); err != nil {
		return MemberDTO{}, err
	}
	status := strings.TrimSpace(input.Status)
	if status != "active" && status != "disabled" && status != "left" {
		return MemberDTO{}, apperrors.BadRequest("VALIDATION_ERROR", "Trạng thái thành viên không hợp lệ.")
	}
	member, err := s.repo.UpdateMemberStatus(ctx, UpdateMemberStatusParams{
		WorkspaceID: strings.TrimSpace(input.WorkspaceID),
		UserID:      strings.TrimSpace(input.UserID),
		Status:      status,
	})
	if err != nil {
		return MemberDTO{}, mapMemberError(err)
	}
	return toMemberDTO(member), nil
}

func (s *Service) ListSettings(ctx context.Context, actorUserID string, workspaceID string) ([]SettingDTO, error) {
	if err := s.ensurePermission(ctx, actorUserID, workspaceID, "workspace.manage"); err != nil {
		return nil, err
	}
	settings, err := s.repo.ListSettings(ctx, strings.TrimSpace(workspaceID))
	if err != nil {
		return nil, err
	}
	return toSettingDTOs(settings), nil
}

func (s *Service) UpsertSetting(ctx context.Context, input UpsertSettingInput) (SettingDTO, error) {
	if err := s.ensurePermission(ctx, input.ActorUserID, input.WorkspaceID, "workspace.manage"); err != nil {
		return SettingDTO{}, err
	}
	input.Key = strings.TrimSpace(input.Key)
	if input.Key == "" || len(input.Value) == 0 || !json.Valid(input.Value) {
		return SettingDTO{}, apperrors.BadRequest("VALIDATION_ERROR", "Key hoặc value setting không hợp lệ.")
	}
	if input.ValueType == "" {
		input.ValueType = "json"
	}
	setting, err := s.repo.UpsertSetting(ctx, UpsertSettingParams{
		WorkspaceID: strings.TrimSpace(input.WorkspaceID),
		Key:         input.Key,
		Value:       input.Value,
		ValueType:   strings.TrimSpace(input.ValueType),
		Description: strings.TrimSpace(input.Description),
		UpdatedBy:   strings.TrimSpace(input.ActorUserID),
	})
	if err != nil {
		return SettingDTO{}, err
	}
	return toSettingDTO(setting), nil
}

func (s *Service) CreateInvite(ctx context.Context, input CreateInviteInput) (InviteDTO, error) {
	if err := s.ensurePermission(ctx, input.ActorUserID, input.WorkspaceID, "workspace.invite_user"); err != nil {
		return InviteDTO{}, err
	}
	email := strings.ToLower(strings.TrimSpace(input.Email))
	if _, err := mail.ParseAddress(email); err != nil {
		return InviteDTO{}, apperrors.BadRequest("VALIDATION_ERROR", "Email không đúng định dạng.")
	}
	token, tokenHash, err := newInviteToken()
	if err != nil {
		return InviteDTO{}, apperrors.Internal("Không tạo được token lời mời.")
	}
	if input.ExpiresDays <= 0 || input.ExpiresDays > 30 {
		input.ExpiresDays = 7
	}
	roleCode := strings.TrimSpace(input.RoleCode)
	if roleCode == "" {
		roleCode = "workspace_member"
	}
	invite, err := s.repo.CreateInvite(ctx, CreateInviteParams{
		WorkspaceID: strings.TrimSpace(input.WorkspaceID),
		Email:       email,
		RoleCode:    roleCode,
		TokenHash:   tokenHash,
		InvitedBy:   strings.TrimSpace(input.ActorUserID),
		ExpiresAt:   s.now().UTC().Add(time.Duration(input.ExpiresDays) * 24 * time.Hour),
	})
	if err != nil {
		return InviteDTO{}, mapMemberError(err)
	}
	dto := toInviteDTO(invite)
	dto.Token = token
	return dto, nil
}

func (s *Service) ListInvites(ctx context.Context, actorUserID string, workspaceID string) ([]InviteDTO, error) {
	if err := s.ensurePermission(ctx, actorUserID, workspaceID, "workspace.invite_user"); err != nil {
		return nil, err
	}
	invites, err := s.repo.ListInvites(ctx, strings.TrimSpace(workspaceID))
	if err != nil {
		return nil, err
	}
	dtos := make([]InviteDTO, 0, len(invites))
	for _, invite := range invites {
		dtos = append(dtos, toInviteDTO(invite))
	}
	return dtos, nil
}

func (s *Service) ensureMember(ctx context.Context, userID string, workspaceID string) error {
	allowed, err := s.checker.HasWorkspacePermission(ctx, strings.TrimSpace(userID), strings.TrimSpace(workspaceID), "workspace.view_members")
	if err != nil {
		return err
	}
	if !allowed {
		return apperrors.Forbidden("Bạn không thuộc workspace này.")
	}
	return nil
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

func mapWorkspaceError(err error) error {
	if errors.Is(err, workspacesdomain.ErrWorkspaceNotFound) {
		return apperrors.NotFound("WORKSPACE_NOT_FOUND", "Không tìm thấy workspace.")
	}
	return err
}

func mapMemberError(err error) error {
	if errors.Is(err, workspacesdomain.ErrMemberNotFound) {
		return apperrors.NotFound("WORKSPACE_MEMBER_NOT_FOUND", "Không tìm thấy thành viên workspace.")
	}
	if errors.Is(err, workspacesdomain.ErrRoleNotFound) {
		return apperrors.NotFound("WORKSPACE_ROLE_NOT_FOUND", "Không tìm thấy vai trò workspace.")
	}
	return err
}

func cleanOptional(value *string) *string {
	if value == nil {
		return nil
	}
	cleaned := strings.TrimSpace(*value)
	return &cleaned
}

func toWorkspaceDTO(workspace workspacesdomain.Workspace) WorkspaceDTO {
	return WorkspaceDTO{
		ID:          workspace.ID,
		ZoneID:      workspace.ZoneID,
		Slug:        workspace.Slug,
		Name:        workspace.Name,
		Description: workspace.Description,
		OwnerID:     workspace.OwnerID,
		Plan:        workspace.Plan,
		Status:      workspace.Status,
		CreatedAt:   formatTime(workspace.CreatedAt),
		UpdatedAt:   formatTime(workspace.UpdatedAt),
	}
}

func toMemberDTOs(members []workspacesdomain.Member) []MemberDTO {
	dtos := make([]MemberDTO, 0, len(members))
	for _, member := range members {
		dtos = append(dtos, toMemberDTO(member))
	}
	return dtos
}

func toMemberDTO(member workspacesdomain.Member) MemberDTO {
	return MemberDTO{
		ID:          member.ID,
		WorkspaceID: member.WorkspaceID,
		UserID:      member.UserID,
		Email:       member.Email,
		Username:    member.Username,
		DisplayName: member.DisplayName,
		AvatarURL:   member.AvatarURL,
		PhoneNumber: member.PhoneNumber,
		Status:      member.Status,
		Title:       member.Title,
		JoinedAt:    formatOptionalTime(member.JoinedAt),
		CreatedAt:   formatTime(member.CreatedAt),
		UpdatedAt:   formatTime(member.UpdatedAt),
	}
}

func toSettingDTOs(settings []workspacesdomain.Setting) []SettingDTO {
	dtos := make([]SettingDTO, 0, len(settings))
	for _, setting := range settings {
		dtos = append(dtos, toSettingDTO(setting))
	}
	return dtos
}

func toSettingDTO(setting workspacesdomain.Setting) SettingDTO {
	return SettingDTO{
		WorkspaceID: setting.WorkspaceID,
		Key:         setting.Key,
		Value:       json.RawMessage(setting.Value),
		ValueType:   setting.ValueType,
		Description: setting.Description,
		UpdatedBy:   setting.UpdatedBy,
		CreatedAt:   formatTime(setting.CreatedAt),
		UpdatedAt:   formatTime(setting.UpdatedAt),
	}
}

func toInviteDTO(invite workspacesdomain.Invite) InviteDTO {
	return InviteDTO{
		ID:          invite.ID,
		WorkspaceID: invite.WorkspaceID,
		Email:       invite.Email,
		RoleID:      invite.RoleID,
		InvitedBy:   invite.InvitedBy,
		ExpiresAt:   formatTime(invite.ExpiresAt),
		AcceptedAt:  formatOptionalTime(invite.AcceptedAt),
		RevokedAt:   formatOptionalTime(invite.RevokedAt),
		CreatedAt:   formatTime(invite.CreatedAt),
	}
}

func newInviteToken() (string, string, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", "", err
	}
	token := "wti_" + base64.RawURLEncoding.EncodeToString(b[:])
	hash := sha256.Sum256([]byte(token))
	return token, hex.EncodeToString(hash[:]), nil
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
