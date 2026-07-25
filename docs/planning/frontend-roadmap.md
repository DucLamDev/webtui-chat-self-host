# Kế hoạch triển khai frontend WebTui Chat

Tài liệu này chia nhỏ kế hoạch frontend thành các phase và task có thể đưa thẳng vào backlog. Phạm vi bám theo backend Go + Gin đã có, agent skill `.agents/webtui-chat-frontend`, theme mockup xanh-trắng và mục tiêu sản phẩm chat nội bộ tự host cho doanh nghiệp Việt.

## Mục tiêu frontend

- Có `apps/web` cho người dùng chat hằng ngày.
- Có `apps/admin` cho quản trị workspace, user, RBAC, tích hợp và vận hành.
- Dùng Next.js App Router, TypeScript, Tailwind, shadcn/ui, TanStack Query, Zustand.
- Dùng API thật từ backend qua `packages/api-client`, không gọi API trực tiếp trong component.
- Mọi môi trường frontend mặc định dùng backend đã deploy: `NEXT_PUBLIC_API_BASE_URL=https://chat.vpsttt.com` và `NEXT_PUBLIC_WS_BASE_URL=wss://chat.vpsttt.com/ws`, không dùng `localhost` nếu không có yêu cầu backend-dev riêng.
- Bám theme ảnh số 3: rail xanh đậm, vùng chat sáng, panel phải cho ghim/ảnh/file, admin dense và dễ quét.
- Chuẩn bị đường mở rộng desktop app Tauri và mobile Flutter sau MVP web.

## Nguyên tắc triển khai

- Làm web app thật trước, không làm landing page trong phase MVP.
- Tách rõ `apps/*`, `features/*`, `packages/*`.
- Server state dùng TanStack Query; client UI state dùng Zustand.
- UI gate bằng permission từ `/api/v1/rbac/me`, không suy từ role name.
- WebSocket browser dùng query `?access_token=...` vì native WebSocket không set được header Authorization.
- Không log URL có `access_token`; subprotocol `["webtui.jwt", accessToken]` chỉ là fallback khi cần tránh token trong URL.
- Mỗi task phải có acceptance rõ, ưu tiên test mapper/API client/permission/realtime cache.
- Nếu backend API chưa có, không giả endpoint production; ghi rõ mock/placeholder.

## Mốc MVP frontend

Frontend đạt MVP khi có đủ:

- Login/register/logout/refresh token.
- Chọn workspace và load permission.
- Xem danh sách kênh, hội thoại riêng, member/presence cơ bản.
- Xem/gửi/sửa/xóa message, reaction, thread cơ bản.
- Realtime WebSocket nhận message giữa hai browser.
- Upload/download file và gắn file vào message.
- Notification mention và mark read.
- Admin dashboard cơ bản, user/member/RBAC, audit/health.
- API token, bot, webhook, cronjob, backup ở mức quản trị nền.

## Bảng phase tổng quan

Trạng thái hiện tại:

- F0: hoàn thành, xem `docs/planning/frontend-phase-f0.md`.
- F1: hoàn thành, xem `docs/planning/frontend-phase-f1.md`.
- F2: hoàn thành, xem `docs/planning/frontend-phase-f2.md`.
- F3: hoàn thành, xem `.agents/webtui-chat-frontend/references/frontend-implementation-status.md`.
- F4: hoàn thành, xem `.agents/webtui-chat-frontend/references/frontend-implementation-status.md`.
- F5: hoàn thành P0, xem `.agents/webtui-chat-frontend/references/frontend-implementation-status.md`.
- F6: hoàn thành P0/P1 chính, xem `.agents/webtui-chat-frontend/references/frontend-implementation-status.md`.
- F7: hoàn thành P0/P1 chính, xem `.agents/webtui-chat-frontend/references/frontend-implementation-status.md`.
- F8: hoàn thành P0 chính, xem `.agents/webtui-chat-frontend/references/frontend-implementation-status.md`.
- F9: hoàn thành P0/P1 chính, xem `.agents/webtui-chat-frontend/references/frontend-implementation-status.md`.
- F10: hoàn thành P0/P1 chính, xem `.agents/webtui-chat-frontend/references/frontend-implementation-status.md`.

| Phase | Tên phase | Mục tiêu | Kết quả bàn giao | Điều kiện chuyển phase |
|---|---|---|---|---|
| F0 | Chốt phạm vi và contract | Khóa API, theme, route, env, rủi ro | Roadmap, route map, task board | Không còn câu hỏi lớn về MVP |
| F1 | Nền monorepo frontend | Tạo app/package/config | Next.js web/admin chạy được | `dev`, `lint`, `typecheck` có script |
| F2 | Design system và app shell | Dựng theme giống mockup | UI shell xanh-trắng responsive | Có layout desktop/tablet/mobile |
| F3 | API client và type nền | Bọc REST/WebSocket/upload/download | `packages/api-client`, `packages/types` | Gọi API health/auth mock thật được |
| F4 | Auth và session | Đăng nhập, refresh, bảo vệ route | Login/register/session restore | Vào app sau login ổn định |
| F5 | Workspace và RBAC | Chọn workspace, load permission | Workspace switcher, permission gate | UI action đúng quyền |
| F6 | Channel và hội thoại | Sidebar kênh/DM/member | Channel list, direct list | Chọn channel/DM và route đúng |
| F7 | Message timeline REST | Chat text qua REST | Timeline, composer, CRUD message | Gửi message text thành công |
| F8 | Realtime WebSocket | Nhận event live | Realtime gateway, cache merge | Hai browser nhận message |
| F9 | File và attachment | Upload/download/file panel | Upload, attach, download blob | Message có file dùng được |
| F10 | Notification và presence | Trạng thái online/thông báo | Notification list, heartbeat | Mention notification hiển thị |
| F11 | Admin MVP | Dashboard/user/RBAC/audit/health | `apps/admin` dùng được | Admin quản trị workspace cơ bản |
| F12 | Integration UI | API token/bot/webhook | Màn hình tích hợp | Gửi message qua token/webhook test được |
| F13 | Operations UI | Cronjob/backup/health nâng cao | Màn vận hành | Run cronjob/backup từ UI |
| F14 | UX hardening và responsive | Tối ưu polish | Loading, empty, error, mobile | Dùng mượt như app nội bộ |
| F15 | Test, CI, release web | Kiểm thử và đóng gói | CI, E2E, Docker/env | Sẵn sàng deploy demo |

## Phase F0: Chốt phạm vi và contract

