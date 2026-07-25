package application

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	aptokensdomain "github.com/duclamdev/application-chat/backend/internal/modules/api_tokens/domain"
	apperrors "github.com/duclamdev/application-chat/backend/internal/shared/errors"
)

type PermissionChecker interface {
	HasWorkspacePermission(ctx context.Context, userID string, workspaceID string, permissionCode string) (bool, error)
}

type Repository interface {
	ListScopes(ctx context.Context) ([]aptokensdomain.Scope, error)
	ListTokens(ctx context.Context, workspaceID string) ([]aptokensdomain.Token, error)
	CreateToken(ctx context.Context, params CreateTokenParams) (aptokensdomain.Token, error)
	RevokeToken(ctx context.Context, workspaceID string, tokenID string) error
	Authenticate(ctx context.Context, tokenHash string) (aptokensdomain.AuthenticatedToken, error)
}

type Service struct {
	repo    Repository
	checker PermissionChecker
}

type CreateTokenInput struct {
	ActorUserID string
	WorkspaceID string
	Name        string
	ScopeCodes  []string
	ExpiresDays int
}

type CreateTokenParams struct {
	WorkspaceID string
	OwnerID     string
	Name        string
	TokenHash   string
	ScopeCodes  []string
	ExpiresAt   *time.Time
}

type TokenDTO struct {
	ID          string     `json:"id"`
	WorkspaceID string     `json:"workspace_id"`
	OwnerID     *string    `json:"owner_id,omitempty"`
	Name        string     `json:"name"`
	Status      string     `json:"status"`
	LastUsedAt  *string    `json:"last_used_at,omitempty"`
	ExpiresAt   *string    `json:"expires_at,omitempty"`
	CreatedAt   string     `json:"created_at"`
	UpdatedAt   string     `json:"updated_at"`
	RevokedAt   *string    `json:"revoked_at,omitempty"`
	Scopes      []ScopeDTO `json:"scopes"`
}

type CreatedTokenDTO struct {
	TokenDTO
	Token string `json:"token"`
}

type ScopeDTO struct {
	ID          string  `json:"id"`
	Code        string  `json:"code"`
	Name        string  `json:"name"`
	Description *string `json:"description,omitempty"`
	Module      string  `json:"module"`
	Action      string  `json:"action"`
}

type AuthenticatedTokenDTO struct {
	TokenID     string
	ZoneID      string
	WorkspaceID string
	OwnerID     *string
	Scopes      []string
}

func NewService(repo Repository, checker PermissionChecker) *Service {
	return &Service{repo: repo, checker: checker}
}

func (s *Service) ListScopes(ctx context.Context) ([]ScopeDTO, error) {
	scopes, err := s.repo.ListScopes(ctx)
	if err != nil {
		return nil, err
	}
	return toScopeDTOs(scopes), nil
}

func (s *Service) ListTokens(ctx context.Context, actorUserID string, workspaceID string) ([]TokenDTO, error) {
	if err := s.ensureManagePermission(ctx, actorUserID, workspaceID); err != nil {
		return nil, err
	}
	tokens, err := s.repo.ListTokens(ctx, strings.TrimSpace(workspaceID))
	if err != nil {
		return nil, err
	}
	return toTokenDTOs(tokens), nil
}

func (s *Service) CreateToken(ctx context.Context, input CreateTokenInput) (CreatedTokenDTO, error) {
	if err := s.ensureManagePermission(ctx, input.ActorUserID, input.WorkspaceID); err != nil {
		return CreatedTokenDTO{}, err
	}
	name := strings.TrimSpace(input.Name)
	if name == "" || len([]rune(name)) > 120 {
		return CreatedTokenDTO{}, apperrors.BadRequest("VALIDATION_ERROR", "Tên API token phải dài từ 1 đến 120 ký tự.")
	}
	scopeCodes := normalizeScopeCodes(input.ScopeCodes)
	if len(scopeCodes) == 0 {
		return CreatedTokenDTO{}, apperrors.BadRequest("VALIDATION_ERROR", "API token cần ít nhất một scope.")
	}
	token, tokenHash, err := newAPIToken()
	if err != nil {
		return CreatedTokenDTO{}, apperrors.Internal("Không tạo được API token.")
	}
	var expiresAt *time.Time
	if input.ExpiresDays > 0 {
		value := time.Now().UTC().Add(time.Duration(input.ExpiresDays) * 24 * time.Hour)
		expiresAt = &value
	}
	created, err := s.repo.CreateToken(ctx, CreateTokenParams{
		WorkspaceID: strings.TrimSpace(input.WorkspaceID),
		OwnerID:     strings.TrimSpace(input.ActorUserID),
		Name:        name,
		TokenHash:   tokenHash,
		ScopeCodes:  scopeCodes,
		ExpiresAt:   expiresAt,
	})
	if err != nil {
		return CreatedTokenDTO{}, mapTokenError(err)
	}
	dto := CreatedTokenDTO{TokenDTO: toTokenDTO(created), Token: token}
	return dto, nil
}

