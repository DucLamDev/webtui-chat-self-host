# Mẫu tin nhắn sử dụng bot VPSTTT

Các kênh `#ticket`, `#gia-han` và `#ke-toan` tự mở một phiên riêng cho từng người dùng. Chỉ người đang làm việc trong phiên mới đọc được nội dung và phản hồi của bot. Các kênh public như `#server-alert` hoạt động theo kiểu nhóm.

## CSKH Bot và Ticket Bot — `#ticket`

Tra cứu số dư ví:

```text
Tra ví
Email: khach@example.com
```

Hoặc tra theo mã người dùng:

```text
Tra ví
User ID: 6812
```

Tạo yêu cầu hỗ trợ:

```text
Khách: Nguyễn Văn A
Email: khach@example.com
Dịch vụ: VPS #1234
Lỗi: Không kết nối được SSH từ 09:30
Mức độ: Khẩn cấp
```

Yêu cầu tư vấn hoặc mua dịch vụ:

```text
Tôi muốn mua VPS cho website bán hàng.
Email: khach@example.com
Nhu cầu: 4 CPU, 8 GB RAM, 100 GB SSD, thời hạn 12 tháng
Khu vực: Việt Nam
Ngân sách: 500000/tháng
```

Yêu cầu mua theo gói đã biết:

```text
Tạo yêu cầu mua dịch vụ
Email: khach@example.com
Loại dịch vụ: VPS
Plan ID: 12
Server ID: 3
OS ID: 900
Số tháng: 12
Số lượng: 1
```

Ticket Bot tiếp nhận và phân loại yêu cầu mua. Khi hệ thống Order đã tạo Quick Order intent, người dùng gửi `intent_code` sang `#ke-toan` để Thanh Toán Bot tạo QR đúng số tiền của đơn hàng.

Yêu cầu phân loại ticket:

```text
Tạo ticket hỗ trợ cho khach@example.com
VPS #1234 bị mất kết nối, ping timeout và port 22 không truy cập được.
```

## Gia Hạn Bot — `#gia-han`

Thống kê toàn bộ dịch vụ sắp hết hạn:

```text
Email: khach@example.com
Số ngày: 7
Loại dịch vụ: Tất cả
```

Chỉ thống kê VPS:

```text
Kiểm tra dịch vụ sắp hết hạn
Email: khach@example.com
Số ngày: 30
Loại dịch vụ: VPS
```

Yêu cầu gia hạn một dịch vụ cụ thể:

```text
Tôi muốn gia hạn dịch vụ VPS #1234 của tài khoản khach@example.com thêm 1 tháng.
```

Yêu cầu gia hạn theo tên dịch vụ:

```text
Gia hạn dịch vụ vps-hanoi-01 của tài khoản khach@example.com trong 3 tháng.
```

Flow gia hạn tự động gọi endpoint nội bộ `/internal/services/renew` với `idempotency_key` là ID tin nhắn kích hoạt. Order API phải trả kết quả giao dịch, chi phí, số dư trước/sau và số tiền còn thiếu. Nếu Order API chưa triển khai endpoint này, bot thông báo rõ chưa có khoản tiền nào bị trừ và hướng người dùng sang Zalo OA hoặc `#ke-toan`; bot tuyệt đối không thông báo gia hạn thành công giả.

Khi số dư không đủ, phản hồi chuẩn:

```text
GIA HẠN · SỐ DƯ KHÔNG ĐỦ
Chi phí: 500.000 VND
Số dư ví: 350.000 VND
Số tiền còn thiếu: 150.000 VND

Vui lòng liên hệ Zalo OA VPSTTT hoặc gửi yêu cầu tại #ke-toan để nạp thêm tiền vào ví.
```

## Thanh Toán Bot — `#ke-toan`

Tạo QR nạp ví:

```text
Tạo QR nạp ví
Email: khach@example.com
Số tiền: 200000
```

Tạo QR nạp ví số tiền viết tắt:

```text
Nạp ví 500k cho tài khoản khach@example.com
```

Tạo QR cho một đơn hàng Quick Order:

```text
Tạo QR cho đơn hàng
Intent code: QOIABCD1234EFGH5678
```

Tạo QR theo intent ID:

```text
Tạo QR thanh toán đơn hàng
Intent ID: 12345
```

Tạo QR theo mã tham chiếu:

```text
Tạo QR thanh toán
Mã tham chiếu: QOHOMEORDER10001A1B2C3D4E5
```

QR đơn hàng luôn sử dụng số tiền do Order API chốt; bot không nhận số tiền tùy ý cho đơn hàng đã tồn tại.

## Server Alert Bot — `#server-alert`

Cảnh báo mất ping:

```text
Server: vps-01
Lỗi: Mất ping 3 phút
IP: 192.0.2.10
Mức độ: critical
```

Cảnh báo port hoặc dịch vụ:

```text
Server: web-prod-02
Port: 443 timeout
Dịch vụ: nginx
Thời gian bắt đầu: 10:15
Mức độ: critical
```

Cảnh báo tài nguyên:

```text
Server: db-prod-01
CPU: 95%
RAM: 91%
Disk: 88%
Mức độ: warning
```

## Lệnh trợ giúp

Trong kênh tương ứng, gửi một trong các nội dung:

```text
/help
```

```text
Hướng dẫn sử dụng bot
```

Không gửi password, API key, token hoặc keypass trong nội dung chat.