| Task | Công việc | Phụ thuộc | Kết quả | Acceptance | Ưu tiên |
|---|---|---|---|---|---|
| F0.1 | Đọc `frontend/ARCHITECTURE.md` và `.agents/webtui-chat-frontend/SKILL.md` | Không | Checklist kiến trúc frontend | Team thống nhất hướng phụ thuộc | P0 |
| F0.2 | Đối chiếu `backend-api-map.md` với `openapi.yaml` | Không | Danh sách endpoint frontend sẽ dùng | Không bỏ sót API MVP | P0 |
| F0.3 | Chốt MVP screens | F0.1 | Danh sách screen web/admin | Không có màn ngoài scope MVP | P0 |
| F0.4 | Chốt route frontend | F0.3 | Route table `apps/web`, `apps/admin` | Route phản ánh workspace/channel/message | P0 |
| F0.5 | Chốt env frontend | F0.2 | `.env.example` frontend | `NEXT_PUBLIC_API_BASE_URL=https://chat.vpsttt.com`, `NEXT_PUBLIC_WS_BASE_URL=wss://chat.vpsttt.com/ws`; không mặc định `localhost` | P0 |
| F0.6 | Chốt token storage strategy | F0.2 | Quyết định storage/cookie/local | Refresh flow rõ, logout sạch | P0 |
| F0.7 | Chốt WebSocket auth strategy | F0.2 | Dùng query `access_token` trong browser | Không dùng header Authorization trong native WS | P0 |
| F0.8 | Chốt backend gaps | F0.2 | Backlog backend hardening | RBAC admin/user và realtime gap được ghi rõ | P1 |
| F0.9 | Tạo task board | F0.3 | Issue/task theo mã F* | Mỗi task có owner/status | P1 |

## Phase F1: Nền monorepo frontend

| Task | Công việc | Phụ thuộc | Kết quả | Acceptance | Ưu tiên |
|---|---|---|---|---|---|
| F1.1 | Chọn package manager | F0 | `pnpm`/npm policy | Lockfile nhất quán | P0 |
| F1.2 | Tạo `frontend/package.json` | F1.1 | Workspace scripts | Có `dev:web`, `dev:admin`, `lint`, `typecheck` | P0 |
| F1.3 | Tạo `apps/web` Next.js | F1.2 | App web App Router | Mở được trang shell | P0 |
| F1.4 | Tạo `apps/admin` Next.js | F1.2 | App admin App Router | Mở được trang shell admin | P0 |
| F1.5 | Tạo `packages/types` | F1.2 | Package type shared | Import type từ app được | P0 |
| F1.6 | Tạo `packages/api-client` | F1.2 | Package client shared | Import client từ app được | P0 |
| F1.7 | Tạo `packages/ui` | F1.2 | Package UI shared | Render Button/Card thử | P0 |
| F1.8 | Tạo `packages/icons` | F1.2 | Icon adapter lucide | Dùng icon chung từ app | P1 |
| F1.9 | Tạo `packages/config` | F1.2 | TS/ESLint/Tailwind config | App dùng config chung | P1 |
| F1.10 | Setup path alias | F1.3 | `@/`, `@webtui/*` | Import không dùng relative sâu | P0 |
| F1.11 | Setup `.env.example` frontend | F0.5 | Env mẫu | Dev mới chạy được | P0 |
| F1.12 | Setup build scripts | F1.3-F1.4 | Build web/admin | `build:web`, `build:admin` chạy được | P0 |

## Phase F2: Design system và app shell

| Task | Công việc | Phụ thuộc | Kết quả | Acceptance | Ưu tiên |
|---|---|---|---|---|---|
| F2.1 | Cài Tailwind và shadcn/ui | F1 | UI base | Component shadcn hoạt động | P0 |
| F2.2 | Định nghĩa CSS tokens | F2.1 | Brand color, surface, border, text | Theme không lệch mockup | P0 |
| F2.3 | Tạo `AppRail` | F2.2 | Rail xanh trái | Icon active rõ, avatar đáy | P0 |
| F2.4 | Tạo `ChannelListShell` | F2.2 | Cột kênh/hội thoại | Có tabs, search/add button | P0 |
| F2.5 | Tạo `ChatMainShell` | F2.2 | Header/timeline/composer frame | Layout không overflow desktop | P0 |
| F2.6 | Tạo `RightDetailPanel` | F2.2 | Tabs ghim/ảnh/file | Panel có thể ẩn/drawer | P0 |
| F2.7 | Tạo admin shell | F2.3 | Rail + content admin | Admin dùng chung visual language | P0 |
| F2.8 | Tạo UI primitives | F2.1 | Button, Input, Avatar, Badge, Tooltip | Dùng được trong app | P0 |
| F2.9 | Tạo feedback components | F2.8 | Toast, Empty, Error, Skeleton | Có state loading/error chuẩn | P0 |
| F2.10 | Responsive desktop/tablet/mobile | F2.3-F2.6 | Breakpoints | Mobile không vỡ layout | P1 |
| F2.11 | Visual regression checklist | F2.10 | Checklist screenshot | So được với mockup ảnh số 3 | P2 |

## Phase F3: API client và type nền

| Task | Công việc | Phụ thuộc | Kết quả | Acceptance | Ưu tiên |
|---|---|---|---|---|---|
| F3.1 | Tạo `ApiEnvelope<T>` | F1.5 | Type response chung | Map đúng `success/data/error/meta` | P0 |
| F3.2 | Tạo `ApiError` typed | F3.1 | Error class/helper | UI đọc được `code/message/request_id` | P0 |
| F3.3 | Tạo `HttpClient` | F3.1 | Fetch wrapper | GET/POST/PATCH/DELETE dùng chung | P0 |
| F3.4 | Attach bearer token | F3.3 | Auth interceptor | Request có `Authorization` | P0 |
| F3.5 | Refresh token queue | F3.4 | Chống refresh song song | 401 đồng thời không loop | P0 |
| F3.6 | Multipart upload helper | F3.3 | `uploadForm` | Upload field `file` đúng backend | P0 |
| F3.7 | Blob download helper | F3.3 | `downloadBlob` | Không unwrap envelope file download | P0 |
| F3.8 | Query key factory | F3.3 | Keys theo feature | Không hardcode key rời rạc | P0 |
| F3.9 | API clients auth/users | F3.3 | `authClient`, `usersClient` | Login/me/list gọi được | P0 |
| F3.10 | API clients workspace/RBAC | F3.3 | `workspacesClient`, `rbacClient` | Load workspace/permission | P0 |
| F3.11 | API clients chat/file | F3.3 | `channelsClient`, `messagesClient`, `filesClient` | List/send/upload dùng được | P0 |
| F3.12 | API clients admin/integration | F3.3 | Admin/integration clients | Admin phase không cần viết lại client | P1 |
| F3.13 | RealtimeGateway base | F0.7 | WS connect/send/close/events | Có reconnect hook point | P0 |
| F3.14 | Mock adapter | F3.3 | MSW/mock data nếu cần | Component phát triển không cần backend luôn bật | P2 |
| F3.15 | Unit test API helpers | F3.1-F3.7 | Test unwrap/error/upload/blob | Error case được cover | P0 |

