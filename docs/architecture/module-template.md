# Chuẩn module backend

Tài liệu này dùng khi tạo module mới trong `backend/internal/modules`.

## Cấu trúc chuẩn

```text
<module>/
├── domain/
│   ├── entity.go
│   ├── repository.go
│   ├── event.go
│   ├── errors.go
│   └── value_object.go
├── application/
│   ├── service.go
│   ├── command.go
│   ├── query.go
│   ├── dto.go
│   ├── mapper.go
│   └── validator.go
├── infrastructure/
│   ├── postgres/
│   ├── redis/
│   ├── rabbitmq/
│   └── storage/
├── delivery/
│   ├── http/
│   └── websocket/
├── worker/
├── routes.go
└── module.go
```

## Vai trò file

- `domain/entity.go`: entity và method nghiệp vụ cốt lõi.
- `domain/repository.go`: interface repository, không chứa SQL.
- `domain/event.go`: domain event phát ra từ module.
- `domain/errors.go`: lỗi nghiệp vụ có thể map sang HTTP/WebSocket response.
- `application/service.go`: use case chính.
- `application/command.go`: input cho hành động ghi.
- `application/query.go`: input cho hành động đọc.
- `application/dto.go`: output của use case.
- `application/mapper.go`: chuyển đổi entity sang DTO.
- `application/validator.go`: validate nghiệp vụ cấp use case.
- `infrastructure/postgres`: hiện thực repository bằng PostgreSQL.
- `infrastructure/redis`: cache, lock hoặc session nếu module cần.
- `infrastructure/rabbitmq`: publisher/consumer event của module.
- `infrastructure/storage`: adapter file nếu module cần.
- `delivery/http`: handler, request, response và route HTTP.
- `delivery/websocket`: handler hoặc event binding realtime.
- `worker`: consumer và background job của module.
- `module.go`: khởi tạo dependency nội bộ module.
- `routes.go`: đăng ký route vào router chính nếu module có HTTP API.

## Checklist khi tạo module mới

- [ ] Có `domain` trước khi viết handler.
- [ ] Repository là interface ở `domain` hoặc `application`.
- [ ] Use case nằm trong `application`.
- [ ] Handler không gọi trực tiếp repository.
- [ ] Adapter PostgreSQL không rò rỉ SQL model ra ngoài infrastructure.
- [ ] Event quan trọng có tên rõ và payload ổn định.
- [ ] Worker xử lý event idempotent.
- [ ] Lỗi nghiệp vụ được định nghĩa rõ và map response thống nhất.
- [ ] Test unit cho domain/application.
- [ ] Test integration cho repository hoặc delivery nếu có logic đáng kể.

## Quy tắc đặt tên

- Module dùng danh từ số ít hoặc cụm ngắn: `message`, `channel`, `workspace`.
- Riêng module người dùng dùng `users` để tránh xung đột với từ phổ biến `user`.
- Command dùng động từ: `SendMessageCommand`, `CreateChannelCommand`.
- Query dùng mục đích đọc: `ListMessagesQuery`, `GetWorkspaceQuery`.
- Event dùng thì quá khứ: `MessageCreated`, `FileUploaded`, `WebhookDeliveryFailed`.

