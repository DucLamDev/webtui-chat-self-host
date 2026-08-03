package application

import (
	"context"
	"errors"
	"net/url"
	"regexp"
	"strings"
	"time"

	tenancydomain "github.com/duclamdev/application-chat/backend/internal/modules/tenancy/domain"
	apperrors "github.com/duclamdev/application-chat/backend/internal/shared/errors"
	"github.com/duclamdev/application-chat/backend/internal/shared/securevalue"
)

var s3BucketPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9.-]{1,61}[a-z0-9]$`)

type ZoneStorageTester interface {
	Test(ctx context.Context, config ZoneStorageConnectionConfig) error
}

type ZoneStorageConnectionConfig struct {
	Provider        string
	Endpoint        string
	Region          string
	Bucket          string
	AccessKeyID     string
	SecretAccessKey string
}

type UpdateZoneStorageInput struct {
	ActorUserID     string
	ZoneID          string
	Provider        string
	Endpoint        string
	Region          string
	Bucket          string
	AccessKeyID     string
	SecretAccessKey string
}

type UpsertZoneStorageConfigParams struct {
	ActorUserID              string
	ZoneID                   string
	Provider                 string
	Endpoint                 string
	Region                   string
	Bucket                   string
	AccessKeyID              string
	SecretAccessKeyEncrypted string
}

type ZoneStorageSettingsDTO struct {
	Provider           string  `json:"provider"`
	Endpoint           string  `json:"endpoint,omitempty"`
	Region             string  `json:"region,omitempty"`
	Bucket             string  `json:"bucket,omitempty"`
	AccessKeyID        string  `json:"access_key_id,omitempty"`
	HasSecretAccessKey bool    `json:"has_secret_access_key"`
	Configured         bool    `json:"configured"`
	UpdatedAt          *string `json:"updated_at,omitempty"`
}

func (s *Service) SetZoneStorageTester(tester ZoneStorageTester) {
	s.storageTester = tester
}

func (s *Service) GetZoneStorageSettings(
	ctx context.Context,
	actorUserID string,
	zoneID string,
) (ZoneStorageSettingsDTO, error) {
	actorUserID = strings.TrimSpace(actorUserID)
	zoneID = strings.TrimSpace(zoneID)
	if err := s.ensureZoneManager(ctx, actorUserID, zoneID); err != nil {
		return ZoneStorageSettingsDTO{}, err
	}
	config, err := s.repo.GetZoneStorageConfig(ctx, zoneID)
	if errors.Is(err, tenancydomain.ErrZoneStorageConfigNotFound) {
		return ZoneStorageSettingsDTO{Provider: "local", Configured: true}, nil
	}
	if err != nil {
		return ZoneStorageSettingsDTO{}, mapClaimError(err)
	}
	return toZoneStorageSettingsDTO(config), nil
}

func (s *Service) UpdateZoneStorageSettings(
	ctx context.Context,
	input UpdateZoneStorageInput,
) (ZoneStorageSettingsDTO, error) {
	input.ActorUserID = strings.TrimSpace(input.ActorUserID)
	input.ZoneID = strings.TrimSpace(input.ZoneID)
	if err := s.ensureZoneManager(ctx, input.ActorUserID, input.ZoneID); err != nil {
		return ZoneStorageSettingsDTO{}, err
	}

	config, err := normalizeZoneStorageInput(input)
	if err != nil {
		return ZoneStorageSettingsDTO{}, err
	}

	existing, existingErr := s.repo.GetZoneStorageConfig(ctx, input.ZoneID)
	if existingErr != nil && !errors.Is(existingErr, tenancydomain.ErrZoneStorageConfigNotFound) {
		return ZoneStorageSettingsDTO{}, mapClaimError(existingErr)
	}

	secretEncrypted := ""
	if config.Provider != "local" {
		if config.SecretAccessKey == "" && existingErr == nil && existing.SecretAccessKeyEncrypted != "" {
			config.SecretAccessKey, err = securevalue.Decrypt(
				s.options.StorageCredentialsKey,
				existing.SecretAccessKeyEncrypted,
				zoneStorageSecretAAD(input.ZoneID),
			)
			if err != nil {
				return ZoneStorageSettingsDTO{}, apperrors.Internal("Không giải mã được Secret Key S3 đã lưu.")
			}
		}
		if config.SecretAccessKey == "" {
			return ZoneStorageSettingsDTO{}, apperrors.BadRequest(
				"VALIDATION_ERROR",
				"Secret Key là bắt buộc khi cấu hình MinIO hoặc S3.",
			)
		}
		if s.storageTester != nil {
			if err := s.storageTester.Test(ctx, config); err != nil {
				return ZoneStorageSettingsDTO{}, apperrors.BadRequest(
					"STORAGE_CONNECTION_FAILED",
					"Không kết nối được bucket. Hãy kiểm tra endpoint, region, bucket, Access Key và Secret Key.",
				)
			}
		}
		secretEncrypted, err = securevalue.Encrypt(
			s.options.StorageCredentialsKey,
			config.SecretAccessKey,
			zoneStorageSecretAAD(input.ZoneID),
		)
		if err != nil {
			return ZoneStorageSettingsDTO{}, apperrors.Internal("Máy chủ chưa sẵn sàng mã hóa thông tin đăng nhập S3.")
		}
	}

	stored, err := s.repo.UpsertZoneStorageConfig(ctx, UpsertZoneStorageConfigParams{
		ActorUserID:              input.ActorUserID,
		ZoneID:                   input.ZoneID,
		Provider:                 config.Provider,
		Endpoint:                 config.Endpoint,
		Region:                   config.Region,
		Bucket:                   config.Bucket,
		AccessKeyID:              config.AccessKeyID,
		SecretAccessKeyEncrypted: secretEncrypted,
	})
	if err != nil {
		return ZoneStorageSettingsDTO{}, mapClaimError(err)
	}
	return toZoneStorageSettingsDTO(stored), nil
}

func normalizeZoneStorageInput(input UpdateZoneStorageInput) (ZoneStorageConnectionConfig, error) {
	config := ZoneStorageConnectionConfig{
		Provider:        strings.ToLower(strings.TrimSpace(input.Provider)),
		Endpoint:        strings.TrimRight(strings.TrimSpace(input.Endpoint), "/"),
		Region:          strings.TrimSpace(input.Region),
		Bucket:          strings.ToLower(strings.TrimSpace(input.Bucket)),
		AccessKeyID:     strings.TrimSpace(input.AccessKeyID),
		SecretAccessKey: strings.TrimSpace(input.SecretAccessKey),
	}
	if config.Provider == "" {
		config.Provider = "local"
	}
	if config.Provider == "local" {
		return ZoneStorageConnectionConfig{Provider: "local"}, nil
	}
	if config.Provider != "minio" && config.Provider != "s3" {
		return ZoneStorageConnectionConfig{}, apperrors.BadRequest("VALIDATION_ERROR", "Nhà cung cấp storage không hợp lệ.")
	}
	endpoint, err := url.Parse(config.Endpoint)
	if err != nil || (endpoint.Scheme != "http" && endpoint.Scheme != "https") || endpoint.Host == "" ||
		endpoint.User != nil || endpoint.RawQuery != "" || endpoint.Fragment != "" {
		return ZoneStorageConnectionConfig{}, apperrors.BadRequest(
			"VALIDATION_ERROR",
			"Endpoint phải là URL HTTP/HTTPS hợp lệ và không chứa thông tin đăng nhập, query hoặc fragment.",
		)
	}
	if config.Region == "" {
		config.Region = "us-east-1"
	}
	if len(config.Region) > 100 {
		return ZoneStorageConnectionConfig{}, apperrors.BadRequest("VALIDATION_ERROR", "Region không hợp lệ.")
	}
	if !s3BucketPattern.MatchString(config.Bucket) || strings.Contains(config.Bucket, "..") {
		return ZoneStorageConnectionConfig{}, apperrors.BadRequest(
			"VALIDATION_ERROR",
			"Tên bucket phải dài 3–63 ký tự, chỉ gồm chữ thường, số, dấu chấm hoặc gạch ngang.",
		)
	}
	if config.AccessKeyID == "" || len(config.AccessKeyID) > 256 {
		return ZoneStorageConnectionConfig{}, apperrors.BadRequest("VALIDATION_ERROR", "Access Key không hợp lệ.")
	}
	if len(config.SecretAccessKey) > 1024 {
		return ZoneStorageConnectionConfig{}, apperrors.BadRequest("VALIDATION_ERROR", "Secret Key không hợp lệ.")
	}
	return config, nil
}

func zoneStorageSecretAAD(zoneID string) string {
	return "vpsttt:zone-storage:" + strings.TrimSpace(zoneID)
}

func toZoneStorageSettingsDTO(config tenancydomain.ZoneStorageConfig) ZoneStorageSettingsDTO {
	var updatedAt *string
	if !config.UpdatedAt.IsZero() {
		formatted := config.UpdatedAt.UTC().Format(time.RFC3339)
		updatedAt = &formatted
	}
	return ZoneStorageSettingsDTO{
		Provider:           config.Provider,
		Endpoint:           config.Endpoint,
		Region:             config.Region,
		Bucket:             config.Bucket,
		AccessKeyID:        config.AccessKeyID,
		HasSecretAccessKey: config.SecretAccessKeyEncrypted != "",
		Configured:         config.Provider == "local" || config.SecretAccessKeyEncrypted != "",
		UpdatedAt:          updatedAt,
	}
}
