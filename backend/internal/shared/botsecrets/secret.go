package botsecrets

import (
	"strings"

	"github.com/duclamdev/application-chat/backend/internal/shared/securevalue"
)

const (
	maskedReference = "vault://configured"
	referencePrefix = "vault://"
	envelopeAAD     = "vpsttt-chat:bot-ai-secret:v1"
)

func Encrypt(masterSecret string, apiKey string) (string, error) {
	envelope, err := securevalue.Encrypt(masterSecret, strings.TrimSpace(apiKey), envelopeAAD)
	if err != nil {
		return "", err
	}
	return referencePrefix + envelope, nil
}

func Decrypt(masterSecret string, reference string) (string, error) {
	return securevalue.Decrypt(masterSecret, strings.TrimPrefix(reference, referencePrefix), envelopeAAD)
}

func IsEncrypted(reference string) bool {
	return strings.HasPrefix(strings.TrimSpace(reference), referencePrefix+"enc:v1:")
}

func IsMasked(reference string) bool {
	return strings.TrimSpace(reference) == maskedReference
}

func MaskedReference() string {
	return maskedReference
}
