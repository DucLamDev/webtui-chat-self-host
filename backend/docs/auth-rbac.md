# Auth, user và RBAC

Tài liệu này mô tả phần nền của phase 3 cho backend WebTui Chat.

## Endpoint auth

Base path: `/api/v1/auth`

| Endpoint | Mục đích | Auth |
|---|---|---|
| `POST /register` | Tạo user mới và trả access/refresh token | Không |
| `POST /login` | Đăng nhập bằng email hoặc username | Không |
| `POST /refresh` | Rotate refresh token và cấp access token mới | Không |
| `POST /logout` | Thu hồi refresh token hiện tại | Không |
| `GET /me` | Lấy thông tin user hiện tại | Có |
| `GET /sessions` | Xem danh sách phiên đăng nhập | Có |
| `DELETE /sessions/{session_id}` | Thu hồi một phiên đăng nhập | Có |
| `DELETE /sessions` | Thu hồi toàn bộ phiên đăng nhập của user hiện tại | Có |

## Endpoint user

Base path: `/api/v1/users`

| Endpoint | Mục đích | Auth |
|---|---|---|
| `GET /me` | Lấy hồ sơ user hiện tại | Có |
| `PATCH /me` | Cập nhật `display_name`, `avatar_url`, `locale`, `timezone` | Có |
| `GET /` | Danh sách user, hỗ trợ `q`, `status`, `limit` | Có |
| `GET /{user_id}` | Xem hồ sơ user theo id | Có |

## Endpoint RBAC

Base path: `/api/v1/rbac`

| Endpoint | Mục đích | Auth |
|---|---|---|
| `GET /permissions` | Xem danh mục permission hệ thống | Có |
| `GET /roles?workspace_id=...` | Xem role hệ thống và role riêng của workspace | Có |
| `POST /roles` | Tạo role riêng cho workspace | Có |
| `GET /me?workspace_id=...` | Xem permission của user trong workspace | Có |
| `GET /check?workspace_id=...&permission=...` | Kiểm tra user có permission hay không | Có |
| `GET /workspaces/{workspace_id}/members/{user_id}/roles` | Xem role của một thành viên workspace | Có |
| `POST /workspaces/{workspace_id}/members/{user_id}/roles` | Gán role cho thành viên workspace | Có |
| `DELETE /workspaces/{workspace_id}/members/{user_id}/roles/{role_id}` | Gỡ role khỏi thành viên workspace | Có |

## Quy tắc token

- Access token là JWT ký bằng HMAC SHA-256.
- Refresh token là chuỗi opaque, chỉ lưu hash HMAC SHA-256 trong bảng `user_sessions`.
- Khi gọi `/refresh`, refresh token cũ bị thay bằng refresh token mới.
- Production bắt buộc dùng secret riêng, dài tối thiểu 32 ký tự và không dùng giá trị `change_me` hoặc secret dev.

## Thiết bị và IP

- `device_name` trong request register/login là tùy chọn.
- Nếu client không gửi `device_name`, backend tự suy ra từ `User-Agent`, ví dụ `Windows - Chrome`, `Android - Chrome`, `iPhone - Safari`.
- IP được lấy tự động từ `CF-Connecting-IP`, `X-Real-IP`, `X-Forwarded-For` hoặc fallback về IP mà Gin đọc được từ request.
- Bảng `user_sessions` lưu lịch sử từng phiên đăng nhập với `device_name`, `ip_address`, `user_agent`.
- Bảng `users` lưu nhanh `registration_device_name`, `registration_ip_address`, `device_name`, `last_ip_address` để truy vấn hồ sơ và lần đăng nhập gần nhất.

## RBAC phase 3

Migration `000002_seed_rbac_defaults` seed sẵn:

- Role hệ thống: `workspace_owner`, `workspace_admin`, `workspace_member`.
- Permission nền: `workspace.manage`, `workspace.invite_user`, `workspace.view_members`, `channel.create`, `channel.manage`, `message.send`, `file.upload`, `bot.manage`, `api_token.manage`, `webhook.manage`, `module.manage`, `audit.view`, `backup.manage`, `admin.view`, `notification.manage`, `cronjob.manage`.

Trong phase 3, RBAC đã có service kiểm tra quyền theo `workspace_id`, API tạo role riêng và API gán/gỡ role cho workspace member. Phase 4 dùng service này cho workspace, department, channel và direct message.
