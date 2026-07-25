# Tổng quan kiến trúc WebTui Chat

WebTui Chat được thiết kế như một hệ thống chat realtime có backend Go + Gin, client đa nền tảng và hạ tầng vận hành đủ để mở rộng lâu dài.

## Bức tranh tổng thể

```text
Client
-> Nginx / SSL / Rate limit
-> Go API Server
-> PostgreSQL / Redis / MinIO
-> RabbitMQ
-> Worker
-> WebSocket broadcast / Notification / Webhook / Bot
```

## Vai trò từng lớp

- **Client**: web, admin, desktop và mobile. Client chỉ giao tiếp qua REST API, WebSocket hoặc endpoint upload được backend cấp quyền.
- **Access layer**: Nginx xử lý TLS, reverse proxy, route API/WebSocket, giới hạn tốc độ và log truy cập.
- **API server**: nhận request đồng bộ, xác thực, điều phối use case và trả response nhanh.
- **WebSocket platform**: quản lý connection, room, presence và broadcast event realtime.
- **Database layer**: PostgreSQL lưu dữ liệu bền vững, Redis lưu cache/session/rate limit, MinIO/S3 lưu file.
- **Queue layer**: RabbitMQ xử lý task bất đồng bộ, retry, dead letter và fan-out event.
- **Worker**: chạy job nền như notification, webhook, bot, report, cleanup và xử lý file.
- **Operations**: CI/CD, monitoring, logging, backup và rollback.

## Nguyên tắc thiết kế

- API server không xử lý tác vụ nặng nếu có thể đưa sang worker.
- WebSocket không thay thế database. Event realtime chỉ là cơ chế phân phối trạng thái mới.
- Queue event phải idempotent để retry không gây trùng dữ liệu.
- Module nghiệp vụ độc lập ở mức domain và application.
- Chi tiết kỹ thuật nằm trong adapter, có thể thay thế mà không sửa nghiệp vụ lõi.

