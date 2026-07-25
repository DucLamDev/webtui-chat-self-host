# Phase F0 - Chốt phạm vi và contract frontend

Phase F0 đã chốt các quyết định nền để các phase triển khai không phải đoán lại contract, route, env hoặc hướng UI.

## Trạng thái task

| Task | Trạng thái | Ghi chú |
|---|---|---|
| F0.1 | Hoàn thành | Đã đọc `frontend/ARCHITECTURE.md` và `.agents/webtui-chat-frontend/SKILL.md`. |
| F0.2 | Hoàn thành | Dùng `.agents/webtui-chat-frontend/references/backend-api-map.md` làm bản đồ API chính, đối chiếu thêm `backend/api/openapi/openapi.yaml` khi triển khai client. |
| F0.3 | Hoàn thành | MVP screen được khóa trong roadmap và screen backlog. |
| F0.4 | Hoàn thành | Route web/admin được chốt ở tài liệu này. |
| F0.5 | Hoàn thành | Env frontend mặc định trỏ backend đã deploy. |
| F0.6 | Hoàn thành | Token strategy có adapter rõ để F4 triển khai. |
| F0.7 | Hoàn thành | WebSocket browser dùng query `access_token`; subprotocol `webtui.jwt` là fallback. |
| F0.8 | Hoàn thành | Backend gaps được ghi lại để không chặn F1-F3. |
| F0.9 | Hoàn thành | Task board giữ theo mã phase trong `docs/planning/frontend-roadmap.md`. |

## Quyết định contract

- REST base URL mặc định: `https://chat.vpsttt.com`.
- WebSocket endpoint mặc định: `wss://chat.vpsttt.com/ws`.
- Không dùng backend `localhost` trong frontend MVP/production. Chỉ dùng local backend khi có yêu cầu backend-dev riêng.
- Tất cả request nghiệp vụ đi qua `packages/api-client`.
- Component React không gọi `fetch`, `axios` hoặc `WebSocket` trực tiếp.
- Response JSON dùng envelope `success`, `data`, `error`, `meta`, `request_id`, `timestamp`.
- File download trả binary và không unwrap envelope.
- UI gate bằng permission từ `/api/v1/rbac/me?workspace_id=...`, không suy từ tên role.

## Env frontend

```env
NEXT_PUBLIC_API_BASE_URL=https://chat.vpsttt.com
NEXT_PUBLIC_WS_BASE_URL=wss://chat.vpsttt.com/ws
NEXT_PUBLIC_APP_NAME=WebTui Chat
NEXT_PUBLIC_DEFAULT_LOCALE=vi-VN
```

## Token strategy

MVP dùng `AuthTokenStore` dạng adapter để có thể thay đổi nơi lưu token mà không sửa feature.

- Access token giữ trong memory store khi app đang chạy.
- Refresh token có thể lưu bằng adapter trình duyệt để khôi phục phiên sau reload.
- Logout phải xóa cả memory store, persistent store và Query cache.
- Không log token, URL có `access_token` hoặc nội dung header `Authorization`.
- Hướng hardening sau MVP: ưu tiên backend hỗ trợ refresh token bằng cookie `HttpOnly`, `Secure`, `SameSite=Lax`.

## WebSocket strategy

Frontend mặc định dùng:

