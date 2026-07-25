# Kế hoạch hoàn thiện backend WebTui Chat

Tài liệu này chia nhỏ kế hoạch backend thành nhiều phase để hoàn thiện hệ thống trước khi chuyển sang frontend. Phạm vi bám theo kiến trúc đã thiết kế: Go + Gin, PostgreSQL, Redis, RabbitMQ, WebSocket, MinIO/S3, worker, CI/CD và Docker Compose.

## Nguyên tắc triển khai

- Hoàn thiện backend trước frontend; frontend chỉ bắt đầu khi API contract, auth, chat realtime, file upload và admin API nền đã ổn.
- Mỗi phase phải có tiêu chí nghiệm thu rõ ràng, có test tối thiểu và có tài liệu cập nhật.
- Ưu tiên MVP chạy thật: đăng nhập, workspace, channel, message, WebSocket, upload file, notification, bot API, webhook, cronjob, backup.
- Không tách microservice trong giai đoạn đầu; API server và worker là hai process/container độc lập trong cùng modular monolith.
- Không đưa logic nghiệp vụ vào `cmd`, `delivery` hoặc adapter kỹ thuật.

## Mốc MVP backend

Backend được xem là đạt MVP khi có đủ:

- Đăng nhập, refresh token, logout và phân quyền workspace/channel.
- Quản lý user, workspace, department, channel và direct message.
- Gửi, sửa, xóa, đọc message; reaction, mention, thread và WebSocket realtime.
- Upload/download file qua Local/MinIO, lưu metadata và gắn file vào message.
- Notification realtime và notification job tối thiểu.
- Bot API, API token, incoming webhook, outgoing webhook và webhook log.
- Cronjob cơ bản, audit log, health check và backup database.
- Docker Compose chạy được local và production.
- CI/CD chạy lint, test, build image và deploy qua GitHub Actions.

## Bảng kế hoạch tổng quan

| Phase | Tên phase | Mục tiêu chính | Kết quả bàn giao | Điều kiện chuyển phase |
|---|---|---|---|---|
| 0 | Chốt kiến trúc và contract | Khóa phạm vi backend MVP, chuẩn hóa tài liệu, schema và API style | Tài liệu kiến trúc, database schema, API convention, roadmap | Schema nền và roadmap được review |
| 1 | Nền tảng Go + Gin | Khởi tạo backend chạy được, có config, logger, router, graceful shutdown | `go.mod`, `cmd/api`, `cmd/worker`, health endpoint | API server chạy local và trả `/health` |
| 2 | Platform adapters | Kết nối PostgreSQL, Redis, RabbitMQ, storage, WebSocket manager | Adapter trong `internal/platform` | Integration smoke test qua Docker Compose |
| 3 | Auth, user, RBAC | Đăng nhập và phân quyền động theo role/permission | Auth API, user API, RBAC service, middleware | Có thể login và kiểm tra quyền route |
| 4 | Workspace, department, channel | Quản lý tenant, phòng ban, kênh và direct message | Workspace/channel/direct APIs | Tạo workspace, channel, DM thành công |
| 5 | Message và realtime | Chat realtime ổn định qua WebSocket | Message API, WebSocket event, thread, reaction, mention | 2 client nhận message realtime |
| 6 | File và storage | Upload/download file, gắn file vào message | File API, storage adapter, file version | Upload file và gửi message có attachment |
| 7 | Notification và worker | Xử lý queue, notification, outbox event, retry | Worker, notification jobs, outbox publisher | Event từ API được worker xử lý |
| 8 | Bot, API token, webhook | Mở tích hợp hệ thống ngoài | Bot API, token scope, incoming/outgoing webhook | Gửi message qua API token/webhook |
| 9 | Cronjob và module runner | Chạy job định kỳ và job/script có kiểm soát | Cronjob manager, job run log, module runner MVP | Job chạy theo lịch và ghi log |
| 10 | Admin, audit, health, backup | Hoàn thiện API quản trị và vận hành | Admin APIs, audit log, health check, backup/restore | Admin có API đủ để quản lý hệ thống |
| 11 | Hardening và performance | Tăng độ tin cậy, bảo mật, quan sát và test | Rate limit, metrics, logs, test suite, benchmark | Backend đủ ổn để demo nội bộ |
| 12 | Đóng gói demo và release backend | Chạy thật trên server demo, chuẩn bị open-source backend | Dockerfile, compose prod, CI/CD, tài liệu deploy | Deploy thành công lên server demo |

