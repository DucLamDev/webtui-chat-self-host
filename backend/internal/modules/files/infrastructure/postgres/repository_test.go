package postgres

import (
	"strings"
	"testing"

	filesapp "github.com/duclamdev/application-chat/backend/internal/modules/files/application"
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

func TestAttachFileCapacityIsSerializedAndBounded(t *testing.T) {
	lockQuery := strings.ToLower(strings.Join(strings.Fields(lockAttachmentTargetQuery), " "))
	capacityQuery := strings.ToLower(strings.Join(strings.Fields(attachmentCapacityQuery), " "))
	if !strings.Contains(lockQuery, "for update of m") {
		t.Fatal("attachment capacity must be serialized on the target message")
	}
	for _, expected := range []string{"count(*)", "existing.file_id", "message_attachments"} {
		if !strings.Contains(capacityQuery, expected) {
			t.Fatalf("attachmentCapacityQuery must contain %q", expected)
		}
	}

	if attachmentLimitExceeded(false, filesapp.MaxAttachmentsPerMessage-1) {
		t.Fatal("a new attachment below the cap must remain allowed")
	}
	if !attachmentLimitExceeded(false, filesapp.MaxAttachmentsPerMessage) {
		t.Fatal("a new attachment at the cap must be rejected")
	}
	if attachmentLimitExceeded(true, filesapp.MaxAttachmentsPerMessage) {
		t.Fatal("an idempotent re-attach must remain allowed at the cap")
	}
}

func TestAttachFileRequiresMessageAndFileOwnership(t *testing.T) {
	for name, rawQuery := range map[string]string{
		"lock":   lockAttachmentTargetQuery,
		"attach": attachFileQuery,
	} {
		query := strings.ToLower(strings.Join(strings.Fields(rawQuery), " "))
		for _, expected := range []string{
			"m.sender_id = $5::uuid",
			"f.owner_id = $5::uuid",
		} {
			if !strings.Contains(query, expected) {
				t.Fatalf("%s query must require actor ownership with %q", name, expected)
			}
		}
	}
}
