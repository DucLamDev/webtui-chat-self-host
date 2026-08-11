package bootstrap

import "testing"

func TestValidatePushRelayInstanceIdentity(t *testing.T) {
	const zoneID = "3f1e32b9-0a2f-4ca1-b0dc-04221a551c1c"
	for _, test := range []struct {
		name       string
		configured string
		zone       string
		wantError  bool
	}{
		{name: "exact persisted zone UUID", configured: zoneID, zone: zoneID},
		{name: "normalizes surrounding whitespace", configured: "  " + zoneID + "  ", zone: zoneID},
		{name: "different instance", configured: "c51a22fc-dab7-48a7-8b59-f6f42a82877c", zone: zoneID, wantError: true},
		{name: "missing configured identity", zone: zoneID, wantError: true},
		{name: "missing persisted identity", configured: zoneID, wantError: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := validatePushRelayInstanceIdentity(test.configured, test.zone)
			if (err != nil) != test.wantError {
				t.Fatalf("validatePushRelayInstanceIdentity() error = %v, wantError = %v", err, test.wantError)
			}
		})
	}
}
