# Workspace, phòng ban, kênh và direct message

Tài liệu này mô tả phần nền phase 4 của backend WebTui Chat.

## Workspace

Base path: `/api/v1/workspaces`

| Endpoint | Mục đích |
|---|---|
| `GET /` | Danh sách workspace của user hiện tại |
| `POST /` | Tạo workspace mới |
| `GET /{workspace_id}` | Xem chi tiết workspace |
| `PATCH /{workspace_id}` | Cập nhật tên, mô tả workspace |
| `DELETE /{workspace_id}` | Xóa mềm hoặc archive workspace |
| `GET /{workspace_id}/members` | Danh sách thành viên |
| `POST /{workspace_id}/members` | Thêm user vào workspace và gán role |
| `PATCH /{workspace_id}/members/{user_id}` | Cập nhật trạng thái thành viên |
| `GET /{workspace_id}/settings` | Danh sách setting |
| `PUT /{workspace_id}/settings/{key}` | Tạo hoặc cập nhật setting |
| `GET /{workspace_id}/invites` | Danh sách lời mời |
| `POST /{workspace_id}/invites` | Tạo lời mời và trả token một lần |

Khi tạo workspace, backend tự:

- Tạo bản ghi `workspaces`.
- Thêm người tạo vào `workspace_members`.
- Gán role hệ thống `workspace_owner` cho người tạo.
- Tạo 9 kênh mặc định: `thong-bao`, `ban-giam-doc`, `ky-thuat`, `sale`, `ke-toan`, `ticket`, `server-alert`, `gia-han`, `ban-giao-ca`. Hai kênh nhạy cảm `ban-giam-doc` và `ke-toan` là private; các kênh còn lại là public.
- Tạo và cài `Ticket Bot`, `Server Alert Bot`, `Gia Hạn Bot` lần lượt vào kênh `ticket`, `server-alert`, `gia-han`.
- Ghi audit `workspace.create`.

Toàn bộ bước trên chạy trong cùng một database transaction. Nếu một kênh, bot
hoặc installation không tạo được thì workspace cũng không được ghi dở dang.

## Phòng ban

Base path: `/api/v1/workspaces/{workspace_id}/departments`

| Endpoint | Mục đích |
|---|---|
| `GET /` | Danh sách phòng ban |
| `POST /` | Tạo phòng ban |
| `GET /{department_id}` | Xem chi tiết phòng ban |
| `PATCH /{department_id}` | Cập nhật phòng ban |
| `DELETE /{department_id}` | Xóa mềm phòng ban |
| `GET /{department_id}/members` | Danh sách thành viên phòng ban |
| `POST /{department_id}/members` | Gán user vào phòng ban |
| `DELETE /{department_id}/members/{user_id}` | Gỡ user khỏi phòng ban |

Vai trò phòng ban hiện hỗ trợ `lead` và `member`.

## Kênh

Base path: `/api/v1/workspaces/{workspace_id}/channels`

| Endpoint | Mục đích |
|---|---|
| `GET /` | Danh sách kênh |
| `POST /` | Tạo kênh public/private |
| `GET /{channel_id}` | Xem chi tiết kênh |
| `PATCH /{channel_id}` | Cập nhật kênh |
| `DELETE /{channel_id}` | Archive kênh |
| `GET /{channel_id}/members` | Danh sách thành viên kênh |
| `POST /{channel_id}/members` | Thêm user vào kênh |
| `PATCH /{channel_id}/members/{user_id}` | Cập nhật trạng thái thành viên kênh |
| `PUT /{channel_id}/read-state` | Cập nhật `last_read_at` và `last_read_message_id` |

Loại kênh hiện hỗ trợ `public` và `private`.

API danh sách, chi tiết và danh sách thành viên kênh chỉ trả dữ liệu khi user là
thành viên active/muted của kênh. Khi thêm một user vào workspace, backend tự
thêm user đó vào các kênh public đang active.

## Direct message

Base path: `/api/v1/workspaces/{workspace_id}/direct-conversations`

| Endpoint | Mục đích |
|---|---|
| `GET /` | Danh sách direct conversation của user hiện tại |
| `POST /` | Tạo hoặc lấy lại DM theo danh sách participant |

Backend sort participant id và tạo `participant_key`, nên cùng một nhóm participant sẽ không tạo trùng direct conversation.

## Quyền dùng trong phase 4

- Tạo kênh: `channel.create`.
- Quản lý kênh/member kênh: `channel.manage`.
- Archive kênh: `channel.delete`.
- Tạo/gửi DM: `message.send`.
- Quản lý workspace, department, setting: `workspace.manage`.
- Mời/thêm member workspace: `workspace.invite_user`.
- Xem member workspace: `workspace.view_members`.