func (s *Service) RevokeToken(ctx context.Context, actorUserID string, workspaceID string, tokenID string) error {
	if err := s.ensureManagePermission(ctx, actorUserID, workspaceID); err != nil {
		return err
	}
	if err := s.repo.RevokeToken(ctx, strings.TrimSpace(workspaceID), strings.TrimSpace(tokenID)); err != nil {
		return mapTokenError(err)
	}
	return nil
}

func (s *Service) Authenticate(ctx context.Context, token string, requiredScope string) (AuthenticatedTokenDTO, error) {
	token = strings.TrimSpace(strings.TrimPrefix(token, "Bearer "))
	if token == "" {
		return AuthenticatedTokenDTO{}, apperrors.Unauthorized("Thiếu API token.")
	}
	authenticated, err := s.repo.Authenticate(ctx, hashToken(token))
	if err != nil {
		return AuthenticatedTokenDTO{}, mapTokenError(err)
	}
	if !hasScope(authenticated.Scopes, requiredScope) {
		return AuthenticatedTokenDTO{}, apperrors.Forbidden("API token không có scope phù hợp.")
	}
	return AuthenticatedTokenDTO{
		TokenID:     authenticated.TokenID,
		ZoneID:      authenticated.ZoneID,
		WorkspaceID: authenticated.WorkspaceID,
		OwnerID:     authenticated.OwnerID,
		Scopes:      authenticated.Scopes,
	}, nil
}

func (s *Service) ensureManagePermission(ctx context.Context, userID string, workspaceID string) error {
	allowed, err := s.checker.HasWorkspacePermission(ctx, strings.TrimSpace(userID), strings.TrimSpace(workspaceID), "api_token.manage")
	if err != nil {
		return err
	}
	if !allowed {
		return apperrors.Forbidden("Bạn không có quyền quản lý API token.")
	}
	return nil
}

func normalizeScopeCodes(codes []string) []string {
	seen := map[string]bool{}
	result := make([]string, 0, len(codes))
	for _, code := range codes {
		code = strings.ToLower(strings.TrimSpace(code))
		if code == "" || seen[code] {
			continue
		}
		seen[code] = true
		result = append(result, code)
	}
	return result
}

func hasScope(scopes []string, requiredScope string) bool {
	requiredScope = strings.ToLower(strings.TrimSpace(requiredScope))
	for _, scope := range scopes {
		if strings.EqualFold(scope, requiredScope) {
			return true
		}
	}
	return false
}

func newAPIToken() (string, string, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", "", err
	}
	token := "wta_" + base64.RawURLEncoding.EncodeToString(b[:])
	return token, hashToken(token), nil
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func mapTokenError(err error) error {
	if errors.Is(err, aptokensdomain.ErrTokenNotFound) {
		return apperrors.NotFound("API_TOKEN_NOT_FOUND", "Không tìm thấy API token.")
	}
	if errors.Is(err, aptokensdomain.ErrScopeNotFound) {
		return apperrors.BadRequest("API_SCOPE_NOT_FOUND", "Một hoặc nhiều API scope không tồn tại.")
	}
	if errors.Is(err, aptokensdomain.ErrTokenInactive) {
		return apperrors.Unauthorized("API token không còn hoạt động.")
	}
	return err
}

func toTokenDTOs(tokens []aptokensdomain.Token) []TokenDTO {
	dtos := make([]TokenDTO, 0, len(tokens))
	for _, token := range tokens {
		dtos = append(dtos, toTokenDTO(token))
	}
	return dtos
}

func toTokenDTO(token aptokensdomain.Token) TokenDTO {
	return TokenDTO{
		ID:          token.ID,
		WorkspaceID: token.WorkspaceID,
		OwnerID:     token.OwnerID,
		Name:        token.Name,
		Status:      token.Status,
		LastUsedAt:  formatOptionalTime(token.LastUsedAt),
		ExpiresAt:   formatOptionalTime(token.ExpiresAt),
		CreatedAt:   formatTime(token.CreatedAt),
		UpdatedAt:   formatTime(token.UpdatedAt),
		RevokedAt:   formatOptionalTime(token.RevokedAt),
		Scopes:      toScopeDTOs(token.Scopes),
	}
}

func toScopeDTOs(scopes []aptokensdomain.Scope) []ScopeDTO {
	dtos := make([]ScopeDTO, 0, len(scopes))
	for _, scope := range scopes {
		dtos = append(dtos, ScopeDTO{
			ID:          scope.ID,
			Code:        scope.Code,
			Name:        scope.Name,
			Description: scope.Description,
			Module:      scope.Module,
			Action:      scope.Action,
		})
	}
	return dtos
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
