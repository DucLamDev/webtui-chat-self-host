package fcm

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"testing"
)

func TestNewSenderFailsFastForMalformedCredentials(t *testing.T) {
	sender := NewSender(Config{
		ProjectID:                "project-1",
		ServiceAccountJSONBase64: "not-base64",
	})
	if sender.InitializationError() == nil || sender.Enabled() {
		t.Fatal("malformed Firebase credential must disable sender with an initialization error")
	}
}

func TestNewSenderValidatesPrivateKeyAtInitialization(t *testing.T) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		t.Fatalf("generate RSA key: %v", err)
	}
	encodedKey, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		t.Fatalf("marshal RSA key: %v", err)
	}
	raw, err := json.Marshal(serviceAccount{
		ClientEmail: "push@example.iam.gserviceaccount.com",
		PrivateKey: string(pem.EncodeToMemory(&pem.Block{
			Type:  "PRIVATE KEY",
			Bytes: encodedKey,
		})),
		ProjectID: "project-1",
	})
	if err != nil {
		t.Fatalf("marshal service account: %v", err)
	}
	sender := NewSender(Config{ServiceAccountJSONBase64: base64.StdEncoding.EncodeToString(raw)})
	if err := sender.InitializationError(); err != nil {
		t.Fatalf("InitializationError() = %v", err)
	}
	if !sender.Enabled() {
		t.Fatal("valid Firebase credential should enable sender")
	}
}

func TestBuildMessageUsesHighPriorityCallDataOnlyPayload(t *testing.T) {
	message := buildMessage("device-token", map[string]any{
		"event_type": "call_invite",
		"title":      "Incoming call",
		"body":       "Lam is calling",
		"call_id":    "call-1",
		"tag":        "call-call-1",
	})

	if message["token"] != "device-token" {
		t.Fatalf("token = %#v", message["token"])
	}
	android, ok := message["android"].(map[string]any)
	if !ok || android["priority"] != "high" || android["collapse_key"] != "call-call-1" {
		t.Fatalf("android config = %#v", message["android"])
	}
	if _, exists := android["notification"]; exists {
		t.Fatal("call invite must not include android notification config")
	}
	data, ok := message["data"].(map[string]string)
	if !ok || data["event_type"] != "call_invite" || data["call_id"] != "call-1" {
		t.Fatalf("data = %#v", message["data"])
	}
	if _, exists := message["notification"]; exists {
		t.Fatal("call invite must stay data-only so mobile background handlers can show CallKit")
	}
}

func TestBuildMessageKeepsMessagePayload(t *testing.T) {
	message := buildMessage("device-token", map[string]any{
		"event_type": "message",
		"message_id": "message-1",
		"unread":     2,
	})
	data := message["data"].(map[string]string)
	if data["message_id"] != "message-1" || data["unread"] != "2" {
		t.Fatalf("data = %#v", data)
	}
	if _, exists := message["notification"]; exists {
		t.Fatal("notification must be omitted when title and body are empty")
	}
}

func TestPermanentDeliveryErrorRequiresDeviceSpecificProviderCode(t *testing.T) {
	tests := []struct {
		name string
		body string
		want bool
	}{
		{name: "unregistered token", body: `{"error":{"details":[{"errorCode":"UNREGISTERED"}]}}`, want: true},
		{name: "sender mismatch", body: `{"error":{"details":[{"errorCode":"SENDER_ID_MISMATCH"}]}}`, want: true},
		{name: "project not found", body: `{"error":{"status":"NOT_FOUND","message":"Requested entity was not found."}}`, want: false},
		{name: "temporary unavailable", body: `{"error":{"status":"UNAVAILABLE"}}`, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isPermanentDeliveryError([]byte(tt.body)); got != tt.want {
				t.Fatalf("isPermanentDeliveryError() = %v, want %v", got, tt.want)
			}
		})
	}
}
