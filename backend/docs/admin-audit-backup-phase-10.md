# Admin, audit, health và backup

Tài liệu này mô tả phase 10 của backend WebTui Chat: API quản trị, audit log, health check sâu và backup database.

## Admin API

Các endpoint admin yêu cầu JWT và quyền `admin.view` trong workspace.

```text
GET /api/v1/workspaces/{workspace_id}/admin/stats
GET /api/v1/workspaces/{workspace_id}/admin/health
```

`/admin/stats` trả các chỉ số tổng quan của workspace:

- Thành viên active.
- Channel.
- Message.
- File.
- Bot.
- Incoming/outgoing webhook.
- Audit log.
- Backup job.

`/admin/health` chạy deep check có phân quyền, gồm API, database, Redis, RabbitMQ, storage và WebSocket nếu dependency được bật.

Các thao tác quản lý user, workspace, channel, bot và webhook vẫn dùng API module hiện có với permission tương ứng. Phase 10 bổ sung lớp dashboard và vận hành, không tạo lại CRUD trùng với từng module nghiệp vụ.

## Audit API

Audit API yêu cầu quyền `audit.view`.

```text
GET /api/v1/workspaces/{workspace_id}/audit-logs
```

Query hỗ trợ:

- `actor_user_id`
- `action`
- `entity_type`
- `entity_id`
- `from` và `to` theo RFC3339
- `limit`

Response trả cả `before_data`, `after_data` và `metadata` để điều tra thay đổi.

## Backup database

Backup API yêu cầu quyền `backup.manage`.

```text
GET /api/v1/workspaces/{workspace_id}/backup-jobs
POST /api/v1/workspaces/{workspace_id}/backup-jobs
GET /api/v1/workspaces/{workspace_id}/backup-jobs/{backup_job_id}/runs
POST /api/v1/workspaces/{workspace_id}/backup-jobs/{backup_job_id}/run
```

MVP hiện hỗ trợ `backup_type = database`. Backend chạy `pg_dump` cố định qua cấu hình `BACKUP_PG_DUMP_PATH`, không nhận command tùy ý từ request.

Ví dụ tạo backup job:

```json
{
  "name": "Backup database mỗi ngày",
  "target": "minio",
  "backup_type": "database",
  "schedule": "@daily",
  "status": "active",
  "config": {}
}
```

Khi chạy backup:

- Tạo bản ghi `backup_runs` trạng thái `running`.
- Chạy `pg_dump --format=custom --no-owner --no-privileges`.
- Lưu object vào storage hiện tại với key dạng `backups/database/{workspace_id}/{yyyy/mm/dd/hhmmss}-{job_id}.dump`.
- Ghi `byte_size`, `checksum_sha256`, `object_key`, `finished_at`.
- Nếu lỗi, ghi `status = failed` và `error`.

Worker có thêm task:

```text
backup_jobs
```

Task này chạy mỗi 60 giây, claim backup job đến hạn bằng `next_run_at`, đặt `locked_at/locked_by` để tránh chạy trùng và cập nhật lịch kế tiếp sau mỗi lần chạy.

## Cấu hình

```env
BACKUP_PG_DUMP_PATH=pg_dump
BACKUP_TIMEOUT=10m
```

Nếu dùng Docker image production, cần bảo đảm image có binary `pg_dump` tương thích PostgreSQL đang chạy.

## Restore thử

Backup được tạo ở định dạng custom của PostgreSQL, restore bằng `pg_restore`.

Ví dụ restore vào database dev mới:

```powershell
createdb webtui_restore_test
pg_restore --clean --if-exists --no-owner --no-privileges --dbname "postgres://postgres:123456@localhost:5432/webtui_restore_test?sslmode=disable" "duong_dan_file.dump"
```

Có thể dùng script trong repo:

```powershell
.\deploy\scripts\restore-postgres.ps1 `
  -RestoreDatabaseUrl "postgres://postgres:123456@localhost:5432/webtui_restore_test?sslmode=disable" `
  -RestoreDumpFile "duong_dan_file.dump"
```

```bash
RESTORE_DATABASE_URL="postgres://postgres:123456@localhost:5432/webtui_restore_test?sslmode=disable" \
RESTORE_DUMP_FILE="duong_dan_file.dump" \
deploy/scripts/restore-postgres.sh
```

Sau restore, chạy API với `DATABASE_URL` trỏ vào database restore test và kiểm tra `/ready`, login, workspace, channel, message mẫu.

## Lưu ý an toàn

- Không log nội dung dump hoặc secret database.
- Không dùng backup chưa restore thử làm căn cứ an toàn dữ liệu.
- Nên lưu backup ra storage ngoài server chính.
- Nên giới hạn quyền `backup.manage` cho owner/admin vận hành.
