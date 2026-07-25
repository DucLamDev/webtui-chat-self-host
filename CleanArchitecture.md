# Kiến trúc Clean Architecture cho WebTui Chat

Tài liệu này mô tả kiến trúc mục tiêu cho hệ thống WebTui Chat dựa trên mô hình trong sơ đồ: client đa nền tảng, lớp truy cập qua Nginx, backend Go + Gin, WebSocket realtime, RabbitMQ/Kafka cho bất đồng bộ, PostgreSQL/Redis/MinIO cho dữ liệu và CI/CD bằng GitHub Actions.

## Mục tiêu kiến trúc

- Phát triển backend theo modular monolith để dễ bắt đầu, dễ triển khai và vẫn có đường tách module thành microservice khi cần.
- Áp dụng Clean Architecture ở cấp module: nghiệp vụ nằm trong `domain` và `application`, chi tiết kỹ thuật nằm ở `infrastructure` và `delivery`.
- Hỗ trợ chat realtime bằng WebSocket, đồng thời dùng queue cho các tác vụ nặng như gửi thông báo, webhook, bot, xử lý file và báo cáo.
- Chuẩn hóa vận hành: migration, backup, health check, log, metric, alert và pipeline CI/CD.
- Giữ tài liệu dự án bằng tiếng Việt có dấu để cả người phát triển và agent đều đọc cùng một quy ước.

## Thành phần tổng quan

1. **Người dùng / Client**
   - Web App: Next.js.
   - Admin Panel: Next.js + shadcn/ui.
   - Desktop App: Tauri.
   - Mobile App: Flutter.

2. **Lớp truy cập**
   - Nginx làm reverse proxy.
   - SSL qua Let's Encrypt.
   - Routing riêng cho REST API và WebSocket.
   - Rate limiting, request logging và WAF ở biên.

3. **WebTui Chat Server**
   - Backend Go + Gin cho REST API.
   - WebSocket manager cho realtime connection.
   - Các module chính: auth, users, workspace, department, channel, message, file, notification, bot, webhook, api token, cronjob, audit, admin, health và backup.

4. **Dữ liệu và lưu trữ**
   - PostgreSQL primary cho dữ liệu chính.
   - PostgreSQL read replica cho truy vấn đọc nặng khi cần mở rộng.
   - Redis cho cache, session, rate limit và dữ liệu tạm.
   - Local Storage, MinIO hoặc S3 cho file.
   - Backup tự động ra local, MinIO/S3 hoặc remote storage.

5. **Queue và event stream**
   - RabbitMQ là mặc định cho task queue, job và domain event.
   - Kafka chỉ bật khi hệ thống cần event streaming dung lượng lớn hoặc nhiều consumer độc lập.
   - Dead letter queue và retry queue là bắt buộc với tác vụ quan trọng.

6. **Tích hợp hệ thống**
   - Incoming webhook để hệ thống ngoài đẩy dữ liệu vào.
   - Outgoing webhook để phát event ra ngoài.
   - Bot API để module bot xử lý lệnh và automation.
   - Tích hợp ticket, billing, monitoring, alerting qua adapter riêng.

7. **Worker / Background job**
   - Worker chạy độc lập với API server.
   - Xử lý file, gửi thông báo, scheduled job, báo cáo, cleanup và webhook retry.

8. **CI/CD và vận hành**
   - GitHub Actions chạy lint, test, security scan, Docker build và deploy.
   - Dev, staging và production tách cấu hình.
   - Monitoring bằng Prometheus, Grafana, Loki và AlertManager.

## Quy tắc phụ thuộc

Luồng phụ thuộc đi từ ngoài vào trong:

```text
delivery -> application -> domain
infrastructure -> application/domain qua interface
bootstrap -> module wiring
cmd -> bootstrap
```

Các quy tắc bắt buộc:

- `domain` không import Gin, PostgreSQL, Redis, RabbitMQ, MinIO hoặc package kỹ thuật.
- `application` điều phối use case, kiểm tra quyền, validate nghiệp vụ và gọi interface repository/publisher.
- `infrastructure` hiện thực các interface bằng PostgreSQL, Redis, RabbitMQ, storage hoặc hệ thống ngoài.
- `delivery` chuyển HTTP/WebSocket/gRPC request thành command/query của application.
- `bootstrap` là nơi nối dependency, đăng ký route, worker và provider.
- `cmd/*` chỉ khởi động process, không chứa nghiệp vụ.

## Cấu trúc backend mục tiêu

```text
backend/
├── api/openapi/
├── cmd/
│   ├── api/
│   └── worker/
├── configs/
├── deployments/
├── db/
│   ├── migrations/
│   ├── seed/
│   └── schema/
├── docs/
├── internal/
│   ├── bootstrap/
│   ├── config/
│   ├── platform/
│   ├── shared/
│   └── modules/
├── pkg/
├── scripts/
├── test/
├── Dockerfile
└── go.mod
```