```ts
new WebSocket(`wss://chat.vpsttt.com/ws?access_token=${encodeURIComponent(accessToken)}`);
```

Quy ước triển khai:

- Kết nối WebSocket chỉ nằm trong `RealtimeGateway`.
- Query token `access_token` là mặc định cho browser native WebSocket.
- Subprotocol `["webtui.jwt", accessToken]` chỉ là fallback cấu hình khi backend/proxy cần.
- Không ghi log URL WebSocket khi có token.
- Gateway phải có hook point cho reconnect, heartbeat và join/leave channel ở F3/F8.

## Route table web

| Route | Mục đích | Phase chính |
|---|---|---|
| `/login` | Đăng nhập người dùng | F4 |
| `/register` | Đăng ký tài khoản | F4 |
| `/workspaces` | Chọn hoặc tạo workspace | F5 |
| `/w/[workspaceId]` | Màn hình chat mặc định của workspace | F5-F6 |
| `/w/[workspaceId]/channels/[channelId]` | Kênh chat | F6-F8 |
| `/w/[workspaceId]/dm/[conversationId]` | Hội thoại riêng | F6-F8 |
| `/w/[workspaceId]/threads/[messageId]` | Thread hoặc chi tiết tin nhắn | F7-F8 |
| `/w/[workspaceId]/files/[fileId]` | Xem chi tiết file | F9 |
| `/w/[workspaceId]/notifications` | Danh sách thông báo | F10 |
| `/settings` | Cài đặt cá nhân | F10-F14 |

## Route table admin

| Route | Mục đích | Phase chính |
|---|---|---|
| `/login` | Đăng nhập admin | F4 |
| `/` | Dashboard tổng quan | F11 |
| `/workspaces` | Quản lý workspace | F11 |
| `/workspaces/[workspaceId]/members` | Thành viên workspace | F11 |
| `/workspaces/[workspaceId]/roles` | Vai trò và quyền | F11 |
| `/users` | Quản lý người dùng hệ thống | F11 |
| `/integrations/api-tokens` | API token | F12 |
| `/integrations/bots` | Bot | F12 |
| `/integrations/webhooks` | Incoming/outgoing webhook | F12 |
| `/operations/cronjobs` | Cronjob | F13 |
| `/operations/backups` | Backup | F13 |
| `/operations/health` | Health và readiness | F13 |
| `/audit` | Audit log | F11-F13 |
| `/settings` | Cấu hình hệ thống | F13-F14 |

## MVP screen scope

- Login/register.
- Chat shell gồm rail, danh sách kênh/hội thoại, timeline, composer và panel phải.
- Workspace switcher.
- Channel list, direct conversation list, member/presence cơ bản.
- Message timeline, reaction, thread cơ bản.
- File upload/download/attachment panel.
- Notification list/dropdown.
- Admin dashboard, user/member/RBAC, audit/health.
- Integration UI: API token, bot, webhook.
- Operations UI: cronjob, backup, health.

## Backend gaps cần theo dõi

| Gap | Ảnh hưởng | Hướng xử lý |
|---|---|---|
| User admin route cần RBAC backend | Admin UI có thao tác nhạy cảm | Backend kiểm `user.manage` theo `workspace_id`; frontend vẫn gate permission và hiển thị lỗi 403 rõ ràng. |
| Một số operation có thể thiếu trong OpenAPI | Sinh client tự động dễ thiếu endpoint | Route Go là nguồn sự thật khi có lệch. |
| Pin/ticket/task có thể chưa đủ API MVP | Panel mockup có phần chưa có dữ liệu thật | Làm placeholder rõ, không giả endpoint production. |
| Refresh token chưa có cookie `HttpOnly` | Token browser còn rủi ro XSS | Dùng adapter token, chuẩn bị đường đổi sang cookie. |
| CORS production cần xác nhận origin frontend | App local gọi `https://chat.vpsttt.com` có thể bị chặn | Nếu gặp CORS, sửa allowlist/proxy, không đổi base API sang backend local. |

## Quy tắc nội dung tiếng Việt

- Toàn bộ copy hiển thị cho người dùng phải dùng tiếng Việt có dấu.
- Tài liệu planning và handoff dùng tiếng Việt có dấu.
- Code identifier, route, biến môi trường, package name và thuật ngữ kỹ thuật giữ theo chuẩn kỹ thuật.

## Bàn giao cho F1

F1 triển khai npm workspaces, hai app Next.js, package dùng chung, env mẫu và shell tĩnh đủ đẹp để làm nền cho F2.
