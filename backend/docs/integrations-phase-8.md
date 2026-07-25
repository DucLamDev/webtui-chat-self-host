# Bot, API token và webhook

Tài liệu này mô tả phase 8 của backend WebTui Chat: mở tích hợp ngoài hệ thống bằng API token, bot, incoming webhook và outgoing webhook.

## API token

API token thuộc một workspace và được gán scope từ bảng `api_scopes`.

Endpoint quản lý API token yêu cầu quyền `api_token.manage`. Webhook vẫn dùng quyền `webhook.manage`.

Scope seed mặc định:

- `message.read`
- `message.write`
- `file.write`
- `bot.write`
- `webhook.write`
- `admin.read`

Endpoint quản lý:

```text
GET /api/v1/api-scopes
GET /api/v1/workspaces/{workspace_id}/api-tokens
POST /api/v1/workspaces/{workspace_id}/api-tokens
DELETE /api/v1/workspaces/{workspace_id}/api-tokens/{token_id}
```

Token gốc chỉ trả về một lần khi tạo. Database chỉ lưu SHA-256 hash của token.

Gửi message bằng API token:

```text
POST /api/v1/integrations/messages
Authorization: Bearer {api_token}
```

Body:

```json
{
  "channel_id": "CHANNEL_ID",
  "body": "Server alert: CPU cao",
  "metadata": {
    "severity": "warning"
  }
}
```

Token cần scope `message.write`.

## Bot

Bot thuộc workspace, có thể cài ở toàn workspace hoặc một channel cụ thể.

Endpoint:

```text
GET /api/v1/workspaces/{workspace_id}/bots
POST /api/v1/workspaces/{workspace_id}/bots
GET /api/v1/workspaces/{workspace_id}/bots/{bot_id}/installations
POST /api/v1/workspaces/{workspace_id}/bots/{bot_id}/installations
POST /api/v1/workspaces/{workspace_id}/bots/{bot_id}/messages
```

Bot message được lưu vào `messages` với `kind = bot`, đồng bộ `search_documents` và ghi `outbox_events` để realtime/notification/outgoing webhook xử lý tiếp.

## Incoming webhook

Incoming webhook dùng để hệ thống ngoài gửi message vào channel.

Endpoint quản lý:

```text
GET /api/v1/workspaces/{workspace_id}/incoming-webhooks
POST /api/v1/workspaces/{workspace_id}/incoming-webhooks
```

Endpoint dispatch public:

```text
POST /api/v1/hooks/incoming/{webhook_id}
```

Secret có thể gửi qua header:

```text
X-WebTui-Webhook-Secret: {secret}
```

Hoặc gửi trong body:

```json
{
  "secret": "SECRET",
  "body": "Deploy production thành công",
  "metadata": {
    "source": "github-actions"
  }
}
```

Nếu incoming webhook có `channel_id` mặc định thì body không cần gửi `channel_id`. Nếu muốn ghi đè channel, gửi thêm `channel_id`.

## Outgoing webhook

Outgoing webhook nhận event từ `outbox_events`. Worker tạo `webhook_deliveries`, sau đó gửi HTTP POST đến `target_url`.

Endpoint quản lý:

```text
GET /api/v1/workspaces/{workspace_id}/outgoing-webhooks
POST /api/v1/workspaces/{workspace_id}/outgoing-webhooks
GET /api/v1/workspaces/{workspace_id}/outgoing-webhooks/{webhook_id}/deliveries
```

Nếu `event_types` rỗng, webhook nhận tất cả event. Nếu có danh sách, webhook chỉ nhận event nằm trong danh sách, ví dụ:

```json
{
  "event_types": ["MessageCreated", "ReactionChanged"]
}
```

Payload gửi đi:

```json
{
  "event_id": "EVENT_ID",
  "event_type": "MessageCreated",
  "event_version": 1,
  "aggregate_type": "message",
  "aggregate_id": "MESSAGE_ID",
  "payload": {},
  "occurred_at": "2026-07-06T00:00:00Z"
}
```

Header chữ ký:

```text
X-WebTui-Event-ID
X-WebTui-Event-Type
X-WebTui-Timestamp
X-WebTui-Signature
```

`X-WebTui-Signature` có dạng `sha256={hex}`. Backend ký chuỗi:

```text
timestamp + "." + raw_body
```

Key ký là SHA-256 hex của secret được trả về khi tạo outgoing webhook. Cách này giúp backend không cần lưu secret gốc trong database.

## Worker

Worker phase 8 có thêm tác vụ:

```text
webhook_deliveries
```

Tác vụ này claim delivery `pending` hoặc `failed` đã đến hạn, gửi HTTP, retry và chuyển `dead` khi vượt số lần retry.
