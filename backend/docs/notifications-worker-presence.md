# Notification, outbox, worker và presence

Tài liệu này mô tả phần phase 7 của backend WebTui Chat: xử lý event bền vững bằng outbox, tạo notification từ mention, chạy notification job và lưu presence theo database để WebSocket có thể scale nhiều node.

## Mục tiêu

- API ghi dữ liệu nghiệp vụ và ghi event vào `outbox_events` trong cùng transaction.
- Worker lấy event từ `outbox_events`, xử lý handler nội bộ, publish RabbitMQ nếu bật, rồi đánh dấu `published`.
- Mention trong message tạo bản ghi `notifications` và `notification_jobs`.
- Notification job hiện có kênh `desktop` ở mức nền; các kênh `push`, `email`, `webhook`, `sms` đã có schema để nối provider sau.
- Presence lưu trong PostgreSQL qua `user_presence`, không phụ thuộc bộ nhớ của một API node.

## Luồng outbox

Khi gửi message có mention:

```text
POST /api/v1/workspaces/{workspace_id}/channels/{channel_id}/messages
-> messages
-> message_mentions
-> outbox_events: MessageCreated
-> worker claim outbox event
-> notification handler tạo notifications và notification_jobs
-> worker publish RabbitMQ nếu RABBITMQ_ENABLED=true
-> outbox_events.status = published
```

Worker có retry:

- `pending`: event mới.
- `processing`: event đang được worker xử lý.
- `failed`: event lỗi và còn lượt retry.
- `dead`: event lỗi quá số lượt retry.
- `published`: event đã xử lý xong.

Nếu worker chết khi event đang `processing`, event sẽ được claim lại sau khoảng thời gian an toàn.

## Notification API

Các endpoint hiện có:

```text
GET /api/v1/notifications?workspace_id={workspace_id}&limit=50
PUT /api/v1/notifications/{notification_id}/read
PUT /api/v1/notifications/read-all?workspace_id={workspace_id}
```

Notification mention được tạo idempotent theo `event_id`, nên worker chạy lại cùng event không tạo trùng notification cho cùng user.

## Presence API

Các endpoint hiện có:

```text
GET /api/v1/workspaces/{workspace_id}/presence?limit=50
PUT /api/v1/workspaces/{workspace_id}/presence/heartbeat
```

Body heartbeat:

```json
{
  "device_id": "desktop-local",
  "socket_id": "socket-local-1",
  "node_id": "api-local",
  "status": "online",
  "metadata": {
    "platform": "desktop"
  }
}
```

Quy ước:

- `device_id` bắt buộc để nhận diện thiết bị.
- `socket_id` không gửi thì backend dùng `device_id`.
- `node_id` không gửi thì backend dùng `api-local`.
- `status` chỉ nhận `online`, `away`, `offline`; giá trị rỗng hoặc sai sẽ về `online`.
- Worker tự chuyển presence cũ sang `offline` nếu heartbeat quá hạn.

## Chạy worker local

Từ thư mục `backend/`:

```powershell
go run ./cmd/worker
```

Nếu muốn publish event ra RabbitMQ local:

```powershell
$env:RABBITMQ_ENABLED="true"
$env:RABBITMQ_URL="amqp://guest:guest@localhost:5672/"
go run ./cmd/worker
```

Nếu không bật RabbitMQ, worker vẫn xử lý handler nội bộ và đánh dấu outbox event là `published`.

## Kiểm tra nhanh bằng SQL

```sql
SELECT id, event_type, status, retry_count, created_at
FROM outbox_events
ORDER BY created_at DESC
LIMIT 20;

SELECT id, user_id, type, title, read_at, delivered_at, created_at
FROM notifications
ORDER BY created_at DESC
LIMIT 20;

SELECT id, channel, status, attempt_count, sent_at, error
FROM notification_jobs
ORDER BY created_at DESC
LIMIT 20;

SELECT user_id, workspace_id, device_id, socket_id, node_id, status, last_heartbeat_at
FROM user_presence
ORDER BY last_heartbeat_at DESC
LIMIT 20;
```
