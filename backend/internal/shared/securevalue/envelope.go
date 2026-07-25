package securevalue

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strings"
)

const envelopePrefix = "enc:v1:"

var ErrInvalidEnvelope = errors.New("invalid encrypted value envelope")

func Encrypt(masterSecret string, plaintext string, aad string) (string, error) {
	if strings.TrimSpace(masterSecret) == "" {
		return "", errors.New("encryption master secret is empty")
	}
	if plaintext == "" {
		return "", errors.New("plaintext is empty")
	}
	gcm, err := newGCM(masterSecret)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("generate encrypted value nonce: %w", err)
	}
	sealed := gcm.Seal(nonce, nonce, []byte(plaintext), []byte(aad))
	return envelopePrefix + base64.RawURLEncoding.EncodeToString(sealed), nil
}

func Decrypt(masterSecret string, envelope string, aad string) (string, error) {
	if strings.TrimSpace(masterSecret) == "" {
		return "", errors.New("encryption master secret is empty")
	}
	if !strings.HasPrefix(envelope, envelopePrefix) {
		return "", ErrInvalidEnvelope
	}
	payload, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(envelope, envelopePrefix))
	if err != nil {
		return "", ErrInvalidEnvelope
	}
	gcm, err := newGCM(masterSecret)
	if err != nil {
		return "", err
	}
	if len(payload) <= gcm.NonceSize() {
		return "", ErrInvalidEnvelope
	}
	nonce := payload[:gcm.NonceSize()]
	plaintext, err := gcm.Open(nil, nonce, payload[gcm.NonceSize():], []byte(aad))
	if err != nil || len(plaintext) == 0 {
		return "", ErrInvalidEnvelope
	}
	return string(plaintext), nil
}

func newGCM(masterSecret string) (cipher.AEAD, error) {
	key := sha256.Sum256([]byte(masterSecret))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, fmt.Errorf("create encrypted value cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create encrypted value gcm: %w", err)
	}
	return gcm, nil
}
