package apns

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"testing"
)

func TestNewSenderFailsFastForMalformedPrivateKey(t *testing.T) {
	sender := NewSender(Config{
		KeyID:            "KEY123",
		TeamID:           "TEAM123",
		BundleID:         "com.example.chat",
		PrivateKeyBase64: "not-base64",
	})
	if sender.InitializationError() == nil || sender.Enabled() {
		t.Fatal("malformed APNs credential must disable sender with an initialization error")
	}
}

func TestNewSenderAcceptsValidP256PrivateKey(t *testing.T) {
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate P-256 key: %v", err)
	}
	encodedKey, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		t.Fatalf("marshal P-256 key: %v", err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: encodedKey})
	sender := NewSender(Config{
		KeyID:            "KEY123",
		TeamID:           "TEAM123",
		BundleID:         "com.example.chat",
		PrivateKeyBase64: base64.StdEncoding.EncodeToString(keyPEM),
	})
	if err := sender.InitializationError(); err != nil {
		t.Fatalf("InitializationError() = %v", err)
	}
	if !sender.Enabled() {
		t.Fatal("valid APNs credential should enable sender")
	}
}
