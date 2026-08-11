package postgres

import "testing"

func TestNativeProviderPayloadOverwritesUntrustedInstanceIdentity(t *testing.T) {
	const zoneID = "3f1e32b9-0a2f-4ca1-b0dc-04221a551c1c"
	for _, test := range []struct {
		name    string
		payload map[string]any
	}{
		{name: "missing", payload: map[string]any{"event_id": "event-1", "event_type": "message"}},
		{name: "forged", payload: map[string]any{"instance_id": "attacker-instance", "event_id": "event-1", "event_type": "message"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			providerPayload, err := nativeProviderPayload(test.payload, zoneID)
			if err != nil {
				t.Fatalf("nativeProviderPayload() error = %v", err)
			}
			if providerPayload["instance_id"] != zoneID {
				t.Fatalf("provider instance_id = %#v, want persisted zone UUID", providerPayload["instance_id"])
			}
			providerPayload["event_id"] = "changed"
			if test.payload["event_id"] != "event-1" {
				t.Fatalf("nativeProviderPayload() mutated queued payload: %#v", test.payload)
			}
			if test.name == "forged" && test.payload["instance_id"] != "attacker-instance" {
				t.Fatalf("nativeProviderPayload() rewrote raw queued payload in place: %#v", test.payload)
			}
		})
	}
}

func TestNativeProviderPayloadFailsClosedWithoutZoneUUID(t *testing.T) {
	if _, err := nativeProviderPayload(map[string]any{"event_type": "message"}, "zone-1"); err == nil {
		t.Fatal("nativeProviderPayload() accepted a non-UUID destination zone")
	}
}

func TestSelectNativeDestinationsPrefersPairedVoIPForCallInvite(t *testing.T) {
	destinations := []nativeDestination{
		{id: "ios-fcm", deviceID: "ios-device", provider: "fcm", token: "ios-fcm-token"},
		{id: "ios-voip", deviceID: "ios-device:voip", provider: "apns", token: "ios-voip-token"},
		{id: "android-fcm", deviceID: "android-device", provider: "fcm", token: "android-token"},
	}

	selected := selectNativeDestinations(destinations, "call_invite", true)
	assertDestinationIDs(t, selected, "ios-voip", "android-fcm")
}

func TestSelectNativeDestinationsFallsBackToFCMWhenVoIPUnavailable(t *testing.T) {
	destinations := []nativeDestination{
		{id: "ios-fcm", deviceID: "ios-device", provider: "fcm", token: "ios-fcm-token"},
		{id: "ios-voip", deviceID: "ios-device:voip", provider: "apns", token: "ios-voip-token"},
	}

	selected := selectNativeDestinations(destinations, "call_invite", false)
	assertDestinationIDs(t, selected, "ios-fcm")
}

func TestSelectNativeDestinationsKeepsUnpairedFCMCallDestination(t *testing.T) {
	destinations := []nativeDestination{
		{id: "fcm-only", deviceID: "device-without-voip", provider: "fcm", token: "fcm-token"},
	}

	selected := selectNativeDestinations(destinations, "call_invite", true)
	assertDestinationIDs(t, selected, "fcm-only")
}

func TestSelectNativeDestinationsNeverUsesVoIPForOrdinaryNotification(t *testing.T) {
	destinations := []nativeDestination{
		{id: "ios-fcm", deviceID: "ios-device", provider: "fcm", token: "ios-fcm-token"},
		{id: "ios-voip", deviceID: "ios-device:voip", provider: "apns", token: "ios-voip-token"},
	}

	selected := selectNativeDestinations(destinations, "message_created", true)
	assertDestinationIDs(t, selected, "ios-fcm")
}

func assertDestinationIDs(t *testing.T, destinations []nativeDestination, want ...string) {
	t.Helper()
	if len(destinations) != len(want) {
		t.Fatalf("destination count = %d, want %d: %#v", len(destinations), len(want), destinations)
	}
	seen := make(map[string]bool, len(destinations))
	for _, destination := range destinations {
		seen[destination.id] = true
	}
	for _, id := range want {
		if !seen[id] {
			t.Fatalf("destination %q missing from %#v", id, destinations)
		}
	}
}
