# Frontend release checklist

Tài liệu này dùng cho release web app và admin panel WebTui Chat.

## Môi trường

- REST API production: `https://chat.vpsttt.com`
- WebSocket production: `wss://chat.vpsttt.com/ws`
- Web app dự kiến: `chat.vpsttt.com`
- Admin app dự kiến: `chat.vpsttt.com/admin`

## Biến môi trường

```env
NEXT_PUBLIC_API_BASE_URL=https://chat.vpsttt.com
NEXT_PUBLIC_WS_BASE_URL=wss://chat.vpsttt.com/ws
NEXT_PUBLIC_APP_NAME=WebTui Chat
NEXT_PUBLIC_DEFAULT_LOCALE=vi-VN
```

## Kiểm tra bắt buộc trước release

Chạy trong thư mục `frontend`:

```powershell
npm.cmd ci
npm.cmd run typecheck
npm.cmd run lint
npm.cmd run test:unit
npm.cmd run build:web
npm.cmd run build:admin
```

## E2E smoke test

E2E mặc định được skip để CI không phụ thuộc tài khoản thật. Khi có tài khoản staging hoặc production, chạy:

```powershell
$env:E2E_RUN="true"
$env:E2E_WEB_BASE_URL="http://localhost:3000"
$env:E2E_ADMIN_BASE_URL="http://localhost:3001"
$env:E2E_IDENTIFIER="user@example.com"
$env:E2E_PASSWORD="password"
$env:E2E_ADMIN_IDENTIFIER="admin@example.com"
$env:E2E_ADMIN_PASSWORD="password"
npm.cmd run test:e2e
```

## Docker build

Build từ thư mục `frontend`:

```powershell
docker build -f apps/web/Dockerfile -t webtui-chat-web:latest .
docker build -f apps/admin/Dockerfile -t webtui-chat-admin:latest .
```

Chạy thử:

```powershell
docker run --rm -p 3000:3000 --env-file .env.example webtui-chat-web:latest
docker run --rm -p 3001:3001 --env-file .env.example webtui-chat-admin:latest
```

Admin Docker image mặc định phục vụ ở `http://localhost:3001/admin`; production đi qua `https://chat.vpsttt.com/admin`.

## Checklist bàn giao

- Đăng nhập web app thành công bằng tài khoản thật.
- Chọn workspace, mở kênh, tải timeline, gửi tin nhắn và nhận realtime.
- Upload file, tải file và xem attachment trong timeline.
- Mở admin panel bằng tài khoản có `admin.view`.
- Kiểm tra users, RBAC, audit, token/webhook/bot, cronjob và backup.
- Không có dữ liệu mẫu hoặc dữ liệu áp cứng trên các màn chính.
- Không có tiếng Việt không dấu trong UI mới.
- Rollback image hoặc tag trước đó đã được xác định.