Chỉ tạo `pkg/` khi có thư viện Go thật sự cần public ra ngoài `internal/`.

## Chuẩn module backend

Mỗi module đi theo cùng một cấu trúc:

```text
message/
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

Ví dụ luồng `SendMessage`:

```text
HTTP/WebSocket request
-> delivery chuyển request thành command
-> application validate dữ liệu
-> application kiểm tra quyền trong workspace/channel
-> domain tạo entity hoặc event
-> repository lưu PostgreSQL
-> publisher phát event vào RabbitMQ
-> WebSocket manager broadcast tới client
-> worker xử lý thông báo, webhook, bot hoặc audit log
```

## WebSocket

WebSocket là platform dùng chung, không đặt logic kết nối rải rác trong từng module.

```text
backend/internal/platform/websocket/
├── hub.go
├── client.go
├── manager.go
├── event.go
└── connection.go
```

Module chỉ đăng ký event handler hoặc gọi interface broadcast. Không để module tự quản lý vòng đời connection.

## RabbitMQ

RabbitMQ cần tách rõ exchange, queue, publisher, consumer, dead letter và retry:

```text
backend/internal/platform/rabbitmq/
├── exchange/
├── queue/
├── publisher/
├── consumer/
├── deadletter/
└── retry/
```

Các event quan trọng như message created, file uploaded, notification requested, webhook delivery failed phải có retry và dead letter queue.

## Storage

Storage được bọc qua interface để chuyển đổi giữa Local, MinIO và S3:

```text
backend/internal/platform/storage/
├── local/
├── minio/
└── s3/
```

Luồng upload:

```text
Upload request
-> application kiểm tra quyền và metadata
-> storage adapter lưu file
-> repository lưu metadata
-> publisher phát event file uploaded
-> worker tạo preview hoặc scan nếu cần
```

## Frontend mục tiêu

```text
frontend/
├── apps/
│   ├── web/
│   └── admin/
├── packages/
│   ├── api-client/
│   ├── config/
│   ├── icons/
│   ├── types/
│   └── ui/
└── tests/
```

Frontend dùng Next.js App Router, TanStack Query cho server state, Zustand cho client state, shadcn/ui cho UI primitive và typed API client để tránh gọi API trực tiếp trong component.

## Deploy mục tiêu

```text
deploy/
├── docker/
├── nginx/templates/
├── postgres/
├── redis/
├── rabbitmq/
├── minio/
├── prometheus/
├── grafana/
├── loki/
├── scripts/
└── k8s/
```

Secret thật chỉ nằm trong `.env` trên server hoặc GitHub Secrets, không commit vào repository.

## Database PostgreSQL

Schema nền nằm ở `backend/db/migrations/000001_initial_schema.up.sql`.

Nhóm bảng chính:

- Auth và user: `users`, `user_sessions`, `workspace_invites`.
- Workspace và RBAC: `workspaces`, `workspace_settings`, `workspace_members`, `departments`, `department_members`, `permissions`, `roles`, `role_permissions`, `workspace_member_roles`, `channel_member_roles`.
- Chat realtime: `channels`, `channel_members`, `direct_conversations`, `direct_conversation_members`, `messages`, `message_reactions`, `message_mentions`, `message_reads`, `message_attachments`, `search_documents`.
- File: `files`, `file_versions`.
- Notification và presence: `notifications`, `notification_jobs`, `user_presence`.
- Bot, API token và webhook: `bots`, `bot_installations`, `api_tokens`, `api_scopes`, `api_token_scopes`, `incoming_webhooks`, `outgoing_webhooks`, `webhook_deliveries`.
- Worker và vận hành: `cron_jobs`, `cron_job_runs`, `outbox_events`, `audit_logs`, `backup_jobs`, `backup_runs`, `system_settings`.

Chi tiết thiết kế nằm ở `docs/database/postgresql-design.md`.

## CI/CD với Docker Compose

Workflow mục tiêu:

```text
pull request -> CI
push main -> CI -> build image -> push GHCR
manual deploy -> SSH server -> docker compose pull -> migration -> up -d -> health check
```

File chính:

- `.github/workflows/ci.yml`
- `.github/workflows/docker.yml`
- `.github/workflows/deploy.yml`
- `deploy/docker/compose.dev.yml`
- `deploy/docker/compose.prod.yml`

## Nguyên tắc mở rộng

- Bắt đầu bằng modular monolith, chưa tách microservice nếu chưa có áp lực vận hành rõ ràng.
- Khi một module cần tách, giữ nguyên `domain` và `application`, thay `delivery` và `infrastructure` bằng adapter phù hợp.
- Mọi tích hợp ngoài hệ thống phải đi qua interface trong application/domain và adapter trong infrastructure.
- Tài liệu mới trong `docs/` phải viết bằng tiếng Việt có dấu.
