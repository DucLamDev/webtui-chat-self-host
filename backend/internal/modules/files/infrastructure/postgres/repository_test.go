package postgres

import (
	"strings"
	"testing"
)

func TestAttachFileQueryAtomicallyMarksMessageMetadata(t *testing.T) {
	query := strings.ToLower(strings.Join(strings.Fields(attachFileQuery), " "))

	for _, expected := range []string{
		"with target_message as",
		"insert into message_attachments",
		"update messages as m",
		"jsonb_set(",
		"'{has_attachments}'",
		"'true'::jsonb",
		"from attached",
	} {
		if !strings.Contains(query, expected) {
			t.Fatalf("attachFileQuery must contain %q", expected)
		}
	}

	if strings.Contains(query, "set kind =") || strings.Contains(query, "set metadata = '{") {
		t.Fatal("attachFileQuery must preserve the message kind and existing metadata")
	}
}
