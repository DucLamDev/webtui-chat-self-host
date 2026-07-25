package security

import (
	"errors"
	"strings"
	"testing"
)

func TestSecretEnvelopeRoundTrip(t *testing.T) {
	const masterSecret = "test-master-secret-with-at-least-32-characters"
	const signingSecret = "wtow_customer-visible-secret"

	envelope, err := EncryptSecret(masterSecret, signingSecret)
	if err != nil {
		t.Fatalf("EncryptSecret() error = %v", err)
	}
	if !strings.HasPrefix(envelope, envelopePrefix) || strings.Contains(envelope, signingSecret) {
		t.Fatalf("EncryptSecret() returned an invalid envelope: %q", envelope)
	}

	decrypted, err := DecryptSecret(masterSecret, envelope)
	if err != nil {
		t.Fatalf("DecryptSecret() error = %v", err)
	}
	if decrypted != signingSecret {
		t.Fatalf("DecryptSecret() = %q, want %q", decrypted, signingSecret)
	}
}

func TestSecretEnvelopeRejectsWrongKeyAndLegacyValue(t *testing.T) {
	envelope, err := EncryptSecret("correct-master-secret", "wtow_secret")
	if err != nil {
		t.Fatalf("EncryptSecret() error = %v", err)
	}
	if _, err := DecryptSecret("wrong-master-secret", envelope); !errors.Is(err, ErrInvalidEnvelope) {
		t.Fatalf("DecryptSecret() wrong key error = %v, want ErrInvalidEnvelope", err)
	}
	if _, err := DecryptSecret("correct-master-secret", "legacy:v0:rotation-required"); !errors.Is(err, ErrInvalidEnvelope) {
		t.Fatalf("DecryptSecret() legacy error = %v, want ErrInvalidEnvelope", err)
	}
}
