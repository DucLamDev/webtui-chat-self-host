package outboundhttp

import (
	"net/netip"
	"testing"
)

func TestPublicIPPolicy(t *testing.T) {
	tests := []struct {
		address string
		allowed bool
	}{
		{address: "8.8.8.8", allowed: true},
		{address: "2606:4700:4700::1111", allowed: true},
		{address: "127.0.0.1", allowed: false},
		{address: "10.0.0.1", allowed: false},
		{address: "169.254.169.254", allowed: false},
		{address: "100.64.0.1", allowed: false},
		{address: "198.51.100.20", allowed: false},
		{address: "::1", allowed: false},
		{address: "fd00::1", allowed: false},
		{address: "2001:db8::1", allowed: false},
	}
	for _, tt := range tests {
		t.Run(tt.address, func(t *testing.T) {
			if actual := IsPublicIP(netip.MustParseAddr(tt.address)); actual != tt.allowed {
				t.Fatalf("IsPublicIP(%s) = %t, want %t", tt.address, actual, tt.allowed)
			}
		})
	}
}
