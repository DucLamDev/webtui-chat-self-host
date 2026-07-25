# Kiến trúc frontend

Frontend WebTui Chat dùng Next.js App Router, shadcn/ui, TanStack Query và Zustand. Mục tiêu là một chat nội bộ tự host có web app người dùng và admin panel, bám backend Go + Gin hiện có.

## Đánh giá hiện trạng

Bản kiến trúc cũ mới đúng ở mức liệt kê công nghệ và package, chưa đủ chuẩn Clean Architecture cho frontend lớn. Những phần còn thiếu là hướng phụ thuộc, ranh giới feature, API client tập trung, RBAC, realtime lifecycle, upload/download, testing và cách chia sẻ code giữa `apps/web` với `apps/admin`.

## Ứng dụng

- `apps/web`: trải nghiệm chat của người dùng.
- `apps/admin`: trang quản trị hệ thống và vận hành workspace.

## Package dùng chung

- `packages/api-client`: REST/WebSocket client typed, auth token, envelope unwrap, upload/download.
- `packages/config`: ESLint, Tailwind, TypeScript và cấu hình build dùng chung.
- `packages/icons`: adapter icon dùng chung, ưu tiên lucide.
- `packages/types`: DTO, domain type, permission code và API envelope.
- `packages/ui`: component UI thuần trình bày, không gọi API.

## Hướng phụ thuộc

```text
apps/* routes/pages
  -> features/*
    -> hooks/use-cases
    -> adapters/api-client
    -> adapters/realtime
    -> model
  -> packages/ui
  -> packages/types

packages/api-client -> packages/types
packages/ui -> packages/icons
packages/types -> không phụ thuộc runtime
```

Component không gọi API trực tiếp. Component gọi hook/use case của feature; hook/use case gọi `packages/api-client`.

## Cấu trúc feature

```text
features/messages/
├── api/
├── components/
├── hooks/
├── model/
├── stores/
└── index.ts
```

- `api`: query key, mapper DTO, function dùng API client.
- `hooks`: TanStack Query hook, mutation hook, realtime hook.
- `model`: type domain nhẹ, permission helper, optimistic update helper.
- `stores`: client state cục bộ như composer draft, selected thread.
- `components`: UI feature, chỉ nhận data/callback và gọi hook.

## API và realtime

- Frontend mặc định dùng backend đã deploy trên cùng domain: REST base `https://chat.vpsttt.com`, WebSocket public endpoint `wss://chat.vpsttt.com/ws`. Không dùng `localhost` cho app web/admin trừ khi đang làm phiên backend-dev được yêu cầu riêng.
- Đọc contract chính ở `backend/api/openapi/openapi.yaml`.
- Khi OpenAPI thiếu, đối chiếu route thật trong `backend/internal/**/delivery/http/handler.go`.
- Response JSON dùng envelope `success`, `data`, `error`, `meta`, `request_id`, `timestamp`.
- List response nằm trong key cụ thể như `messages`, `channels`, `files`, `api_tokens`, `presence`.
- `GET /api/v1/workspaces/{workspace_id}/files/{file_id}/download` trả binary, không unwrap envelope.
- WebSocket public l� `GET /ws` qua Nginx, proxy v? route Go n?i b? `GET /api/v1/ws`; backend h? tr? `Authorization: Bearer ...`, query `access_token` v� browser subprotocol `['webtui.jwt', accessToken]`. Frontend n�n uu ti�n subprotocol ho?c query token qua HTTPS/WSS, kh�ng log URL ch?a token.

## State

- TanStack Query cho server state: workspace, channel, message, file, notification, permission, admin stats, cronjob, backup.
- Zustand cho client state: selected workspace/channel, composer draft, sidebar collapsed, right panel tab, upload queue, socket status.
- URL state cho route params, search/filter và deep-link thread/message khi cần.

## RBAC

- Sau khi chọn workspace, gọi `GET /api/v1/rbac/me?workspace_id=...`.
- UI gate bằng permission code, không suy từ tên role.
- Permission quan trọng: `workspace.manage`, `workspace.invite_user`, `workspace.view_members`, `role.manage`, `channel.create`, `channel.manage`, `channel.delete`, `message.send`, `message.manage`, `file.upload`, `api_token.manage`, `bot.manage`, `webhook.manage`, `cronjob.manage`, `backup.manage`, `audit.view`, `admin.view`.

## Theme giao diện

Giao diện bám mockup chat nội bộ màu xanh-trắng: rail trái xanh đậm, channel list sáng, main chat rộng, panel phải cho ghim/ảnh/file, dashboard/admin dense và dễ quét. Không làm landing page khi nhiệm vụ là app chat.

Quy tắc nhanh:

- Dùng lucide icons trong navigation và action button.
- Radius card tối đa 8px.
- Không đặt card trong card.
- Badge unread đỏ nhỏ, online status bằng chấm xanh.
- Admin dùng bảng/filter/search/status rõ ràng.
- Copy UI tiếng Việt ngắn: `Tin nhắn`, `Kênh`, `Thông báo`, `File`, `Bot`, `Automation`, `Danh bạ`, `Cài đặt`.

## Testing

- Unit test mapper, query key, permission helper và store reducer.
- Component test các surface chính: sidebar, timeline, composer, upload, admin form.
- Contract/mock test phải dùng đúng API envelope của backend.
- E2E MVP: login, chọn workspace, tạo channel, gửi message, upload file, đánh dấu đã đọc.

## Agent skill

Khi agent làm frontend, dùng `.agents/webtui-chat-frontend/SKILL.md`.

- `references/backend-api-map.md`: toàn bộ API backend và cách map vào màn hình.
- `references/frontend-clean-architecture.md`: ranh giới Clean Architecture và workflow.
- `references/ui-theme.md`: theme, layout và component rule theo mockup.