## Phase F4: Auth và session

| Task | Công việc | Phụ thuộc | Kết quả | Acceptance | Ưu tiên |
|---|---|---|---|---|---|
| F4.1 | Login page | F2, F3.9 | Form login | Validate client cơ bản | P0 |
| F4.2 | Register page | F2, F3.9 | Form register | Tạo user và nhận token | P1 |
| F4.3 | Auth store | F3.4 | Zustand/session provider | App biết trạng thái auth | P0 |
| F4.4 | Token persistence | F4.3 | Lưu access/refresh | Reload vẫn restore được | P0 |
| F4.5 | Refresh flow | F3.5 | Auto refresh | Access token hết hạn vẫn tiếp tục | P0 |
| F4.6 | Protected route | F4.3 | Auth guard | Chưa login redirect login | P0 |
| F4.7 | Logout | F4.3 | Clear token + call logout | Logout không còn session local | P0 |
| F4.8 | Current user hook | F3.9 | `useCurrentUser` | Header/avatar có data user | P0 |
| F4.9 | Session list/revoke UI | F3.9 | Security settings | Revoke session hoạt động | P2 |
| F4.10 | Auth tests | F4.1-F4.7 | Unit/component tests | Login error/success cover | P1 |

## Phase F5: Workspace và RBAC

| Task | Công việc | Phụ thuộc | Kết quả | Acceptance | Ưu tiên |
|---|---|---|---|---|---|
| F5.1 | Load workspace list | F3.10, F4 | `useWorkspaces` | User thấy workspace của mình | P0 |
| F5.2 | Workspace switcher | F5.1 | Dropdown/switcher | Chọn workspace đổi route/state | P0 |
| F5.3 | Workspace detail query | F5.2 | `useWorkspace` | Header/shell có tên workspace | P0 |
| F5.4 | Permission query | F3.10, F5.2 | `usePermissions` | Load theo workspace | P0 |
| F5.5 | `can(permission)` helper | F5.4 | Permission helper | UI gate thống nhất | P0 |
| F5.6 | Permission boundary component | F5.5 | `<Can>`/disabled state | Action bị chặn rõ lý do | P0 |
| F5.7 | Workspace settings basic | F5.3 | Name/description/settings | Save được nếu có quyền | P1 |
| F5.8 | Member list basic | F5.3 | Danh sách member | Search/filter local ban đầu | P0 |
| F5.9 | Invite member basic | F5.8 | Form invite | Cần `workspace.invite_user` | P1 |
| F5.10 | RBAC stale handling | F5.4 | Refetch khi 403 | User thấy thông báo quyền thay đổi | P1 |

## Phase F6: Channel và hội thoại

| Task | Công việc | Phụ thuộc | Kết quả | Acceptance | Ưu tiên |
|---|---|---|---|---|---|
| F6.1 | List channels | F3.11, F5 | `useChannels` | Sidebar hiển thị channel | P0 |
| F6.2 | Channel item UI | F2.4, F6.1 | Item có badge/unread/pin UI | Giống mockup | P0 |
| F6.3 | Channel route | F6.1 | `/workspaces/[id]/channels/[channelId]` | Chọn channel đổi URL | P0 |
| F6.4 | Channel header | F6.3 | Tên/mô tả/member count | Header đúng channel | P0 |
| F6.5 | Create channel | F6.1, F5.5 | Form create | Gate `channel.create` | P1 |
| F6.6 | Update/archive channel | F6.1, F5.5 | Menu actions | Gate `channel.manage/delete` | P2 |
| F6.7 | Channel members | F6.3 | Member drawer/list | Xem member trong channel | P1 |
| F6.8 | Add/remove channel member | F6.7 | Form action | Gate `channel.manage` | P2 |
| F6.9 | List direct conversations | F3.11 | `useDirectConversations` | Sidebar có DM | P0 |
| F6.10 | Create direct conversation | F6.9 | Friend request accepted | Tạo DM từ danh bạ bạn bè | P1 |
| F6.11 | Departments grouping | F6.1 | Group channel theo department | Không bắt buộc MVP nếu API đủ | P2 |

## Phase F7: Message timeline REST

| Task | Công việc | Phụ thuộc | Kết quả | Acceptance | Ưu tiên |
|---|---|---|---|---|---|
| F7.1 | Message DTO/type | F3.11 | Message domain model | Map metadata/mentions/reactions | P0 |
| F7.2 | List messages | F6.3 | `useMessages` infinite/cursor | Load timeline theo `before` | P0 |
| F7.3 | Timeline renderer | F2.5, F7.2 | Message bubbles/list | Không giật layout | P0 |
| F7.4 | Composer | F2.5 | Input + actions | Enter gửi, Shift Enter xuống dòng | P0 |
| F7.5 | Send message mutation | F7.4 | `useSendMessage` | Message gửi được | P0 |
| F7.6 | Optimistic sending | F7.5 | Pending message state | Gửi lỗi có retry | P1 |
| F7.7 | Edit message | F7.3 | Inline/menu edit | Owner hoặc `message.manage` | P1 |
| F7.8 | Delete message | F7.3 | Soft delete UI | Xác nhận trước delete | P1 |
| F7.9 | Reaction picker | F7.3 | Add/remove emoji | Reaction summary cập nhật | P1 |
| F7.10 | Mention parse UI | F7.4 | Suggest member khi gõ `@` | Gửi `mentioned_user_ids` đúng | P1 |
| F7.11 | Thread panel | F2.6, F7.2 | Root + replies | Reply bằng `parent_id` | P1 |
| F7.12 | Search messages | F3.11 | Search view/modal | Gọi `/messages/search` | P2 |
| F7.13 | Read state update | F7.2 | Mark read khi vào channel | Gửi `last_read_message_id` | P0 |
| F7.14 | Message tests | F7.1-F7.9 | Tests mapper/mutations | CRUD/reaction cache cover | P1 |

## Phase F8: Realtime WebSocket

