# Realtime, queue và worker

Hệ thống chat cần realtime nhanh nhưng vẫn phải bền vững khi xử lý thông báo, bot, webhook và file. Vì vậy API server, WebSocket và worker được tách trách nhiệm rõ ràng.

## WebSocket

WebSocket manager nằm ở `backend/internal/platform/websocket`.

Trách nhiệm:

- Quản lý connection.
- Gắn user với workspace/channel.
- Theo dõi presence.
- Broadcast event tới room phù hợp.
- Ngắt connection khi token hết hạn hoặc quyền thay đổi.

Không đặt logic nghiệp vụ phức tạp trong WebSocket manager. Module phát event hoặc đăng ký handler, còn manager chỉ phân phối realtime.

## RabbitMQ

RabbitMQ là mặc định cho task bất đồng bộ.

Nhóm chính:

- `exchange`: định nghĩa cách route event.
- `queue`: định nghĩa queue theo consumer.
- `publisher`: gửi event từ application.
- `consumer`: nhận event cho worker.
- `deadletter`: lưu event xử lý thất bại.
- `retry`: xử lý retry có delay.

## Luồng gửi tin nhắn

```text
Client gửi message qua HTTP hoặc WebSocket
-> API server xác thực và gọi SendMessage
-> Message application validate quyền trong workspace/channel
-> Message repository lưu PostgreSQL
-> Message application publish MessageCreated
-> WebSocket broadcast message mới
-> Notification worker gửi thông báo nếu người nhận offline
-> Webhook worker gửi outgoing webhook nếu workspace bật tích hợp
-> Bot worker kiểm tra mention hoặc command
-> Audit worker ghi log hành động nếu cần
```

## Quy tắc event

- Event phải có `event_id`, `event_type`, `occurred_at`, `aggregate_id` và `payload`.
- Consumer phải idempotent theo `event_id`.
- Event quan trọng phải có retry và dead letter queue.
- Payload không chứa secret.
- Thay đổi payload phá vỡ tương thích phải tạo version mới.

## Khi nào dùng Kafka

Chỉ cân nhắc Kafka khi có ít nhất một điều kiện:

- Lưu lượng event rất lớn.
- Nhiều consumer độc lập cần replay event.
- Cần event streaming dài hạn.
- Cần phân tích dữ liệu realtime ở quy mô lớn.

Nếu chưa có các nhu cầu trên, RabbitMQ đơn giản và phù hợp hơn.