## Trạng thái hiện tại

| Phase | Trạng thái | Ghi chú |
|---|---|---|
| 0 | Hoàn thành nền | Đã có roadmap, database schema, API convention, event convention, security baseline và OpenAPI nền |
| 1 | Hoàn thành nền | Đã có `go.mod`, `cmd/api`, `cmd/worker`, config, logger, router, middleware response/recovery/request id/access log và health endpoints |
| 2 | Hoàn thành nền | Đã có PostgreSQL adapter, migration runner, Redis/RabbitMQ adapter bật theo config, storage local, WebSocket manager, `/ready` kiểm tra dependency và nhóm route `/api/v1` |
| 3 | Hoàn thành | Đã có auth API, JWT middleware, refresh token hash/rotate, user CRUD/profile/status, session revoke, RBAC permission/role/role assignment, seed permission/role và audit auth/role cơ bản |
| 4 | Hoàn thành | Đã có workspace CRUD mềm, member, invite, settings, department CRUD/member, channel CRUD/member/archive, direct conversation chống trùng, channel read state, xử lý lỗi cạnh member/role và test application cho DM |
| 5 | Hoàn thành | Đã có message command/query, timeline cursor `before`, thread bằng `thread_root_id`, reaction, mention, search bằng PostgreSQL full text search, đồng bộ `search_documents`, outbox event `MessageCreated/Updated/Deleted/ReactionChanged` và test application |
| 6 | Hoàn thành | Đã có file upload/download, validate MIME/dung lượng, checksum SHA-256, storage adapter local và MinIO/S3, metadata `files`, version trong `file_versions`, attachment với `message_attachments`, OpenAPI/docs/local-run và test application |
| 7 | Hoàn thành | Đã có outbox publisher, RabbitMQ event exchange, worker xử lý outbox/notification job/presence cleanup, notification mention idempotent, API notification, API presence, OpenAPI/docs/local-run và test application |
| 8 | Hoàn thành | Đã có API token hash/scope/revoke với quyền `api_token.manage`, endpoint gửi message bằng API token, bot CRUD/install/send message, incoming webhook dispatch, outgoing webhook delivery log/retry/signature, worker gửi delivery, OpenAPI/docs/local-run và test application |
| 9 | Hoàn thành | Đã có cronjob CRUD, schedule parser, lock tránh chạy trùng, run log, manual run, worker claim job đến hạn, HTTP runner, builtin cleanup allowlist, script runner allowlist, OpenAPI/docs/local-run và test application |
| 10 | Hoàn thành | Đã có admin dashboard stats, admin deep health, audit log filter với before/after data, backup job/run database bằng pg_dump, worker backup theo lịch, script restore dev/staging, OpenAPI/docs/local-run và test application |
| 11 | Hoàn thành | Đã có security headers, trusted proxies, CORS allowlist, rate limit in-memory, `/metrics` Prometheus với HTTP/dependency/WebSocket metric, cấu hình env, docs hardening/observability và test middleware |
| 12 | Hoàn thành nền | Đã có Dockerfile multi-stage API/worker/migrate, Compose production cho VPS, Nginx domain `chat.vpsttt.com`, script Let's Encrypt, CloudAMQP config, mẫu `.env` production, GitHub Actions deploy và tài liệu VPS |

## Bảng kế hoạch chi tiết

