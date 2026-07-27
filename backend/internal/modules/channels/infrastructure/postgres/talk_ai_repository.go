package postgres

import (
	"context"
	"time"

	channelsapp "github.com/duclamdev/application-chat/backend/internal/modules/channels/application"
)

func (r *Repository) ListMessagesForSummary(
	ctx context.Context,
	workspaceID string,
	channelID string,
	since *time.Time,
	limit int,
) ([]channelsapp.TalkAISummaryMessage, error) {
	if limit < 1 || limit > 500 {
		limit = 500
	}
	rows, err := r.pool.Query(ctx, `
SELECT COALESCE(NULLIF(sender.display_name, ''), sender.username, 'Unknown'),
       message.body,
       message.created_at
FROM messages message
LEFT JOIN users sender ON sender.id = message.sender_id
WHERE message.workspace_id = $1::uuid
  AND message.channel_id = $2::uuid
  AND message.deleted_at IS NULL
  AND message.kind IN ('text', 'event')
  AND ($3::timestamptz IS NULL OR message.created_at >= $3::timestamptz)
  AND length(trim(message.body)) > 0
ORDER BY message.created_at DESC
LIMIT $4
`, workspaceID, channelID, since, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	reversed := make([]channelsapp.TalkAISummaryMessage, 0)
	for rows.Next() {
		var item channelsapp.TalkAISummaryMessage
		var createdAt time.Time
		if err := rows.Scan(&item.SenderName, &item.Body, &createdAt); err != nil {
			return nil, err
		}
		item.CreatedAt = createdAt.UTC().Format(time.RFC3339Nano)
		reversed = append(reversed, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	result := make([]channelsapp.TalkAISummaryMessage, len(reversed))
	for index := range reversed {
		result[len(reversed)-1-index] = reversed[index]
	}
	return result, nil
}
