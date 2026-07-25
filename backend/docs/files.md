# File, version và attachment

Phase 6 bổ sung module `files` để backend có thể upload, tải xuống và gắn file vào tin nhắn.

## Thành phần

- `domain`: entity file, version và attachment.
- `application`: kiểm tra quyền, validate MIME, giới hạn dung lượng, tính checksum SHA-256.
- `infrastructure/postgres`: lưu `files`, `file_versions`, `message_attachments`.
- `infrastructure/storage`: adapter bọc `platform/storage.Store`.
- `delivery/http`: REST API upload/download/attachment.

## Endpoint chính

| Method | Path | Mục đích |
|---|---|---|
| `GET` | `/api/v1/workspaces/{workspace_id}/files` | Danh sách file trong workspace |
| `POST` | `/api/v1/workspaces/{workspace_id}/files` | Upload file bằng multipart form |
| `GET` | `/api/v1/workspaces/{workspace_id}/files/{file_id}` | Xem metadata file |
| `GET` | `/api/v1/workspaces/{workspace_id}/files/{file_id}/download` | Tải nội dung file |
| `GET` | `/api/v1/workspaces/{workspace_id}/files/{file_id}/versions` | Danh sách version |
| `POST` | `/api/v1/workspaces/{workspace_id}/files/{file_id}/versions` | Upload version mới |
| `GET` | `/api/v1/workspaces/{workspace_id}/channels/{channel_id}/messages/{message_id}/attachments` | Danh sách attachment của tin nhắn |
| `POST` | `/api/v1/workspaces/{workspace_id}/channels/{channel_id}/messages/{message_id}/attachments` | Gắn file vào tin nhắn |

## Upload

Upload dùng `multipart/form-data`:

- Field `file`: file cần upload.
- Field `metadata`: JSON string tùy chọn.

Backend kiểm tra:

- User phải có quyền `file.upload`.
- Dung lượng lớn hơn `0` và không vượt quá `100MB`.
- MIME type thuộc nhóm được hỗ trợ, ví dụ `image/*`, `text/*`, `application/pdf`, `application/json`, `application/zip`, Office document hoặc `application/octet-stream`.
- Các MIME nguy hiểm như file thực thi Windows sẽ bị chặn.

Khi upload thành công, backend:

1. Lưu object vào storage.
2. Tính `checksum_sha256`.
3. Ghi bản ghi vào `files`.
4. Tạo `file_versions` version `1`.

## Cấu hình MinIO dev

Dùng các biến môi trường sau để bật MinIO/S3 dev:

```powershell
$env:STORAGE_PROVIDER="minio"
$env:MINIO_BUCKET="bucket-duclamdev"
$env:S3_ENDPOINT="https://minio1.webtui.vn:9000"
$env:S3_REGION="vn-1"
$env:S3_ACCESS_KEY_ID="duclamdev"
$env:S3_SECRET_ACCESS_KEY="<mật khẩu MinIO dev>"
```

Có thể tạo file `.env.minio.dev` bị `.gitignore` bỏ qua để lưu cấu hình dev local. Nếu chạy bằng PowerShell, cần nạp các biến này vào session trước khi `go run ./cmd/api`.

```powershell
Get-Content ..\.env.minio.dev | ForEach-Object {
  if ($_ -and -not $_.StartsWith("#")) {
    $name, $value = $_.Split("=", 2)
    Set-Item -Path "Env:$name" -Value $value
  }
}
```

## Download

Download yêu cầu user có quyền thành viên workspace qua quyền `message.send`. Nội dung file được stream qua response, không đọc toàn bộ file vào RAM.

## Version

Khi upload version mới, backend:

- Tạo bản ghi mới trong `file_versions`.
- Tăng `version_number` dựa trên version lớn nhất hiện có.
- Cập nhật metadata hiện tại trên bảng `files` để trỏ tới object mới nhất.

## Attachment

Gắn file vào message yêu cầu:

- User có quyền `message.send` trong workspace.
- User là member active/muted của channel chứa message.
- File thuộc cùng workspace và chưa bị xóa.

Bảng `message_attachments` dùng khóa chính `(workspace_id, message_id, file_id)`, nên gắn lại cùng file sẽ cập nhật `sort_order` thay vì tạo trùng.