| Phase | Hạng mục | Công việc chi tiết | Module/thư mục liên quan | Tiêu chí nghiệm thu | Ưu tiên |
|---|---|---|---|---|---|
| 0 | Kiến trúc backend | Rà soát `CleanArchitecture.md`, database schema, CI/CD, Docker Compose, quyết định MVP backend | `docs/architecture`, `docs/database`, `deploy` | Tài liệu không mâu thuẫn, scope MVP rõ | P0 |
| 0 | API convention | Quy định response format, error code, pagination, filter, sort, idempotency key, request id | `backend/docs`, `api/openapi` | Có tài liệu API convention | P0 |
| 0 | Event convention | Quy định tên event, payload, version, retry, dead letter, outbox | `docs/architecture/realtime-queue.md` | Có mẫu event `MessageCreated`, `NotificationRequested` | P0 |
| 0 | Security baseline | Quy định password hash, JWT, refresh token, API token hash, webhook signature | `docs/architecture`, `internal/shared/auth` | Có checklist bảo mật backend | P0 |
| 1 | Go module | Khởi tạo `go.mod`, chọn module path, thêm Gin, config, logger, validator, migration lib | `backend/go.mod` | `go test ./...` chạy được | P0 |
| 1 | Entrypoint API | Tạo `cmd/api/main.go`, bootstrap app, router, middleware base, graceful shutdown | `backend/cmd/api`, `internal/bootstrap` | Chạy API server local | P0 |
| 1 | Entrypoint worker | Tạo `cmd/worker/main.go`, bootstrap worker, signal handling | `backend/cmd/worker`, `internal/bootstrap` | Worker start/stop sạch | P0 |
| 1 | Config | Đọc env, validate config, tách config API/worker/database/redis/rabbitmq/storage | `internal/config` | Thiếu env quan trọng thì fail rõ | P0 |
| 1 | Shared response | Chuẩn hóa response success/error, request id, error mapping | `internal/shared/response`, `errors` | Handler trả format thống nhất | P0 |
| 1 | Health base | `/health`, `/ready`, `/version` | `health`, `bootstrap` | Docker healthcheck dùng được | P0 |
| 2 | PostgreSQL adapter | Connection pool, transaction manager, migration runner, repository helper | `internal/platform/database` | Kết nối DB qua Compose | P0 |
| 2 | Redis adapter | Redis client, cache helper, lock helper, rate limit storage | `internal/platform/redis` | Ping Redis và set/get smoke test | P1 |
| 2 | RabbitMQ adapter | Connection, exchange, queue, publisher, consumer, retry, dead letter | `internal/platform/rabbitmq` | Publish/consume event test | P0 |
| 2 | Storage adapter | Interface storage, Local, MinIO, presigned URL, object metadata | `internal/platform/storage` | Upload/download qua MinIO local | P0 |
| 2 | WebSocket manager | Hub, client, room, auth handshake, broadcast, presence hook | `internal/platform/websocket` | Kết nối WebSocket local | P0 |
| 2 | Logger/monitoring base | Structured log, request log, panic recover, metrics interface | `internal/platform/logger`, `monitoring` | Log có request id và user id nếu có | P1 |
| 3 | Auth domain | Entity/session/token, password hash, refresh token hash | `modules/auth/domain` | Unit test auth rule | P0 |
| 3 | Auth API | Register/login/logout/refresh/me, JWT middleware | `modules/auth/delivery/http` | Login nhận access/refresh token | P0 |
| 3 | Users module | CRUD user, profile, avatar id, status, last seen | `modules/users` | Admin/user đọc hồ sơ được | P0 |
| 3 | RBAC | Permission, role, role assignment, policy checker | `modules/auth`, `modules/admin` | Route kiểm tra permission đúng | P0 |
| 3 | Session management | Danh sách session, revoke device, revoke all | `user_sessions` | Logout và revoke token hoạt động | P1 |
| 3 | Audit auth | Ghi audit login/logout/role change | `modules/audit` | Audit có before/after data khi đổi quyền | P1 |
| 4 | Workspace | Tạo/sửa/xóa mềm workspace, member, invite, settings | `modules/workspace` | Tạo workspace và invite member | P0 |
| 4 | Department | CRUD department, department member | `modules/department` | Gán user vào phòng ban | P1 |
| 4 | Channel | Public/private channel, archive, member, permission | `modules/channel` | Tạo channel và join/leave | P0 |
| 4 | Direct message | Tạo DM 1-1, group DM, participant key, member list | `direct_conversations`, `modules/channel` | Tạo DM không bị trùng participant | P0 |
| 4 | Channel read state | Cập nhật `last_read_at`, `last_read_message_id` | `channel_members` | Unread count tính được | P0 |
| 5 | Message command | Send/edit/delete message, validation, permission, transaction | `modules/messages` | Gửi message text vào channel | P0 |
| 5 | Message query | List timeline, cursor pagination, thread replies | `modules/messages` | Timeline nhanh theo index | P0 |
| 5 | Reaction/mention | Add/remove reaction, parse mention, ghi event để phase notification xử lý | `message_reactions`, `message_mentions` | Mention được lưu và event được ghi vào outbox | P0 |
| 5 | Thread | `parent_id`, `thread_root_id`, list thread không recursive query | `messages` | Reply nhiều cấp vẫn truy vấn theo root | P1 |
| 5 | WebSocket event | MessageCreated/Updated/Deleted, ReactionChanged qua outbox nền; Typing, Presence nối ở phase realtime sau | `platform/websocket`, `modules/messages` | Event message được ghi bền vững để worker broadcast | P0 |
| 5 | Search message | Full text search message bằng `search_vector` | `messages`, `search_documents` | Tìm message không dùng LIKE | P1 |
| 6 | File upload | Upload multipart, save metadata, checksum, MIME validation | `modules/files`, `platform/storage` | Upload file vào local storage | P0 |
| 6 | File download | Stream file, permission check, private object | `modules/files` | Chỉ member có quyền tải file | P0 |
| 6 | Attachment | Gắn file vào message, list attachment | `message_attachments`, `modules/files` | Gửi message có file | P0 |
| 6 | File version | Tạo version mới cho avatar/logo/document | `file_versions`, `modules/files` | File có version_number tăng đúng | P1 |
| 6 | File worker | Preview/image metadata/cleanup failed upload | `modules/file/worker` | Worker xử lý file event | P2 |
| 7 | Outbox publisher | Lấy `outbox_events`, publish RabbitMQ, retry/dead | `outbox_events`, `platform/rabbitmq` | Event DB được publish an toàn | P0 |
| 7 | Notification service | Tạo notification từ mention/message/invite/system | `modules/notification` | Notification unread hoạt động | P0 |
| 7 | Notification jobs | Tạo job desktop/push/email/webhook/SMS, retry | `notification_jobs` | Job pending được worker xử lý | P0 |
| 7 | Presence | Update heartbeat, online/away/offline, multi-node socket | `user_presence`, `websocket` | Presence không phụ thuộc memory đơn node | P1 |
| 7 | Worker framework | Consumer registry, graceful shutdown, concurrency, idempotency | `cmd/worker`, `bootstrap/worker.go` | Worker chạy nhiều consumer | P0 |
| 8 | API token | Token hash, scope, revoke, last used, permission middleware | `modules/api_token` | API token gọi endpoint được phép | P0 |
| 8 | Incoming webhook | Endpoint nhận webhook, verify secret, map thành message/event | `modules/webhook` | Webhook gửi tin vào channel | P0 |
| 8 | Outgoing webhook | Subscribe event, delivery log, signature, retry/dead | `webhook_deliveries` | Event message gọi target URL | P0 |
| 8 | Bot module | Bot CRUD, bot installation, bot message sender | `modules/bot` | Bot gửi message vào channel | P0 |
| 8 | Server alert API | API gửi alert vào channel bằng token/webhook | `modules/webhook`, `message` | Gửi server alert thành message | P1 |
| 9 | Cronjob manager | CRUD cron job, schedule parser, next run, lock tránh chạy trùng | `modules/cronjob`, `scheduler` | Job chạy đúng lịch | P0 |
| 9 | Cron job runs | Log từng lần chạy, status, error, duration | `cron_job_runs` | Admin xem lịch sử job | P0 |
| 9 | Module runner MVP | Chạy HTTP API, bash/script được allowlist, log output | `modules/cronjob`, `platform/scheduler` | Chạy job mẫu ticket/server alert | P1 |
| 9 | Cleanup jobs | Cleanup session hết hạn, outbox dead, upload lỗi, old presence | `worker` | Job cleanup không xóa nhầm dữ liệu sống | P1 |
| 10 | Admin API | Dashboard stats, user/workspace/channel/bot/webhook management | `modules/admin` | Admin panel có API nền | P0 |
| 10 | Audit API | List/filter audit log, view before/after data | `modules/audit` | Filter theo actor/entity/action | P0 |
| 10 | Health deep check | Check DB, Redis, RabbitMQ, MinIO, queue depth | `modules/health` | `/ready` phản ánh phụ thuộc thật | P0 |
| 10 | Backup database | Job backup PostgreSQL, lưu local/MinIO/S3, metadata run | `modules/backup`, `deploy/scripts` | Tạo backup và ghi `backup_runs` | P0 |
| 10 | Restore test | Script restore dev/staging, tài liệu thao tác | `deploy/scripts`, `docs` | Restore backup thử thành công | P1 |
| 11 | Test suite | Unit domain/application, integration repository, API contract test | `backend/test`, từng module | CI chạy test ổn định | P0 |
| 11 | Security hardening | Rate limit, CORS, secure headers, JWT rotation, webhook signature | `middleware`, `shared/auth` | Endpoint nhạy cảm có bảo vệ | P0 |
| 11 | Performance | Index review, pagination, WebSocket load test, queue throughput | `test`, `docs/performance` | Có baseline latency/throughput | P1 |
| 11 | Observability | Prometheus metrics, structured log, tracing hook, dashboard mẫu | `platform/monitoring`, `deploy` | Có metric API/queue/ws | P1 |
| 11 | Error handling | Chuẩn hóa domain error, retryable/non-retryable, alert khi dead queue tăng | `shared/errors`, `rabbitmq` | Lỗi production dễ điều tra | P0 |
| 12 | Dockerfile | Multi-stage Dockerfile cho API và worker | `backend/Dockerfile` | Build image API/worker thành công | P0 |
| 12 | Compose production | Hoàn thiện compose prod, env, healthcheck, volumes, backup | `deploy/docker` | Server chạy bằng compose | P0 |
| 12 | GitHub Actions | CI, Docker build, deploy staging/production, environment approval | `.github/workflows` | Deploy từ GitHub Actions | P0 |
| 12 | Demo nội bộ | Deploy `chat.vpsttt.com`, tạo workspace demo, seed user/kênh | `deploy`, `db/seed` | Demo nội bộ dùng được | P0 |
| 12 | Tài liệu open-source backend | README tiếng Việt, Ubuntu/AlmaLinux guide, API/webhook/module docs | `README.md`, `docs` | Người khác tự deploy được | P1 |