| Task | Công việc | Phụ thuộc | Kết quả | Acceptance | Ưu tiên |
|---|---|---|---|---|---|
| F8.1 | WS URL builder | F3.13, F0.5 | `http` -> `ws`, `https` -> `wss` | URL đúng env | P0 |
| F8.2 | Connect with browser query token | F3.13, F4 | `?access_token=...` | Backend nhận auth | P0 |
| F8.3 | Join/leave room | F6.3 | Room lifecycle | Đổi channel leave/join đúng | P0 |
| F8.4 | Reconnect strategy | F8.2 | Backoff reconnect | Offline/online không crash | P0 |
| F8.5 | Socket Zustand state | F8.2 | connected/reconnecting/offline | UI hiển thị trạng thái | P0 |
| F8.6 | Handle `MessageCreated` | F7.2 | Cache merge append/prepend | Hai browser nhận message | P0 |
| F8.7 | Handle `MessageUpdated` | F7.7 | Cache replace | Edit realtime | P0 |
| F8.8 | Handle `MessageDeleted` | F7.8 | Cache mark/remove | Delete realtime | P0 |
| F8.9 | Handle `ReactionChanged` | F7.9 | Cache reaction replace | Reaction realtime | P0 |
| F8.10 | Duplicate guard | F8.6 | ID dedupe | Không nhân đôi optimistic/server event | P0 |
| F8.11 | Expired token handling | F4.5, F8.2 | Refresh then reconnect | WS phục hồi sau token hết hạn | P0 |
| F8.12 | Realtime tests | F8.6-F8.10 | Cache reducer tests | Event merge đúng | P1 |

## Phase F9: File và attachment

| Task | Công việc | Phụ thuộc | Kết quả | Acceptance | Ưu tiên |
|---|---|---|---|---|---|
| F9.1 | File DTO/type | F3.11 | File/Version/Attachment types | Map đúng backend | P0 |
| F9.2 | Upload helper UI | F3.6, F2 | Upload queue | Chọn file và gửi multipart | P0 |
| F9.3 | Upload permission gate | F5.5 | Gate `file.upload` | User không quyền không upload | P0 |
| F9.4 | Attach file to message | F9.2, F7 | Attachment mutation | Message có attachment | P0 |
| F9.5 | List attachments | F9.4 | Attachment display | File hiện trong message | P0 |
| F9.6 | Download blob | F3.7 | Download action | Tải file đúng tên | P0 |
| F9.7 | File right panel | F2.6, F9.1 | Recent files list | Panel giống mockup | P1 |
| F9.8 | Image/media grid | F9.1 | Preview images | Có fallback icon file | P2 |
| F9.9 | File versions UI | F9.1 | Versions list/upload version | Không bắt buộc MVP | P2 |
| F9.10 | Upload error/retry | F9.2 | Retry/remove queue item | Lỗi rõ ràng | P1 |

## Phase F10: Notification và presence

| Task | Công việc | Phụ thuộc | Kết quả | Acceptance | Ưu tiên |
|---|---|---|---|---|---|
| F10.1 | Notification client/hooks | F3.12 | `useNotifications` | Load theo workspace | P0 |
| F10.2 | Notification dropdown | F2.9 | Bell + list | Badge unread | P0 |
| F10.3 | Mark read | F10.2 | Mutation read | Notification đọc xong cập nhật | P0 |
| F10.4 | Mark all read | F10.2 | Bulk action | Clear unread | P1 |
| F10.5 | Notification route target | F10.2 | Click mở channel/message | Deep-link nếu đủ data | P1 |
| F10.6 | Presence client/hooks | F3.12 | `usePresence` | Load presence workspace | P0 |
| F10.7 | Heartbeat loop | F10.6 | PUT heartbeat định kỳ | Không spam backend | P0 |
| F10.8 | Avatar status dot | F10.6 | Online/away/offline UI | Member list có trạng thái | P0 |
| F10.9 | Typing indicator placeholder | F8 | Local/future-ready UI | Chỉ bật nếu backend event có | P2 |
| F10.10 | Presence cleanup UX | F10.7 | Offline khi mất kết nối | State không bị kẹt online | P1 |

## Phase F11: Admin MVP

| Task | Công việc | Phụ thuộc | Kết quả | Acceptance | Ưu tiên |
|---|---|---|---|---|---|
| F11.1 | Admin auth guard | F4, F5 | Guard `admin.view` | Không quyền không vào admin | P0 |
| F11.2 | Admin dashboard stats | F3.12 | Stats cards/charts | Hiển thị users/channels/messages/files | P0 |
| F11.3 | System health | F3.12 | Health checks | DB/Redis/RabbitMQ/storage/ws status | P0 |
| F11.4 | User management list | F3.9 | User table | Search/status/limit | P0 |
| F11.5 | User update/status | F11.4 | Edit user modal | Gate `user.manage`; backend kiểm quyền theo `workspace_id` | P1 |
| F11.6 | Workspace member admin | F5.8 | Members table | Add/update status | P0 |
| F11.7 | Roles list | F3.10 | Roles/permissions table | Load role system/workspace | P0 |
| F11.8 | Create role | F11.7 | Role form | Gate `role.manage` | P1 |
| F11.9 | Assign/revoke role | F11.7 | Role assignment UI | Member role đổi được | P1 |
| F11.10 | Audit logs | F3.12 | Filtered audit table | Filter actor/action/entity/time | P0 |
| F11.11 | Admin table components | F2.8 | DataTable/filter/status | Dùng lại cho ops/integration | P0 |

## Phase F12: Integration UI

| Task | Công việc | Phụ thuộc | Kết quả | Acceptance | Ưu tiên |
|---|---|---|---|---|---|
| F12.1 | API scopes list | F3.12 | Scope catalog | Hiển thị scope/module/action | P0 |
| F12.2 | API token list | F3.12 | Token table | Status/last used/expires | P0 |
| F12.3 | Create API token | F12.2 | Create dialog | Secret chỉ hiện một lần | P0 |
| F12.4 | Revoke API token | F12.2 | Confirm revoke | Token không còn active | P0 |
| F12.5 | Bot list | F3.12 | Bot table/cards | List bots workspace | P1 |
| F12.6 | Create bot | F12.5 | Bot form | Tạo bot slug/name | P1 |
| F12.7 | Bot installations | F12.5 | Installation view | Cài bot vào channel/workspace | P1 |
| F12.8 | Send bot message test | F12.7 | Test send form | Bot gửi message vào channel | P1 |
| F12.9 | Incoming webhook list | F3.12 | Webhook table | URL/secret handling | P0 |
| F12.10 | Create incoming webhook | F12.9 | Create dialog | Secret chỉ hiện một lần | P0 |
| F12.11 | Outgoing webhook list | F3.12 | Outgoing table | Target/event/status | P1 |
| F12.12 | Create outgoing webhook | F12.11 | Form target URL/event types | Validate URL | P1 |
| F12.13 | Delivery logs | F12.11 | Delivery table/detail | Status/response/retry info | P1 |

