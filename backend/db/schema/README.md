# Schema PostgreSQL

Thư mục này chứa thiết kế schema tham chiếu cho PostgreSQL.

- `erd.mmd`: ERD dạng Mermaid để xem nhanh quan hệ bảng.
- `../migrations/000001_initial_schema.up.sql`: migration tạo schema nền.
- `../migrations/000001_initial_schema.down.sql`: migration rollback schema nền trong môi trường dev/test.

Quy ước:

- Dùng `uuid` cho khóa chính.
- Dùng `citext` cho email, username và slug cần so khớp không phân biệt hoa thường.
- Dùng `jsonb` cho metadata, payload event và cấu hình linh hoạt.
- Dùng soft delete qua `deleted_at` ở bảng nghiệp vụ cần giữ lịch sử.
- Dữ liệu phát event sang RabbitMQ đi qua bảng `outbox_events` trước.
- `messages` partition theo hash `workspace_id` và dùng khóa chính composite `(workspace_id, id)`.
- Quyền động dùng RBAC qua `roles`, `permissions` và các bảng gán role.