## Thứ tự module nên làm

| Thứ tự | Module | Lý do ưu tiên |
|---|---|---|
| 1 | `health`, `config`, `shared`, `bootstrap` | Là nền để mọi module khác chạy được |
| 2 | `auth`, `users` | Cần xác thực trước khi có workspace/channel |
| 3 | `workspace`, `department`, `admin` nền | Cần tenant và quyền quản trị |
| 4 | `channel` và direct message | Cần kênh trước khi gửi message |
| 5 | `message` và WebSocket | Lõi sản phẩm chat |
| 6 | `file` | MVP cần upload ảnh/file |
| 7 | `notification`, `audit` | Bổ sung trải nghiệm realtime và truy vết |
| 8 | `api_token`, `webhook`, `bot` | Tích hợp hệ thống ngoài |
| 9 | `cronjob`, `backup` | Vận hành tự động |
| 10 | `health` nâng cao và monitoring | Chuẩn bị deploy thật |

## Milestone đề xuất

| Milestone | Nội dung | Backend có thể demo |
|---|---|---|
| M1 | API server, DB, Redis, RabbitMQ, MinIO, health, migration | Chạy được hạ tầng backend local |
| M2 | Auth, user, workspace, RBAC | Đăng nhập và tạo workspace |
| M3 | Channel, DM, message, WebSocket | Chat realtime 1-1/nhóm/kênh |
| M4 | File, notification, worker, outbox | Gửi file và nhận notification |
| M5 | Bot, token, webhook, cronjob | Tích hợp server alert/ticket mẫu |
| M6 | Admin, audit, backup, health deep check | Quản trị và vận hành backend |
| M7 | Hardening, CI/CD, Docker Compose production | Deploy server demo |

## Definition of Done cho mỗi phase

- Code theo đúng Clean Architecture của module.
- Có migration hoặc seed nếu thay đổi database.
- Có unit test cho domain/application quan trọng.
- Có integration test cho repository hoặc adapter quan trọng.
- Có API docs hoặc OpenAPI stub cho endpoint mới.
- Có log, error mapping và permission check.
- Nội dung log trong code phải viết bằng tiếng Việt có dấu.
- Có cập nhật tài liệu nếu thêm quy ước hoặc luồng mới.
- CI không đỏ vì thay đổi của phase đó.

## Những việc chưa làm trong backend trước khi qua frontend

- Chưa bắt đầu UI web/admin.
- Chỉ tạo OpenAPI, mock response hoặc Postman collection để frontend chuẩn bị sau.
- Chỉ làm seed/demo data tối thiểu để test backend.
- Chỉ tạo endpoint admin cần thiết; giao diện admin panel sẽ làm sau khi backend ổn.