## Phase F13: Operations UI

| Task | Công việc | Phụ thuộc | Kết quả | Acceptance | Ưu tiên |
|---|---|---|---|---|---|
| F13.1 | Cronjob list | F3.12 | Cronjob table | Filter status/limit | P0 |
| F13.2 | Create cronjob | F13.1 | Form schedule/runner/payload | Validate JSON payload | P0 |
| F13.3 | Update/delete cronjob | F13.1 | Actions | Gate `cronjob.manage` | P0 |
| F13.4 | Manual run cronjob | F13.1 | Run now button | Tạo run mới | P0 |
| F13.5 | Cronjob run history | F13.1 | Runs table/detail log | Xem status/error/duration | P0 |
| F13.6 | Backup job list | F3.12 | Backup table | Target/type/schedule/status | P0 |
| F13.7 | Create backup job | F13.6 | Form backup | MVP database backup | P0 |
| F13.8 | Manual backup run | F13.6 | Run now | Tạo backup run | P0 |
| F13.9 | Backup run history | F13.6 | Runs table | Size/checksum/error | P0 |
| F13.10 | Metrics link/help | F11.3 | Link `/metrics`/docs | Không render raw Prometheus trong app user | P2 |

## Phase F14: UX hardening và responsive

| Task | Công việc | Phụ thuộc | Kết quả | Acceptance | Ưu tiên |
|---|---|---|---|---|---|
| F14.1 | Loading skeleton pass | F2.9 | Skeleton cho screen chính | Không nhấp nháy trắng | P0 |
| F14.2 | Empty state pass | F2.9 | Empty channel/message/file/admin | Copy ngắn, không dạy quá nhiều | P0 |
| F14.3 | Error state pass | F3.2 | Error boundary/toast | Hiện request id nếu có | P0 |
| F14.4 | Offline mode hints | F8.5 | Offline banner | User biết mất kết nối | P1 |
| F14.5 | Keyboard shortcuts | F7 | Enter, Shift Enter, Ctrl/Cmd K | Không xung đột input | P1 |
| F14.6 | Mobile channel navigation | F2.10 | List/main/detail routes | Mobile không 4 cột ép ngang | P0 |
| F14.7 | Tablet right drawer | F2.10 | Detail drawer | Panel không che composer | P1 |
| F14.8 | Accessibility pass | F2-F13 | aria/keyboard/focus | Có focus trap modal/drawer | P0 |
| F14.9 | Performance pass | F7-F9 | Virtual list nếu cần | Timeline dài không lag | P1 |
| F14.10 | Visual polish pass | F2 | So với mockup | Không card lồng card, radius <= 8px | P0 |

## Phase F15: Test, CI, release web

| Task | Công việc | Phụ thuộc | Kết quả | Acceptance | Ưu tiên |
|---|---|---|---|---|---|
| F15.1 | Unit test setup | F1 | Vitest/Jest config | Test chạy workspace | P0 |
| F15.2 | API client tests | F3 | Tests unwrap/auth/upload/blob | Edge cases cover | P0 |
| F15.3 | Permission tests | F5 | `can()`/boundary tests | Role/permission cases cover | P0 |
| F15.4 | Message cache tests | F7-F8 | Realtime reducer tests | Dedup/replace/delete đúng | P0 |
| F15.5 | Component tests | F2-F11 | Sidebar/composer/admin form | User flow basic cover | P1 |
| F15.6 | E2E login/chat | F4-F8 | Playwright spec | Login, chọn channel, gửi message | P0 |
| F15.7 | E2E file upload | F9 | Upload/download spec | File flow pass | P1 |
| F15.8 | E2E admin basic | F11 | Admin spec | Dashboard/RBAC/audit smoke | P1 |
| F15.9 | CI frontend workflow | F15.1 | GitHub Actions | lint/typecheck/test/build | P0 |
| F15.10 | Dockerfile web/admin | F1-F14 | Build image | Image chạy được env production | P1 |
| F15.11 | Deploy config | F15.10 | Env/domain docs | `chat.vpsttt.com` sẵn sàng | P1 |
| F15.12 | Release checklist | F15 | Checklist demo | Không còn P0 bug | P0 |

## Thứ tự MVP nên làm trước

| Sprint | Task group | Kết quả kỳ vọng |
|---|---|---|
| S1 | F0, F1, F2 nền | Monorepo, shell, theme chạy được |
| S2 | F3, F4 | API client, auth, protected route |
| S3 | F5, F6 | Workspace/RBAC/channel/sidebar |
| S4 | F7 | Chat REST: timeline + send/edit/delete/reaction |
| S5 | F8 | Realtime hai browser |
| S6 | F9, F10 basic | File, notification, presence |
| S7 | F11 basic | Admin dashboard, users, members, RBAC, audit/health |
| S8 | F12, F13 basic | Token/webhook/bot/cronjob/backup |
| S9 | F14, F15 | Polish, tests, CI, release demo |

## Screen backlog chi tiết

| Mã | Screen | App | Phase | API chính | Ghi chú |
|---|---|---|---|---|---|
| SCR-01 | Login | web/admin | F4 | `/auth/login` | Dùng chung auth layout |
| SCR-02 | Register | web | F4 | `/auth/register` | Có thể ẩn nếu demo nội bộ tạo user sẵn |
| SCR-03 | Workspace switcher | web | F5 | `/workspaces`, `/rbac/me` | Load permission sau chọn workspace |
| SCR-04 | Chat shell | web | F2/F6 | nhiều API | Màn hình chính đầu tiên |
| SCR-05 | Channel timeline | web | F7/F8 | messages, ws | Lõi MVP |
| SCR-06 | Direct conversation | web | F6/F7 | direct, messages | Có thể chung component channel |
| SCR-07 | Thread panel | web | F7 | `/thread` | Drawer/panel phải |
| SCR-08 | File panel | web | F9 | files, attachments | Gần giống mockup |
| SCR-09 | Notifications | web | F10 | notifications | Dropdown + page nếu cần |
| SCR-10 | Member directory | web | F5/F10 | members, presence | Danh bạ nội bộ |
| SCR-11 | Profile/settings | web | F4/F5 | users/me, sessions | Tối thiểu profile |
| SCR-12 | Admin dashboard | admin | F11 | admin/stats | Stats cards/charts |
| SCR-13 | Admin users | admin | F11 | users | Gate `user.manage`, backend kiểm `workspace_id` |
| SCR-14 | Admin members | admin | F11 | workspace members | Add/update status |
| SCR-15 | Admin RBAC | admin | F11 | rbac | Roles/assign/revoke |
| SCR-16 | Audit logs | admin | F11 | audit-logs | Filter table |
| SCR-17 | System health | admin | F11 | admin/health | Dependency status |
| SCR-18 | API tokens | admin | F12 | api-scopes/api-tokens | Secret one-time |
| SCR-19 | Bots | admin | F12 | bots | CRUD/install/test send |
| SCR-20 | Incoming webhooks | admin | F12 | incoming-webhooks | Secret one-time |
| SCR-21 | Outgoing webhooks | admin | F12 | outgoing-webhooks | Deliveries |
| SCR-22 | Cronjobs | admin | F13 | cronjobs | JSON payload editor |
| SCR-23 | Backups | admin | F13 | backup-jobs | Run history |

