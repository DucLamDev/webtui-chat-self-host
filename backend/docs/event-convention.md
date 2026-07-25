# Quy ước event và queue

Backend dùng outbox pattern để đảm bảo dữ liệu đã commit trong PostgreSQL không bị mất event khi publish RabbitMQ thất bại.

## Event envelope

```json
{
  "event_id": "uuid",
  "event_type": "MessageCreated",
  "event_version": 1,
  "aggregate_type": "message",
  "aggregate_id": "uuid",
  "workspace_id": "uuid",
  "occurred_at": "2026-07-06T00:00:00Z",
  "payload": {}
}
```

## Quy tắc đặt tên

- Event dùng thì quá khứ: `MessageCreated`, `FileUploaded`, `WebhookDeliveryFailed`.
- Queue dùng dạng `module.purpose`, ví dụ `notification.send`, `webhook.delivery`.
- Dead letter queue thêm hậu tố `.dlq`.
- Retry queue thêm hậu tố `.retry`.

## Event tối thiểu cho MVP

| Event | Nguồn | Consumer |
|---|---|---|
| `UserLoggedIn` | auth | audit |
| `WorkspaceMemberInvited` | workspace | notification, audit |
| `MessageCreated` | message | websocket, notification, webhook, bot, audit |
| `MessageUpdated` | message | websocket, audit |
| `MessageDeleted` | message | websocket, audit |
| `FileUploaded` | file | file worker, websocket, audit |
| `NotificationRequested` | notification | notification worker |
| `WebhookDeliveryRequested` | webhook | webhook worker |
| `CronJobDue` | cronjob | worker |
| `BackupRequested` | backup | backup worker |

## Retry và idempotency

- Consumer phải idempotent theo `event_id`.
- Event lỗi tạm thời được retry có delay.
- Event lỗi vĩnh viễn đi vào dead letter.
- Không đưa secret vào payload event.

## Outbox

Mọi use case quan trọng ghi event vào `outbox_events` trong cùng transaction với dữ liệu nghiệp vụ. Worker outbox sẽ publish RabbitMQ và cập nhật trạng thái `published`, `failed` hoặc `dead`.

