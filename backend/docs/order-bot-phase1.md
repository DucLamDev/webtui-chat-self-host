# Order Bot VPSTTT Phase 1

Tài liệu này mô tả phần tích hợp an toàn giữa WebTui Chat và hệ thống `order.vpsttt.com` để phục vụ chăm sóc khách hàng nội bộ.

## Phạm vi Phase 1

Phase 1 chỉ dùng các API nội bộ an toàn:

- `POST /internal/wallet/balance`: tra số dư ví khách hàng.
- `POST /internal/wallet/deposit-qr`: tạo QR nạp ví.
- `POST /internal/services/expiring`: kiểm tra dịch vụ sắp hết hạn.

Không gọi các endpoint mua/gia hạn legacy bằng `GET` vì tài liệu order có nhóm API có thể trừ tiền trực tiếp.

## Cấu hình môi trường

Backend cần các biến sau:

```env
ORDER_API_BASE_URL=https://order.vpsttt.com/api
ORDER_INTERNAL_API_KEY=your-internal-api-key
ORDER_QUICK_ORDER_KEY=
ORDER_API_TIMEOUT=10s
```

`ORDER_INTERNAL_API_KEY` được gửi tới Order API bằng header `X-API-Key`. Không commit key thật lên source.
Biến này phải chứa internal key dùng thành công với nhóm `/api/internal/*` trong Swagger.

## Quyền RBAC

Migration `000010_order_bot_phase1` thêm:

- `order.view`: tra ví và dịch vụ sắp hết hạn.
- `order.billing`: tạo QR nạp ví.

Mặc định `workspace_owner` và `workspace_admin` có cả hai quyền.

## Bot và kênh mặc định

Hệ thống tự tạo hoặc cài các bot sau vào workspace:

- `CSKH Bot` (`cskh-bot`) vào kênh `#ticket`.
- `Thanh Toán Bot` (`thanh-toan-bot`) vào kênh `#ke-toan`.
- `Gia Hạn Bot` (`gia-han-bot`) vào kênh `#gia-han`.

Workspace tạo mới cũng được provision các bot này qua default workspace catalogue.

## API WebTui Chat

Các endpoint mới nằm dưới:

```text
/api/v1/workspaces/:workspace_id/order-bot
```

Danh sách endpoint:

```http
GET  /status
POST /wallet/balance
POST /wallet/deposit-qr
POST /services/expiring
```

Ví dụ tra ví:

```json
{
  "email": "khach@example.com",
  "post_to_channel": true
}
```

Ví dụ tạo QR:

```json
{
  "email": "khach@example.com",
  "amount": 200000,
  "expires_minutes": 1440
}
```

Ví dụ kiểm tra dịch vụ hết hạn:

```json
{
  "email": "khach@example.com",
  "days": 7,
  "service_type": "all",
  "include_expired": false
}
```

Mặc định các API sẽ gửi kết quả vào kênh bot tương ứng. Có thể truyền `"post_to_channel": false` nếu chỉ muốn lấy dữ liệu mà không gửi tin nhắn.

## Bot tự động trong kênh

Ngoài module Bot, hệ thống còn có auto responder chạy ngay sau khi nhân viên gửi tin nhắn trong kênh bot mặc định.

### `#gia-han`

Gửi:

```text
Email: khach@example.com
Số ngày: 7
Loại dịch vụ: Tất cả
```

`Gia Hạn Bot` tự gọi Order API `/internal/services/expiring` và phản hồi danh sách dịch vụ sắp hết hạn trong chính kênh.

### `#ke-toan`

Gửi:

```text
Email: khach@example.com
Số tiền: 200000
```

`Thanh Toán Bot` tự gọi Order API `/internal/wallet/deposit-qr` và phản hồi thông tin QR nạp ví.

### `#ticket`

Gửi:

```text
Tra ví khach@example.com
```

`CSKH Bot` tự gọi Order API `/internal/wallet/balance` và phản hồi số dư ví.

Nếu gửi nội dung ticket/lỗi khách hàng, `Ticket Bot` tự phân loại mức ưu tiên và trả checklist xử lý.

### `#server-alert`

Gửi nội dung cảnh báo như:

```text
Server: vps-01
Lỗi: mất ping 3 phút
Port: 22 timeout
Mức độ: critical
```

`Server Alert Bot` tự phân tích mức độ, dấu hiệu và trả checklist vận hành.

Mỗi kênh đều hỗ trợ tin nhắn `help` hoặc `hướng dẫn` để bot trả mẫu lệnh.

## Chạy migration

Local có Go:

```bash
cd backend
go run ./cmd/migrate up
```

Local bằng Docker Compose dev:

```bash
docker compose -f deploy/docker/compose.dev.yml --profile app run --rm api migrate up
```

Production/VPS:

```bash
docker compose -f deploy/docker/compose.prod.yml --profile migration run --rm migrate
docker compose -f deploy/docker/compose.prod.yml up -d api worker web nginx
```

Sau khi migrate, restart API để backend nhận cấu hình `ORDER_*` mới.
