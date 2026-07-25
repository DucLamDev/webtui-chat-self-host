# Security baseline backend

Đây là checklist bảo mật bắt buộc cho backend WebTui Chat.

## Auth và session

- Password phải hash bằng thuật toán chậm như Argon2id hoặc bcrypt.
- Không lưu refresh token dạng plain text; chỉ lưu hash.
- Access token ngắn hạn.
- Refresh token có thể revoke theo session hoặc toàn bộ user.
- Production không được dùng secret mặc định trong `.env.example`.
- Đăng nhập thất bại phải ghi audit `auth.login_failed`; chỉ lưu SHA-256 của định danh, không lưu password hoặc định danh dạng rõ.
- Hệ thống không seed tài khoản/mật khẩu admin mặc định. Workspace owner đăng ký bằng mật khẩu tự chọn, vì vậy không tồn tại mật khẩu admin ban đầu cần sử dụng qua lần đăng nhập đầu.

## API token

- API token chỉ hiển thị một lần khi tạo.
- Database chỉ lưu `token_hash`.
- Scope token quản lý qua `api_scopes` và `api_token_scopes`.
- Mọi lần dùng token cần cập nhật `last_used_at`.

## Webhook

- Incoming webhook cần secret riêng.
- Outgoing webhook cần ký request bằng HMAC.
- Webhook delivery phải có retry, timeout và log trạng thái.
- Payload không chứa password, token hoặc secret.

## Middleware bắt buộc

- Request id.
- Panic recovery.
- Access log.
- Auth middleware cho route private.
- Permission middleware cho admin, workspace, channel và bot/webhook.
- Rate limit cho auth, upload, webhook và API token.
- CORS chỉ cho phép origin trong `CORS_ALLOWED_ORIGINS`.
- Security headers phải bật mặc định qua `SECURE_HEADERS_ENABLED=true`.
- Endpoint `/metrics` chỉ xuất metric kỹ thuật, không xuất token, email riêng tư hoặc payload nghiệp vụ.

## Dữ liệu workspace

- Upload cần quyền `file.upload`; xem/tải file cần quyền workspace và kiểm tra `CanAccessFile` theo owner hoặc message attachment trong kênh mà user tham gia.
- API kênh chỉ trả danh sách, chi tiết và thành viên kênh cho user có membership `active` hoặc `muted`.
- Kênh `ban-giam-doc` và `ke-toan` được provision dạng private; thành viên mới không tự động được thêm vào hai kênh này.
- API token/bot integration bắt buộc có ít nhất một scope và được kiểm tra scope ở thời điểm authenticate.

## Log

- Không log password, token, refresh token, API token hoặc webhook secret.
- Log lỗi phải có `request_id`.
- Audit log cần ghi `before_data` và `after_data` cho thao tác quản trị quan trọng.
- Mọi nội dung log phải viết bằng tiếng Việt có dấu.
- Nghiêm cấm sử dụng tiếng Việt không dấu trong nội dung log.
- Structured log key được phép dùng tiếng Anh hoặc snake_case ổn định để phục vụ tìm kiếm, dashboard và alert.

## Production checklist

- `APP_ENV=production`.
- Secret dài ít nhất 32 ký tự.
- TLS ở Nginx.
- CORS chỉ cho domain được phép.
- Docker volume backup định kỳ.
- Compose production chỉ publish cổng `80` và `443`; database, Redis, worker và object storage chỉ nằm trong network nội bộ.
- Local storage dùng thư mục mode `0700` và object mode `0600`; bucket MinIO/S3 phải để private.
- Health check và alert đang bật.
