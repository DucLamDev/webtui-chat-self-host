# Hardening, performance và observability

Tài liệu này mô tả phần nền của phase 11 cho backend WebTui Chat. Mục tiêu là tăng độ tin cậy khi demo nội bộ và chuẩn bị vận hành production.

## Middleware bảo mật

API server bật các middleware dùng chung ở `internal/shared/middleware`:

- `SecurityHeaders`: thêm `X-Content-Type-Options`, `X-Frame-Options`, `Referrer-Policy`, `Permissions-Policy` và `Strict-Transport-Security` khi `APP_ENV=production`.
- `CORS`: chỉ cho phép origin nằm trong `CORS_ALLOWED_ORIGINS`.
- `RateLimit`: giới hạn request theo IP trong cửa sổ 1 phút, trả `429 RATE_LIMITED` khi vượt ngưỡng.
- `RequestID`, `Recovery`, `AccessLog`: giữ chuẩn request id, log có dấu và response lỗi thống nhất.

Biến môi trường:

```env
CORS_ALLOWED_ORIGINS=http://localhost:3000,http://localhost:3001,http://localhost:5173
TRUSTED_PROXIES=
SECURE_HEADERS_ENABLED=true
RATE_LIMIT_ENABLED=true
RATE_LIMIT_PER_MINUTE=120
RATE_LIMIT_BURST=60
```

`RATE_LIMIT_PER_MINUTE + RATE_LIMIT_BURST` là số request tối đa mỗi IP trong 1 phút. Khi chạy sau Nginx, cấu hình `TRUSTED_PROXIES` bằng IP hoặc CIDR proxy tin cậy để `ClientIP` phản ánh IP thật.

## Metrics Prometheus

Khi `METRICS_ENABLED=true`, API mở endpoint public:

```text
GET /metrics
```

Metric hiện có:

- `webtui_process_uptime_seconds`: thời gian process đã chạy.
- `webtui_http_requests_total`: tổng request theo method, route và status.
- `webtui_http_request_duration_seconds_sum`: tổng thời gian xử lý request theo method, route và status.
- `webtui_dependency_up`: trạng thái dependency chính gồm PostgreSQL, Redis và RabbitMQ.
- `webtui_websocket_clients`: số client WebSocket đang kết nối trên node hiện tại.
- `webtui_websocket_rooms`: số room WebSocket đang có thành viên trên node hiện tại.

Biến môi trường:

```env
METRICS_ENABLED=true
METRICS_PATH=/metrics
```

Ví dụ kiểm tra local:

```powershell
Invoke-RestMethod http://localhost:8080/metrics
```

## Kiểm thử

Phase 11 bổ sung test cho middleware:

```powershell
$env:GOCACHE="C:\Users\duclam\Desktop\application-chat-be\.cache\go-build"
go test ./...
```

Các test kiểm tra header bảo mật, CORS preflight, rate limit và nội dung metric.

## Việc còn lại trước production

- Chuyển rate limit sang Redis nếu chạy nhiều replica API.
- Bổ sung tracing OpenTelemetry khi có dashboard vận hành.
- Thêm benchmark WebSocket, queue throughput và API contract test đầy đủ.
- Cấu hình Prometheus/Grafana trong phase Docker Compose production.
