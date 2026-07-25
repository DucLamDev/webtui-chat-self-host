package fcm

import "testing"

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
