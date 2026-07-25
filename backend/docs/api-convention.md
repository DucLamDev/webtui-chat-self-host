# Quy ước API backend

Backend dùng REST API qua Gin. Mọi endpoint cần giữ response, lỗi, phân trang và request id thống nhất để frontend, desktop, mobile và tích hợp ngoài hệ thống dùng chung một contract.

## Base path

- Public API: `/api/v1`
- Health endpoint: `/health`, `/ready`, `/version`
- WebSocket endpoint mục tiêu: `/ws`

## Response thành công

```json
{
  "success": true,
  "data": {},
  "meta": {},
  "request_id": "9f8a7c...",
  "timestamp": "2026-07-06T00:00:00Z"
}
```

## Response lỗi

```json
{
  "success": false,
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "Dữ liệu không hợp lệ.",
    "details": {
      "email": "Email không đúng định dạng."
    }
  },
  "request_id": "9f8a7c...",
  "timestamp": "2026-07-06T00:00:00Z"
}
```

## HTTP status

| Status | Khi dùng |
|---|---|
| `200` | Đọc hoặc cập nhật thành công |
| `201` | Tạo mới thành công |
| `202` | Nhận job bất đồng bộ |
| `204` | Xóa hoặc thao tác không cần body |
| `400` | Request sai định dạng hoặc sai rule cơ bản |
| `401` | Chưa xác thực hoặc token hết hạn |
| `403` | Không đủ quyền |
| `404` | Không tìm thấy tài nguyên |
| `409` | Xung đột dữ liệu hoặc idempotency |
| `422` | Validation nghiệp vụ không đạt |
| `429` | Rate limit |
| `500` | Lỗi hệ thống |

## Error code

Error code dùng `UPPER_SNAKE_CASE`, ổn định để client xử lý.

Ví dụ:

- `VALIDATION_ERROR`
- `UNAUTHORIZED`
- `FORBIDDEN`
- `RESOURCE_NOT_FOUND`
- `WORKSPACE_NOT_FOUND`
- `CHANNEL_NOT_FOUND`
- `MESSAGE_NOT_FOUND`
- `PERMISSION_DENIED`
- `RATE_LIMITED`
- `INTERNAL_ERROR`

## Pagination

Danh sách realtime như message timeline dùng cursor:

```text
GET /api/v1/channels/{channel_id}/messages?limit=50&before=cursor
```

Response meta:

```json
{
  "next_cursor": "cursor",
  "has_more": true
}
```

Danh sách admin có thể dùng page/page_size khi không cần realtime.

## Filter và sort

- Filter dùng query string rõ tên: `status=active`, `role=admin`.
- Sort dùng `sort=created_at_desc` hoặc `sort=name_asc`.
- Không nhận raw SQL field từ client.

## Request id

- Client có thể gửi `X-Request-ID`.
- Nếu client không gửi, middleware sẽ tự tạo.
- Mọi log và response phải có request id.

## Quy tắc ngôn ngữ cho log

- Mọi nội dung log trong code phải viết bằng tiếng Việt có dấu.
- Nghiêm cấm dùng tiếng Việt không dấu trong nội dung log, ví dụ không dùng `khong doc duoc cau hinh`.
- Structured log key như `error`, `request_id`, `status`, `latency_ms` được giữ ổn định để máy đọc và truy vấn log.
- Response lỗi trả cho người dùng cũng phải viết bằng tiếng Việt có dấu nếu dùng tiếng Việt.

## Idempotency

Endpoint ghi dữ liệu có nguy cơ retry như gửi message, upload file, webhook nên hỗ trợ `Idempotency-Key` ở phase nghiệp vụ tương ứng.
