package pagination

import "testing"

func TestNormalizeLimit(t *testing.T) {
	tests := []struct {
		name string
		in   int
		want int
	}{
		{name: "default when zero", in: 0, want: DefaultLimit},
		{name: "default when negative", in: -1, want: DefaultLimit},
		{name: "keep valid", in: 25, want: 25},
		{name: "cap max", in: 999, want: MaxLimit},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NormalizeLimit(tt.in); got != tt.want {
				t.Fatalf("NormalizeLimit(%d) = %d, want %d", tt.in, got, tt.want)
			}
		})
	}
}
