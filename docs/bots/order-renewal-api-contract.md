# Contract Order API cần có cho Gia Hạn Bot

Gia Hạn Bot phía WebTui Chat đã chuẩn bị client cho endpoint dưới đây. Tại thời điểm kiểm tra Swagger `https://order.vpsttt.com/openapi.yaml`, Order mới có `/internal/wallet/balance`, `/internal/wallet/deposit-qr` và `/internal/services/expiring`; chưa có endpoint gia hạn nội bộ.

Không được thay endpoint này bằng cách gọi `/vps/extend.php`, `/proxy/extend.php`, `/hosting/extend.php` hoặc `/s3/extend.php` với credential dùng chung. Các endpoint đó trừ tiền và yêu cầu token/keypass của tài khoản Order.

## Endpoint

```http
POST /api/internal/services/renew
X-API-Key: <ORDER_INTERNAL_API_KEY>
Content-Type: application/json
```

## Request

```json
{
  "email": "khach@example.com",
  "service_type": "vps",
  "service_id": 1234,
  "service_name": "vps-hanoi-01",
  "months": 1,
  "idempotency_key": "chat-message-uuid"
}
```

Quy tắc:

- Bắt buộc một trong `email` hoặc `user_id`.
- Bắt buộc một trong `service_id` hoặc `service_name`.
- `service_type`: `vps`, `proxy`, `hosting`, `s3`, `drive`, `waf`, `domain`, `separate` hoặc `all`.
- `months`: từ 1 đến 36.
- `idempotency_key` bắt buộc và unique theo giao dịch. Gọi lại cùng key phải trả cùng kết quả, không trừ tiền lần hai.
- Order phải khóa giao dịch/row khi kiểm tra số dư và trừ ví để tránh race condition.
- Không gia hạn khi dịch vụ không thuộc đúng tài khoản tra cứu.

## Thành công

```json
{
  "ok": true,
  "status": "success",
  "message": "Gia hạn thành công",
  "data": {
    "outcome": "renewed",
    "transaction_id": "REN-20260713-00001",
    "user": {
      "user_id": 6812,
      "email": "khach@example.com",
      "name": "Khách hàng",
      "balance": 400000
    },
    "service_type": "VPS",
    "service_id": 1234,
    "service_name": "vps-hanoi-01",
    "months": 1,
    "amount": 500000,
    "balance_before": 900000,
    "balance_after": 400000,
    "shortage_amount": 0,
    "expires_at_before": "2026-07-20",
    "expires_at_after": "2026-08-20"
  }
}
```

## Số dư không đủ

Đây là kết quả nghiệp vụ hợp lệ, nên ưu tiên trả HTTP `200` để client format chính xác số tiền thiếu:

```json
{
  "ok": true,
  "status": "success",
  "message": "Số dư không đủ",
  "data": {
    "outcome": "insufficient_balance",
    "user": {
      "user_id": 6812,
      "email": "khach@example.com",
      "name": "Khách hàng",
      "balance": 350000
    },
    "service_type": "VPS",
    "service_id": 1234,
    "service_name": "vps-hanoi-01",
    "months": 1,
    "amount": 500000,
    "balance_before": 350000,
    "balance_after": 350000,
    "shortage_amount": 150000,
    "expires_at_before": "2026-07-20",
    "expires_at_after": ""
  }
}
```

Khi `outcome=insufficient_balance`, Order không được trừ một phần số dư và không được thay đổi hạn dịch vụ.

## Lỗi cần phân biệt

| HTTP/code | Ý nghĩa |
|---|---|
| `400 VALIDATION_ERROR` | Thiếu/sai input |
| `401/403` | API key hoặc whitelist không hợp lệ |
| `404 USER_NOT_FOUND` | Không tìm thấy tài khoản Order |
| `404 SERVICE_NOT_FOUND` | Không tìm thấy dịch vụ thuộc tài khoản |
| `409 RENEWAL_IN_PROGRESS` | Giao dịch cùng dịch vụ đang xử lý |
| `422 SERVICE_NOT_RENEWABLE` | Loại/trạng thái dịch vụ không cho gia hạn |
| `429` | Rate limit |
| `503` | Order hoặc provider gia hạn chưa sẵn sàng |

## Yêu cầu bảo mật self-service

WebTui Chat cho role có `order.billing` thao tác cho khách hàng. Workspace member chỉ được self-service khi có `order.payment_request` và email yêu cầu trùng chính xác email của tài khoản chat đang active.

Không được cho user thường nhập một email tùy ý rồi trừ ví của email đó. Về lâu dài nên thay phép so sánh email bằng liên kết đã xác minh giữa user WebTui và user Order, ví dụ bảng `user_order_accounts` hoặc signed account-link token; Order cũng nên kiểm tra target account thuộc identity đã liên kết thay vì chỉ tin chuỗi email.

## Checklist triển khai Order

- [ ] Thêm route và schema vào OpenAPI.
- [ ] Xác thực `X-API-Key` và whitelist.
- [ ] Lookup user/service trong cùng transaction.
- [ ] Preview/tính giá phía server, không nhận giá từ Chat.
- [ ] Kiểm tra số dư và tính `shortage_amount`.
- [ ] Idempotency unique + lưu response hoàn chỉnh.
- [ ] Trừ ví và gia hạn atomic hoặc có compensation rõ.
- [ ] Audit actor, service, amount, balance trước/sau.
- [ ] Không log token/keypass/plain secret.
- [ ] Test success, insufficient, duplicate request, race, rollback và provider error.