## API client module backlog

| Module | Functions tối thiểu | Phase |
|---|---|---|
| `authClient` | `login`, `register`, `refresh`, `logout`, `me`, `sessions`, `revokeSession` | F3/F4 |
| `usersClient` | `me`, `updateMe`, `list`, `get`, `update`, `delete` | F3/F11 |
| `workspacesClient` | `listMine`, `get`, `create`, `update`, `archive`, `members`, `settings`, `invites` | F3/F5 |
| `rbacClient` | `permissions`, `roles`, `myPermissions`, `check`, `memberRoles`, `assign`, `revoke` | F3/F5/F11 |
| `channelsClient` | `list`, `get`, `create`, `update`, `archive`, `members`, `readState`, `directs`, `createDirect` | F3/F6 |
| `messagesClient` | `list`, `send`, `get`, `update`, `delete`, `thread`, `addReaction`, `removeReaction`, `search` | F3/F7 |
| `filesClient` | `list`, `upload`, `get`, `download`, `versions`, `createVersion`, `attachments`, `attach` | F3/F9 |
| `notificationsClient` | `listMine`, `markRead`, `markAllRead` | F10 |
| `presenceClient` | `list`, `heartbeat` | F10 |
| `adminClient` | `stats`, `health`, `auditLogs` | F11 |
| `integrationsClient` | scopes, tokens, bots, webhooks, deliveries | F12 |
| `operationsClient` | cronjobs, cronjob runs, backup jobs, backup runs | F13 |

## Rủi ro và quyết định cần theo dõi

| Rủi ro | Tác động | Cách xử lý | Ưu tiên |
|---|---|---|---|
| OpenAPI lệch route Go | Client sinh sai | Đối chiếu `backend-api-map.md` trước khi tạo client | P0 |
| WebSocket token trong query bị log | Lộ token | Không log URL socket; giữ subprotocol làm fallback cấu hình | P0 |
| User admin API backend thiếu RBAC | Lộ thao tác quản trị | Route update/delete user kiểm `user.manage` theo `workspace_id`, frontend gate permission | P0 |
| Timeline dài bị lag | UX kém | Virtualization/lazy rendering ở F14 | P1 |
| File upload lớn hoặc mạng yếu | Người dùng mất trạng thái | Upload queue, retry, progress | P1 |
| Thiếu API ticket/task chuyên biệt | Panel mockup không đủ dữ liệu | Làm placeholder rõ, không giả endpoint thật; pin message đã có endpoint riêng | P1 |
| Admin quá rộng scope | Trễ MVP | Chỉ làm stats/users/RBAC/audit/health trước | P0 |
| Mobile layout phức tạp | Vỡ UI | Tách list/main/detail thành route/drawer | P1 |

## Definition of Done cho mỗi task

- Code đúng ranh giới: component không gọi API trực tiếp.
- TypeScript không lỗi.
- Không hardcode base URL trong component; base URL đi qua config/env và mặc định trỏ `https://chat.vpsttt.com`.
- Loading, empty, error state có xử lý.
- Permission gate đúng nếu là action nhạy cảm.
- Không log secret, access token, refresh token, webhook secret.
- Có test tối thiểu với logic phức tạp: API mapper, permission, realtime cache.
- UI không lệch theme: xanh-trắng, radius tối đa 8px, không card lồng card.
- Tài liệu/skill được cập nhật nếu đổi contract hoặc workflow.

## Ghi chú triển khai đầu tiên

Nếu bắt đầu code ngay, thứ tự commit nên là:

1. Scaffold `frontend` workspace và scripts.
2. Setup Tailwind/shadcn/ui/theme.
3. Tạo `packages/types` và `packages/api-client`.
4. Làm login + auth provider.
5. Làm workspace switcher + permission.
6. Làm chat shell static theo mockup.
7. Nối channel/message REST.
8. Nối WebSocket realtime.

## Cập nhật triển khai ngày 2026-07-08

### Phase F2 rà soát bổ sung

- Đã loại bỏ `chat-workspace-source.ts` và `dashboard-source.ts`; web/admin không còn lấy dữ liệu màn hình từ snapshot áp cứng.
- App shell F2 vẫn được giữ làm nền UI, nhưng dữ liệu hiển thị hiện đi qua TanStack Query và `@webtui/api-client`.
- Các vùng backend chưa có endpoint rõ ràng như media preview ranking theo ngày không dựng số mẫu; tin ghim đã dùng endpoint pin riêng.

### Phase F3 hoàn thành

- `packages/api-client` đã có `HttpClient` dùng chung cho GET/POST/PUT/PATCH/DELETE, unwrap envelope, lỗi typed, bearer token, refresh queue, blob download và multipart upload.
- Đã thêm client theo module: auth, users, workspaces, channels, messages, files, RBAC, admin, notifications, API tokens, webhooks, bots, cronjobs, backups.
- Đã thêm `queryKeys` và `RealtimeGateway` base. Phase F8 sẽ hoàn thiện reconnect, join room và merge cache.
- `packages/types` đã bổ sung DTO auth, workspace, chat, file, admin, RBAC theo handler Go. Lưu ý auth result production dùng `data.tokens.access_token`, không phải token phẳng ở root.

### Phase F4 hoàn thành

- Web/admin có QueryClientProvider, AuthProvider, Zustand auth store, login/register/logout, restore session và auto refresh khi request gặp 401.
- Auth UI dùng tiếng Việt có dấu, bám theme xanh-trắng, không hardcode backend localhost.
- Web app sau đăng nhập gọi thật: workspaces, channels, messages, direct conversations, files; gửi tin nhắn, upload file, attach file và download file đều đi qua API.
- Admin app sau đăng nhập gọi thật: workspaces, admin stats, admin health, workspace settings và users; không còn thêm user local hoặc vẽ metric giả.

