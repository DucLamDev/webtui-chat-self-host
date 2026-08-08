package postgres

import "testing"

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
