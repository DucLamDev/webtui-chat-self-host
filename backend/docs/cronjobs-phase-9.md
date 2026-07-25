# Cronjob và module runner

Tài liệu này mô tả phase 9 của backend WebTui Chat: quản lý cronjob, chạy job định kỳ bằng worker, ghi lịch sử chạy và cung cấp runner có kiểm soát.

## Quyền truy cập

Các endpoint cronjob yêu cầu JWT và quyền `cronjob.manage` trong workspace truyền trên path.

Cronjob hiện là cấu hình vận hành cấp hệ thống theo schema `cron_jobs`, chưa gắn trực tiếp với `workspace_id`. Workspace trên path được dùng để kiểm tra quyền admin trước khi thao tác.

## Endpoint

```text
GET /api/v1/workspaces/{workspace_id}/cronjobs
POST /api/v1/workspaces/{workspace_id}/cronjobs
PATCH /api/v1/workspaces/{workspace_id}/cronjobs/{cronjob_id}
DELETE /api/v1/workspaces/{workspace_id}/cronjobs/{cronjob_id}
GET /api/v1/workspaces/{workspace_id}/cronjobs/{cronjob_id}/runs
POST /api/v1/workspaces/{workspace_id}/cronjobs/{cronjob_id}/run
```

## Lịch chạy

Backend hỗ trợ các kiểu lịch:

- `@every 15m`, `@every 1h`, `@every 24h`.
- `@hourly`, `@daily`, `@weekly`.
- Cron expression 5 trường: `phút giờ ngày tháng thứ`, ví dụ `*/5 * * * *`.

Lịch `@every` tối thiểu là `10s` và tối đa là `366 ngày`.

## Runner allowlist

Runner được allowlist để tránh backend chạy lệnh tùy ý.

### `http`

Gọi một HTTP endpoint nội bộ hoặc hệ thống ngoài.

```json
{
  "name": "Gửi health ping",
  "schedule": "@every 15m",
  "runner": "http",
  "status": "active",
  "payload": {
    "method": "POST",
    "url": "http://localhost:8080/health",
    "headers": {
      "X-WebTui-Job": "health-ping"
    },
    "body": {
      "source": "cronjob"
    }
  }
}
```

### `builtin_cleanup`

Chạy các tác vụ cleanup đã định nghĩa sẵn.

```json
{
  "name": "Dọn session hết hạn",
  "schedule": "@every 1h",
  "runner": "builtin_cleanup",
  "status": "active",
  "payload": {
    "task": "cleanup_expired_sessions",
    "older_than": "168h"
  }
}
```

Các task hiện có:

- `cleanup_expired_sessions`: xóa session đã hết hạn hoặc đã revoke sau thời gian giữ lại.
- `cleanup_dead_outbox`: xóa outbox event đã `published` hoặc `dead` quá cũ.
- `cleanup_failed_uploads`: đánh dấu file `uploading` hoặc `failed` quá cũ thành `deleted`.
- `cleanup_old_presence`: xóa presence `offline` quá cũ.

`older_than` là duration của Go như `24h`, `168h`, `720h`; tối thiểu `1h`. Nếu không truyền, backend dùng retention mặc định theo từng task.

### `script`

Chạy script đã được cấu hình trước trong allowlist. Request chỉ được truyền tên script, tham số và timeout; backend không chạy command tùy ý và không đi qua shell.

Cấu hình allowlist:

```env
MODULE_RUNNER_SCRIPT_ALLOWLIST=ticket=deploy/scripts/ticket-alert.ps1;server_alert=deploy/scripts/server-alert.sh
```

Payload:

```json
{
  "name": "Gửi cảnh báo ticket",
  "schedule": "@every 30m",
  "runner": "script",
  "status": "paused",
  "payload": {
    "name": "ticket",
    "args": ["--severity", "warning"],
    "timeout": "30s"
  }
}
```

Giới hạn:

- Script phải nằm trong `MODULE_RUNNER_SCRIPT_ALLOWLIST`.
- Tối đa 20 tham số.
- Mỗi tham số tối đa 256 ký tự và không được chứa ký tự điều khiển dòng.
- Timeout tối đa 10 phút.

## Worker

Worker có thêm task:

```text
cronjobs
```

Task này chạy mỗi 15 giây, claim các job `active` có `next_run_at <= now()`, đặt `locked_at/locked_by` để tránh nhiều worker chạy trùng, sau đó tạo bản ghi `cron_job_runs`.

Khi job hoàn tất:

- `cron_job_runs.status` là `success` hoặc `failed`.
- `cron_job_runs.log` lưu output ngắn.
- `cron_job_runs.error` lưu lỗi nếu có.
- `cron_jobs.last_run_at` và `cron_jobs.next_run_at` được cập nhật.
- Lock được giải phóng.

## Lưu ý vận hành

- Không lưu secret nhạy cảm trực tiếp trong payload cronjob nếu không thật sự cần. Nếu phải gọi endpoint có secret, ưu tiên dùng endpoint nội bộ hoặc token có scope hẹp.
- Runner script chỉ chạy tên nằm trong allowlist, không nhận command tự do từ request.
- Với job HTTP gọi ra ngoài internet, nên đặt target idempotent vì worker có thể retry sau lỗi hoặc lock hết hạn.
