# Phase F1 - Nền monorepo frontend

Phase F1 tạo nền kỹ thuật đầu tiên cho frontend WebTui Chat. Mục tiêu là có workspace nhất quán, app web/admin mở được shell và các package dùng chung đã có ranh giới rõ.

## Trạng thái task

| Task | Trạng thái | Kết quả |
|---|---|---|
| F1.1 | Hoàn thành | Chọn npm workspaces vì máy hiện có `npm.cmd` và chưa có `pnpm`. |
| F1.2 | Hoàn thành | Tạo `frontend/package.json` với scripts workspace. |
| F1.3 | Hoàn thành | Tạo `frontend/apps/web` dạng Next.js App Router. |
| F1.4 | Hoàn thành | Tạo `frontend/apps/admin` dạng Next.js App Router. |
| F1.5 | Hoàn thành | Tạo `@webtui/types`. |
| F1.6 | Hoàn thành | Tạo `@webtui/api-client` với runtime config mặc định. |
| F1.7 | Hoàn thành | Tạo `@webtui/ui` với primitive đầu tiên. |
| F1.8 | Hoàn thành | Tạo `@webtui/icons` làm adapter lucide. |
| F1.9 | Hoàn thành | Tạo `@webtui/config` cho TypeScript và ESLint base. |
| F1.10 | Hoàn thành | Thiết lập alias `@/*` và `@webtui/*`. |
| F1.11 | Hoàn thành | Tạo `frontend/.env.example` dùng API production. |
| F1.12 | Hoàn thành | Tạo scripts build/typecheck/lint cho web/admin. |

## Cấu trúc đã có

```text
frontend/
├── apps/
│   ├── admin/
│   └── web/
├── packages/
│   ├── api-client/
│   ├── config/
│   ├── icons/
│   ├── types/
│   └── ui/
├── .env.example
└── package.json
```

## Quy ước package manager

- Dùng npm workspaces trong phase hiện tại.
- Script phải chạy qua `npm.cmd` trên Windows nếu PowerShell chặn `npm.ps1`.
- Nếu sau này muốn đổi sang `pnpm`, làm ở một phase riêng để tránh churn lockfile.

## Scripts chính

| Script | Mục đích |
|---|---|
| `npm run dev:web` | Chạy app chat ở cổng 3000 từ thư mục `frontend`. |
| `npm run dev:admin` | Chạy app admin ở cổng 3001 từ thư mục `frontend`. |
| `npm run build:web` | Build app web. |
| `npm run build:admin` | Build app admin. |
| `npm run lint` | Lint toàn workspace frontend. |
| `npm run typecheck` | Typecheck toàn workspace frontend. |

## Kiểm tra đã chạy

| Lệnh | Trạng thái |
|---|---|
| `npm.cmd install` | Thành công, đã tạo `frontend/package-lock.json`. |
| `npm.cmd run typecheck` | Thành công. |
| `npm.cmd run lint` | Thành công. |
| `npm.cmd run build:web` | Thành công với Next.js 16.2.10. |
| `npm.cmd run build:admin` | Thành công với Next.js 16.2.10. |

Ghi chú: `npm install` báo 2 lỗ hổng mức moderate từ cây dependency hiện tại. Không chạy `npm audit fix --force` trong F1 vì lệnh này có thể nâng breaking change ngoài phạm vi.

## Handoff cho F2

F2 nên tiếp tục theo hướng này:

- Nâng các class shell đang ở app CSS thành component pattern trong `@webtui/ui`.
- Thêm Tailwind và shadcn/ui có kiểm soát, giữ radius tối đa 8px.
- Tách `AppRail`, `ChannelListShell`, `ChatMainShell`, `RightDetailPanel` thành component có props thật.
- Duy trì theme xanh-trắng, admin dense, copy tiếng Việt có dấu.
- Chụp screenshot desktop/tablet/mobile sau khi có dev server để kiểm tra layout không vỡ.

## Lưu ý chưa làm trong F1

- F1 chưa gọi API thật trong UI, chỉ tạo nền package và runtime config.
- F1 chưa triển khai auth, Query Client, Zustand hoặc WebSocket runtime.
- Dev server chạy được khi gọi trực tiếp `npm.cmd run dev:web`; môi trường tool hiện tại không giữ được process nền sau khi spawn, nên cần chạy trực tiếp trong terminal IDE khi muốn xem UI lâu dài.
