package application

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	tenancydomain "github.com/duclamdev/application-chat/backend/internal/modules/tenancy/domain"
	"github.com/duclamdev/application-chat/backend/internal/shared/securevalue"
)

type fakeZoneStorageTester struct {
	config ZoneStorageConnectionConfig
	err    error
}

func (t *fakeZoneStorageTester) Test(_ context.Context, config ZoneStorageConnectionConfig) error {
	t.config = config
	return t.err
}

func TestUpdateZoneStorageSettingsEncryptsAndTestsCredentials(t *testing.T) {
	const (
		zoneID       = "13b348f5-cdea-42fd-ac80-2c6219616f87"
		masterSecret = "a-production-webhook-secret-with-enough-entropy"
		secret       = "minio-customer-secret"
	)
	repo := &fakeRepo{}
	tester := &fakeZoneStorageTester{}
	service := NewService(repo, Options{WebhookSigningSecret: masterSecret})
	service.SetZoneStorageTester(tester)

	settings, err := service.UpdateZoneStorageSettings(context.Background(), UpdateZoneStorageInput{
		ActorUserID: "b8619ef7-7ef1-4b72-bc7d-dc8e08017af9",
		ZoneID:      zoneID, Provider: "minio", Endpoint: "https://minio.example.test:9000/",
		Region: "vn-1", Bucket: "host-media-01", AccessKeyID: "host-01", SecretAccessKey: secret,
	})
	if err != nil {
		t.Fatalf("UpdateZoneStorageSettings() error = %v", err)
	}
	if !settings.Configured || !settings.HasSecretAccessKey || settings.Provider != "minio" {
		t.Fatalf("settings = %#v", settings)
	}
	encoded, err := json.Marshal(settings)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), secret) || strings.Contains(string(encoded), repo.storageParams.SecretAccessKeyEncrypted) {
		t.Fatalf("public settings leaked storage credentials: %s", encoded)
	}
	if tester.config.SecretAccessKey != secret || tester.config.Endpoint != "https://minio.example.test:9000" {
		t.Fatalf("tested config = %#v", tester.config)
	}
	if repo.storageParams.SecretAccessKeyEncrypted == "" || strings.Contains(repo.storageParams.SecretAccessKeyEncrypted, secret) {
		t.Fatalf("stored secret envelope = %q", repo.storageParams.SecretAccessKeyEncrypted)
	}
	plaintext, err := securevalue.Decrypt(masterSecret, repo.storageParams.SecretAccessKeyEncrypted, zoneStorageSecretAAD(zoneID))
	if err != nil || plaintext != secret {
		t.Fatalf("decrypt stored secret = %q, %v", plaintext, err)
	}
}

func TestUpdateZoneStorageSettingsKeepsExistingSecretWhenInputIsBlank(t *testing.T) {
	const (
		zoneID       = "13b348f5-cdea-42fd-ac80-2c6219616f87"
		masterSecret = "a-production-webhook-secret-with-enough-entropy"
		secret       = "existing-minio-secret"
	)
	envelope, err := securevalue.Encrypt(masterSecret, secret, zoneStorageSecretAAD(zoneID))
	if err != nil {
		t.Fatal(err)
	}
	repo := &fakeRepo{storageConfig: tenancydomain.ZoneStorageConfig{
		ZoneID: zoneID, Provider: "minio", SecretAccessKeyEncrypted: envelope,
	}}
	tester := &fakeZoneStorageTester{}
	service := NewService(repo, Options{WebhookSigningSecret: masterSecret})
	service.SetZoneStorageTester(tester)

	_, err = service.UpdateZoneStorageSettings(context.Background(), UpdateZoneStorageInput{
		ActorUserID: "b8619ef7-7ef1-4b72-bc7d-dc8e08017af9",
		ZoneID:      zoneID, Provider: "s3", Endpoint: "https://s3.example.test",
		Region: "ap-southeast-1", Bucket: "host-media-02", AccessKeyID: "host-02",
	})
	if err != nil {
		t.Fatalf("UpdateZoneStorageSettings() error = %v", err)
	}
	if tester.config.SecretAccessKey != secret {
		t.Fatalf("tester secret = %q", tester.config.SecretAccessKey)
	}
}

func TestUpdateZoneStorageSettingsRejectsInvalidBucketBeforeConnection(t *testing.T) {
	tester := &fakeZoneStorageTester{}
	service := NewService(&fakeRepo{}, Options{WebhookSigningSecret: "a-production-webhook-secret-with-enough-entropy"})
	service.SetZoneStorageTester(tester)

	_, err := service.UpdateZoneStorageSettings(context.Background(), UpdateZoneStorageInput{
		ActorUserID: "b8619ef7-7ef1-4b72-bc7d-dc8e08017af9",
		ZoneID:      "13b348f5-cdea-42fd-ac80-2c6219616f87", Provider: "minio",
		Endpoint: "https://minio.example.test", Bucket: "bucket-user@example.com",
		AccessKeyID: "host-01", SecretAccessKey: "secret",
	})
	if err == nil {
		t.Fatal("UpdateZoneStorageSettings() expected validation error")
	}
	if tester.config.Provider != "" {
		t.Fatalf("connection tester should not run: %#v", tester.config)
	}
}
