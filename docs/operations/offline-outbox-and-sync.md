# Offline outbox và đồng bộ xung đột

Web/desktop webview dùng IndexedDB native để giữ tin nhắn chưa gửi. Cơ chế này
giúp người dùng tiếp tục soạn khi mạng chập chờn mà không cần thêm dịch vụ hay
thư viện phía client.

## Phạm vi được hỗ trợ

- `message.send` được phép vào outbox vì client tạo `client_message_id` và gửi
  đồng thời header `Idempotency-Key`. Backend có unique constraint theo
  workspace, channel, sender và client id, nên gửi lại sau timeout trả về cùng
  một message thay vì tạo bản sao.
- Sửa và xóa message **không** được xếp hàng offline. Hai endpoint hiện chưa có
  entity version, `If-Match` hoặc precondition tương đương; replay một lệnh cũ
  có thể ghi đè thay đổi mới trên server. UI chỉ thực hiện hai thao tác này khi
  request online và rollback optimistic state nếu request thất bại.
- File/voice chưa vào outbox: `File` của trình duyệt không được giữ lâu dài một
  cách tương thích trên mọi webview. Message có attachment chỉ được xác nhận sau
  khi upload hoàn tất.

Đây là delivery **at-least-once kết hợp idempotency**, không phải lời hứa
exactly-once ở tầng mạng.

## Ownership và vòng đời dữ liệu

Mỗi record mang đủ `serverId`, `userId`, `workspaceId`; key/index được tạo từ cả
ba thành phần. Một tab không thể list, claim, retry hoặc xóa record thuộc scope
khác. Sync checkpoint dùng cùng scope. Khi logout, token bị thu hồi hoặc chuyển
tài khoản/server, ứng dụng xóa toàn bộ outbox và checkpoint của tài khoản cũ
trước khi hoàn tất logout.

Outbox localStorage phiên bản cũ không có user/server ownership nên bị xóa, không
migrate. Quyết định này ưu tiên không làm lộ nội dung nháp giữa hai tài khoản.

IndexedDB thuộc browser profile và có chứa nội dung message dạng rõ. Máy dùng
chung cần profile OS/browser riêng và khóa ổ đĩa; không xem site storage là kho
bí mật đã mã hóa.

## Gửi lại và nhiều tab

Một flush xử lý tuần tự theo thời điểm tạo:

1. transaction IndexedDB claim record và đặt lease 30 giây;
2. gửi request kèm client id/idempotency key;
3. merge message canonical vào cache rồi xóa record;
4. nếu lỗi tạm thời, dừng để giữ thứ tự và đặt exponential backoff có jitter;
5. nếu lỗi vĩnh viễn, đánh dấu failed và tiếp tục record kế tiếp.

Lỗi `408`, `425`, `429`, `5xx` và lỗi network được retry; lỗi validation/auth/
permission/không tồn tại không tự retry. Backoff bắt đầu khoảng một giây, tăng
lũy thừa và cap 60 giây. Online/reconnect có thể flush ngay; retry tự động vẫn
tuân thủ deadline. Lease cùng unique constraint backend bảo vệ khi nhiều tab
cùng mở. `BroadcastChannel` chỉ dùng để cập nhật UI giữa tab, không phải cơ chế
đảm bảo đúng đắn.

Timeline hiển thị riêng `Chờ kết nối`, `Đang gửi`, `Gửi thất bại` và nút
`Thử lại`. Message local không được pin, reaction, mở thread, forward hay gọi
API message-id trước khi server trả canonical id.

## Cursor/delta và conflict policy

Sau mỗi lần WebSocket kết nối lại, client đọc checkpoint của đúng scope rồi gọi
`GET /api/v1/workspaces/:id/sync` theo cursor. Với mỗi page:

- dedup `event_id` đã xử lý;
- coalesce nhiều event của cùng message trong page;
- delete tạo tombstone và xóa cache;
- create/update/reaction/pin lấy lại message canonical bằng REST (payload sync
  hiện chỉ chứa id, không chứa snapshot);
- chỉ sau khi apply thành công mới lưu local checkpoint và gọi `/sync/ack`.

`event_version` hiện là version schema của event, không phải revision tăng dần
của aggregate, nên không được dùng như optimistic concurrency token. Conflict
message theo quy tắc:

1. server id luôn thắng local optimistic id có cùng `client_message_id`;
2. giữa hai server snapshot, `deleted_at/updated_at/edited_at/created_at` mới hơn
   thắng;
3. nếu timestamp bằng nhau, event/response đến sau thắng;
4. tombstone delete chặn response cũ đến trễ làm message xuất hiện lại.

Cursor lưu tối đa 500 event id gần nhất để replay sau crash vẫn idempotent. Nếu
apply REST thất bại, client không ACK; WebSocket vẫn hoạt động và reconnect sau
sẽ thử lại page đó. Web Locks serialize catch-up giữa nhiều tab cùng origin để
checkpoint/ACK của tab chậm không kéo cursor server lùi lại; browser quá cũ
không có Web Locks vẫn an toàn nhờ event/message dedup nhưng có thể replay thêm.

## Kiểm tra vận hành

Trong DevTools, chọn Application → IndexedDB → `webtui-chat-offline`. Hai object
store hợp lệ là `message-outbox` và `sync-checkpoints`.

Checklist smoke test:

1. tắt network, gửi hai text message và reload tab;
2. xác nhận hai message còn đúng thứ tự với trạng thái chờ;
3. bật network, xác nhận mỗi message chỉ có một server id;
4. gây lỗi `422`, xác nhận không có vòng retry tự động và nút Thử lại có thể dùng;
5. mở hai tab rồi reconnect, xác nhận không tạo message trùng;
6. logout/login tài khoản khác, xác nhận store của tài khoản cũ đã bị xóa;
7. ngắt WebSocket, tạo/sửa/xóa message ở client khác, reconnect và xác nhận delta
   được apply trước cursor ACK.
