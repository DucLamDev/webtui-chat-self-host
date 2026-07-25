package postgres

import "testing"

func TestSelfHostedSlug(t *testing.T) {
	tests := map[string]string{
		"chat.company.example": "chat-company-example",
		"  CHAT.EXAMPLE.COM  ": "chat-example-com",
		"":                     "instance",
	}
	for input, expected := range tests {
		if got := selfHostedSlug(input); got != expected {
			t.Fatalf("selfHostedSlug(%q) = %q, want %q", input, got, expected)
		}
	}
}

func TestSelfHostedSlugLimitsLength(t *testing.T) {
	got := selfHostedSlug("this-is-a-very-long-customer-subdomain-that-exceeds-the-zone-slug-limit.example.com")
	if len(got) > 63 {
		t.Fatalf("selfHostedSlug() length = %d, want <= 63", len(got))
	}
}