### Hướng làm tốt hơn cho F5/F6

- Đã tách query/mutation sang `useWorkspaceContext`, `useChatWorkspaceData` và `useAdminDashboardData`.
- Đã thêm permission gate từ `/api/v1/rbac/me?workspace_id=...` trước create channel, send message, upload file và admin dashboard.
- Đã thêm URL state cho `workspace` và `channel`; shell root hiện giữ UI ổn định, có thể thêm route alias đẹp hơn ở phase polish.
- Đã bổ sung form tạo kênh thật có validate `slug`, `name`, `type` và gọi API create channel.
- Nếu backend trả 403 ở admin, UI hiển thị rõ thiếu quyền `admin.view`.

### Phase F5 hoàn thành

- Web/admin load workspace list từ `/api/v1/workspaces`, tự chọn workspace đầu tiên nếu URL chưa có `workspace`.
- Workspace detail query gọi `/api/v1/workspaces/{workspace_id}` để shell có dữ liệu chi tiết, không phụ thuộc payload list.
- RBAC query gọi `/api/v1/rbac/me?workspace_id=...`; `can(permission)` dùng permission code hoặc wildcard `*`.
- `PermissionGate` dùng cho boundary quyền, action nhỏ dùng disabled state kèm lý do.
- Member/settings query đã sẵn sàng cho member directory và settings. DM 1-1 đi qua danh bạ bạn bè/contact request, không dùng picker member để bypass kết bạn.

### Phase F6 hoàn thành

- Channel sidebar lấy từ `/api/v1/workspaces/{workspace_id}/channels`; chọn kênh đồng bộ vào URL query `channel`.
- Header/timeline/composer dùng selected channel từ API, không còn kênh mẫu.
- Direct conversations lấy từ `/api/v1/workspaces/{workspace_id}/direct-conversations`; tạo DM từ member list thật.
- Form tạo kênh gọi API thật và invalidate channel list.
- Composer gửi message qua `/messages`; nếu có file thì upload `/files` và attach vào message bằng endpoint attachment.
- Recent files/media lấy từ `/files`; tin ghim dùng endpoint `/pins`, media chuyên biệt vẫn hiển thị empty state nếu backend chưa có endpoint riêng.

### Hướng làm tốt hơn cho F7/F8/F9

- F7 đã tách timeline thành `useMessageTimeline` với infinite cursor, optimistic sending, edit/delete, reaction, thread và search.
- F8 đã dùng `RealtimeGateway` query token, join/leave room, reconnect backoff, socket status store và merge cache chống trùng event.
- F9 cần nâng upload một file hiện tại thành upload queue có progress, retry, remove, file preview và download action đầy đủ.

### Phase F7 hoàn thành

- Timeline message dùng `useInfiniteQuery` và cursor `before` từ backend.
- `messagesClient` giữ được response meta qua `listPage`, `threadPage`, `searchPage`.
- Composer gửi tin nhắn với optimistic local message, rollback khi lỗi và replace khi API trả message thật.
- Inline edit, delete và reaction gọi API thật, cập nhật cache và hiển thị lỗi qua toast.
- Thread panel lấy dữ liệu từ `/thread`; search lấy dữ liệu từ `/messages/search`.
- Read-state gửi `last_read_message_id` theo tin nhắn cuối đã load.

### Phase F8 hoàn thành

- Realtime hook kết nối `wss://chat.vpsttt.com/ws` bằng query `access_token`; subprotocol JWT chỉ là fallback cấu hình.
- Khi chọn channel, frontend join room `workspace:{workspaceId}:channel:{channelId}` và leave khi đổi kênh/unmount.
- Socket có trạng thái `idle`, `connecting`, `connected`, `reconnecting`, `offline` trong Zustand.
- Event `MessageCreated`, `MessageUpdated`, `MessageDeleted`, `ReactionChanged` merge vào cache timeline, có chống trùng.
- Reconnect dùng backoff tối đa 15 giây.

### Hướng làm tốt hơn cho F9/F10

- F9: upload queue, progress, retry/remove, attachment preview trong timeline, download file trực tiếp từ attachment.
- F10: notification dropdown, mark read/all read, presence heartbeat và avatar status theo API.

### Phase F9 hoàn thành

- `packages/types` đã bổ sung đúng type `FileAttachment`, `FileObject.byte_size/status/checksum` và message attachment có `file` lồng.
- `packages/api-client` đã sửa `files.attachments/attach` trả `FileAttachment`.
- Composer web dùng upload queue nhiều file qua Zustand, có trạng thái chờ gửi, đang tải, đã gắn, lỗi, remove và retry.
- Send message tạo message trước, upload và attach từng file sau; lỗi file không rollback message đã gửi.
- Timeline render attachment list từ API và download blob trực tiếp bằng `file_id`.
- Vì backend message list chưa hydrate attachment, web app query `/attachments` theo message đang hiển thị và invalidate sau attach.
- File panel/media grid tiếp tục dùng `/files` thật, không dùng dữ liệu mẫu. File versions UI vẫn là backlog P2.

### Phase F10 hoàn thành

- `packages/api-client` có `notificationsClient` typed và `presenceClient`.
- `useNotificationPresence` gom notification query, mark read/read-all, presence list và heartbeat 30 giây.
- Header có notification badge/dropdown; click notification mark read và chuyển channel/message nếu backend trả target.
- Presence map cập nhật avatar status trong direct conversation/member list; `away` map sang trạng thái UI bận.
- Presence query/heartbeat xử lý lỗi mềm để chat không bị chặn nếu user thiếu quyền xem member/presence.

### Hướng làm tốt hơn cho F11/F12

- F11 nên bắt đầu bằng admin guard `admin.view`, sau đó stats, health, users, roles/RBAC và audit table bằng API thật.
- Các bảng admin nên dùng component table/filter chung để F12/F13 tái sử dụng cho token, webhook, bot, cronjob và backup.
- Không đưa lại số liệu áp cứng vào dashboard; nếu backend thiếu metric nào thì dùng empty/error state rõ ràng.

### Kiểm tra đã chạy

- `npm.cmd install`
- `npm.cmd run typecheck`
- `npm.cmd run lint`

## Cập nhật triển khai ngày 2026-07-09: Phase F15

### Phase F15 hoàn thành

