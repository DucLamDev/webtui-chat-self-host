package security

import (
	"errors"

	"github.com/duclamdev/application-chat/backend/internal/shared/securevalue"
)

const (
	envelopePrefix = "enc:v1:"
	envelopeAAD    = "vpsttt-chat:outgoing-webhook:v1"
)

var ErrInvalidEnvelope = errors.New("invalid outgoing webhook secret envelope")

func EncryptSecret(masterSecret string, secret string) (string, error) {
	return securevalue.Encrypt(masterSecret, secret, envelopeAAD)
}

func DecryptSecret(masterSecret string, envelope string) (string, error) {
	plaintext, err := securevalue.Decrypt(masterSecret, envelope, envelopeAAD)
	if errors.Is(err, securevalue.ErrInvalidEnvelope) {
		return "", ErrInvalidEnvelope
	}
	return plaintext, err
}
