# Clean Architecture cho backend

Backend dùng modular monolith. Mỗi module có ranh giới riêng và tuân thủ cùng một hướng phụ thuộc.

## Vòng phụ thuộc

```text
cmd
-> bootstrap
-> delivery / infrastructure
-> application
-> domain
```

Trong code nghiệp vụ, hướng phụ thuộc quan trọng là:

```text
delivery -> application -> domain
infrastructure -> domain/application qua interface
```

## Domain

`domain` là lõi nghiệp vụ của module.

Chứa:

- Entity.
- Value object.
- Domain event.
- Repository interface.
- Domain error.
- Business rule thuần.

Không chứa:

- Gin handler.
- SQL query.
- Redis command.
- RabbitMQ publish/consume.
- MinIO/S3 SDK.
- Logic parse HTTP request.

## Application

`application` điều phối use case.

Chứa:

- Service hoặc use case.
- Command và query.
- DTO cho input/output ở cấp use case.
- Mapper giữa DTO và domain.
- Validator nghiệp vụ.
- Interface cho publisher, transaction, clock hoặc external service nếu cần.

Luồng mẫu:

```text
Validate command
-> kiểm tra quyền
-> gọi domain rule
-> gọi repository interface
-> phát domain event qua publisher interface
-> trả DTO cho delivery
```

## Infrastructure

`infrastructure` hiện thực chi tiết kỹ thuật.

Chứa:

- PostgreSQL repository.
- Redis cache/session adapter.
- RabbitMQ publisher/consumer adapter.
- Storage adapter cho Local, MinIO hoặc S3.
- Adapter gọi hệ thống ngoài.

Infrastructure được phép import driver kỹ thuật, nhưng không được đẩy type kỹ thuật vào `domain`.

## Delivery

`delivery` là lớp nhận request từ bên ngoài.

Chứa:

- HTTP handler.
- WebSocket handler.
- Request/response model của giao thức.
- Route registration.
- Middleware binding riêng của module nếu cần.

Delivery không chứa nghiệp vụ. Handler chỉ parse request, gọi application và map lỗi sang response.

## Worker

`worker` chứa consumer hoặc job riêng của module.

Worker được dùng khi:

- Xử lý event RabbitMQ.
- Chạy job nền.
- Retry webhook.
- Gửi notification.
- Tạo report.
- Cleanup dữ liệu.

Worker vẫn gọi `application`, không đi thẳng xuống repository nếu không có lý do rất rõ.

## Bootstrap

`internal/bootstrap` chịu trách nhiệm khởi tạo toàn hệ thống.

Chứa:

- Config.
- Logger.
- Database pool.
- Redis client.
- RabbitMQ connection.
- Storage client.
- WebSocket manager.
- Module wiring.
- Router.
- Worker registry.

`cmd/api/main.go` và `cmd/worker/main.go` chỉ gọi bootstrap rồi chạy process.