- Unit test setup dùng Vitest tại `frontend/vitest.config.ts`; root script có `test`, `test:unit`, `test:unit:watch`.
- API client có test envelope unwrap, auth/query header, JSON body, refresh token, error envelope và binary blob.
- Permission logic được tách thành helper dùng chung trong `@webtui/types`, có test cho string/object permission, wildcard và case thiếu quyền.
- Message cache reducer được tách khỏi hook sang model thuần, có test dedupe, sort, insert, replace optimistic message, update và remove.
- Component test có smoke test cho `PermissionGate` bằng render server.
- Playwright E2E có smoke spec cho web chat và admin, mặc định skip nếu chưa bật `E2E_RUN=true`.
- CI frontend đã chạy `typecheck`, `lint`, `test:unit`, `build:web`, `build:admin`.
- Dockerfile cho web/admin đã được thêm ở `frontend/apps/web/Dockerfile` và `frontend/apps/admin/Dockerfile`.
- Release checklist được thêm tại `docs/deploy/frontend-release.md`.

### Ghi chú xác minh

- Docker CLI có sẵn nhưng local Docker daemon chưa kết nối được pipe `docker_engine`, nên chưa build image tại máy này. Dockerfile đã có lệnh build/run trong release checklist.
- `npm audit` báo 2 cảnh báo mức moderate sau khi cài dependency test; chưa chạy `npm audit fix --force` vì có thể nâng major không kiểm soát.

### Kiểm tra đã chạy

- `npm.cmd run typecheck`
- `npm.cmd run lint`
- `npm.cmd test`
- `npm.cmd run test:e2e` mặc định: 3 skipped do chưa bật `E2E_RUN=true`
- `npm.cmd run build:web`
- `npm.cmd run build:admin`
- `npm.cmd run build:web`
- `npm.cmd run build:admin`

## Cập nhật triển khai ngày 2026-07-09: Phase F13/F14

### Rà soát F11/F12

- F11/F12 không còn thiếu sót P0 trước khi bước sang vận hành. Admin guard, stats/health/users/RBAC/audit, API token, webhook, bot và delivery logs đều đang dùng API thật qua hook/use-case tập trung.
- Điểm cần đưa sang phase sau: `AdminDashboard` đã lớn, nên F15 hoặc lần polish tiếp theo cần tách thành các section file riêng trước khi thêm test sâu.

### Phase F13 hoàn thành

- `packages/types` bổ sung DTO operations cho `CronJob`, `CronJobRun`, `BackupJob`, `BackupRun` theo đúng backend Go.
- `packages/api-client` bổ sung typed client cho `/cronjobs`, `/cronjobs/{id}/runs`, `/cronjobs/{id}/run`, `/backup-jobs`, `/backup-jobs/{id}/runs`, `/backup-jobs/{id}/run`.
- Admin hook có query/mutation thật cho cronjob và backup, gate theo `cronjob.manage` và `backup.manage`.
- Màn Cronjob có tạo job, cập nhật trạng thái, xóa job, chạy thủ công và bảng run history; payload JSON được validate là object trước khi gửi API.
- Màn Backup có tạo database backup job, chạy thủ công và bảng run history; không dựng update/delete giả vì backend chưa có endpoint tương ứng.

### Phase F14 hoàn thành bước đầu

- Operations UI có loading skeleton, empty state, error state, permission notice và toast lỗi từ backend.
- Admin app có breakpoint mobile: rail chuyển ngang, header/action xếp lại, bảng cuộn ngang khi cần.
- Web chat có breakpoint mobile nhỏ hơn để rail chuyển ngang, toolbar/header/composer gọn hơn và tránh layout bốn cột ép ngang.

### Hướng làm tốt hơn cho F15

- Tách `AdminDashboard` thành các section: overview, users, roles, integrations, bots, operations, settings; giữ `useAdminDashboardData` hoặc tách hook con theo nhóm nếu file tiếp tục lớn.
- Bổ sung component test cho form cronjob/backup, JSON validation và permission notice.
- Bổ sung E2E admin smoke cho đăng nhập, chọn workspace, mở Cronjob/Backup, tạo job và kiểm tra run history bằng API thật hoặc test fixture backend.

### Kiểm tra đã chạy

- `npm.cmd run typecheck`
- `npm.cmd run lint`

## Cập nhật triển khai ngày 2026-07-09: Phase F11/F12

### Rà soát F9/F10

- F9 và F10 không còn thiếu sót P0 trước khi làm admin/integration. Upload queue, attachment list/download, notification mark read/read-all và presence heartbeat đang dùng API thật.
- Ghi chú kỹ thuật còn giữ cho phase sau: backend message list chưa hydrate attachment, nên frontend vẫn phải query attachment riêng theo message đang hiển thị.

### Phase F11 hoàn thành

- Admin dashboard dùng `useAdminDashboardData` làm hook/use-case tập trung; component không gọi API trực tiếp.
- Màn tổng quan dùng dữ liệu thật từ `admin/stats`, `admin/health`, `users`, `workspace settings`.
- Màn người dùng có table từ `/api/v1/users`, thao tác cập nhật trạng thái user, member list từ workspace, thêm member và cập nhật trạng thái member.
- Màn vai trò có role table, permission catalog, tạo role, gán role, gỡ role và audit log.
- Audit log dùng endpoint `/workspaces/{workspace_id}/audit-logs`; thiếu quyền `audit.view` sẽ hiển thị notice quyền, không dựng dữ liệu giả.
- UI copy admin đã thay lại tiếng Việt có dấu chuẩn ở các phần mới.

### Phase F12 hoàn thành

- `packages/types` bổ sung DTO integration: API scope/token, bot, bot installation/message, incoming/outgoing webhook, webhook delivery.
- `packages/api-client` bổ sung type cụ thể cho token/bot/webhook thay vì dùng `ModuleRecord` ở luồng F12 chính.
- Màn tích hợp có tạo API token theo scope thật, danh sách token và revoke token.
- Màn tích hợp có tạo incoming webhook, hiển thị URL/secret one-time, danh sách incoming webhook.
- Màn tích hợp có tạo outgoing webhook, nhập event types, danh sách outgoing webhook và delivery logs.
- Màn bot có tạo bot, chọn bot, cài bot vào kênh và gửi message test bằng API thật.

### Hướng làm tốt hơn cho F13/F14

- Trước F13 nên tách component admin lớn thành các section nhỏ để cronjob/backup không làm file dashboard phình thêm.
- F13 nên đi theo cùng pattern: typed DTO, typed client, query keys riêng, permission notice, table run history và mutation form.
- Backend user admin API đã có hardening bước đầu bằng permission `user.manage`; phase sau cần bổ sung test/coverage production cho rule này.

### Kiểm tra đã chạy

- `npm.cmd run typecheck`
- `npm.cmd run lint`
- `npm.cmd run build:web`
- `npm.cmd run build:admin`
