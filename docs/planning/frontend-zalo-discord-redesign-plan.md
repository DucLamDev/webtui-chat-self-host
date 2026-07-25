# Kế hoạch thiết kế lại frontend WebTui Chat

Mục tiêu: WebTui Chat phải hoạt động như Zalo cho nhắn tin 1-1, bạn bè, lời mời kết bạn và thông báo; đồng thời dùng mô hình giống Discord cho workspace, kênh, bot và automation. Người dùng thông thường không phải thấy hoặc hiểu các chi tiết kỹ thuật như API, token, backend hay workspace nội bộ.

## Nguyên tắc sản phẩm

- Nhắn tin riêng là luồng mặc định: tìm bạn bè, gửi lời mời, chấp nhận, mở hội thoại và nhắn tin.
- Workspace là hạ tầng nền, không phải bước bắt buộc trong trải nghiệm người dùng phổ thông.
- Kênh và bot là luồng riêng cho nhóm, thông báo chung và automation.
- Tất cả dữ liệu động phải lấy từ API, không dùng dữ liệu áp cứng cho màn hình production.
- Realtime phải là tiêu chuẩn cho tin nhắn, typing, thông báo, presence và cập nhật lời mời.
- Quyền hạn phải rõ: user thường không vào admin; chỉ chủ tin nhắn được sửa tin nhắn của mình, nhưng tin nhắn quá 1h sẽ không sửa được

## P0 - Sửa lỗi luồng hiện tại

- Không tự chọn hội thoại/kênh đầu tiên khi URL chưa có `channel`.
- Khi mở hội thoại với B, route phải trỏ đúng channel của B và không nhảy sang A.
- Sau khi gửi tin nhắn, danh sách hội thoại phải cập nhật preview tin mới ngay.
- Lời mời kết bạn phải tự cập nhật ở tài khoản bên nhận.
- Sidebar chỉ hiển thị danh sách hội thoại khi đang ở tab Tin nhắn.
- Dark mode phải đọc rõ nội dung trong ô nhập tin nhắn.

Tiêu chí nghiệm thu:
- A kết bạn B, B thấy lời mời trong popup thông báo sau tối đa vài giây.
- B chấp nhận, A/B mở đúng hội thoại của nhau.
- Gửi tin xong preview đổi từ “Chưa có tin nhắn” sang nội dung mới.

## P1 - Khung giao diện Zalo

- Sidebar trái chỉ gồm điều hướng chính, avatar và trạng thái người dùng.
- Panel hội thoại gồm tìm kiếm, tab Tất cả/Chưa đọc/Yêu thích, danh sách hội thoại dạng Zalo.
- Chat box có header gọn, timeline nền nhẹ, bubble nhỏ, composer một dòng.
- Mobile/tablet có layout dạng drawer, không ép 4 cột.

Tiêu chí nghiệm thu:
- Màn hình 1366x768 không bị scroll ngang.
- Bubble tin nhắn không chiếm quá nhiều chiều cao.
- Chế độ sáng/tối đồng nhất.

## P2 - Bạn bè và danh bạ

- Tìm người dùng bằng email, username, số điện thoại.
- Gửi, hủy, chấp nhận, từ chối lời mời kết bạn.
- Danh bạ phân nhóm: Bạn bè, Lời mời đến, Đã gửi, Gợi ý.
- Sau khi chấp nhận có nút “Nhắn tin” mở đúng hội thoại.

API cần đảm bảo:
- `GET /contacts`
- `GET /contacts/requests`
- `POST /contacts/requests`
- `POST /contacts/requests/:id/accept`
- `POST /contacts/requests/:id/reject`
- `DELETE /contacts/requests/:id`

## P3 - Nhắn tin riêng realtime

- Gửi/sửa/xóa/ghim/reaction tin nhắn.
- Typing indicator 3 chấm realtime.
- Read receipt và unread badge.
- Attach ảnh, file, voice message.
- Tìm kiếm tin nhắn trong hội thoại.

API/WebSocket cần đảm bảo:
- REST message list/send/update/delete/pin/reaction.
- WebSocket events: `MessageCreated`, `MessageUpdated`, `MessageDeleted`, `TypingStarted`, `TypingStopped`, `ReadStateUpdated`.
- Browser WebSocket dùng được qua cookie, query token hoặc subprotocol.

## P4 - Thông báo kiểu popup

- Bell nằm trên panel hội thoại, không đặt ở sidebar chính.
- Popup hiển thị lời mời kết bạn, mention, tin ghim, file, bot alert.
- Có mark read, mark all read.
- Realtime hoặc polling fallback khi WebSocket mất kết nối.

## P5 - Kênh và bot kiểu Discord

- Kênh là luồng riêng, không trộn với nhắn tin 1-1.
- Tạo kênh public/private, danh sách thành viên, quyền gửi tin.
- Bot có trang quản lý riêng: bot profile, token, cài vào kênh, gửi test message.
- Automation/webhook/cronjob nằm trong khu vực admin hoặc power user.

## P6 - Hồ sơ và cài đặt cá nhân

- Cập nhật tên, avatar, số điện thoại, mật khẩu.
- Cài đặt sáng/tối, thông báo, quyền riêng tư.
- Phiên đăng nhập và đăng xuất thiết bị.

## P7 - Admin và phân quyền

- User thường không đăng nhập được admin.
- Admin guard kiểm tra role/permission từ backend trước khi render.
- Dashboard admin lấy dữ liệu health/stats thật, không ghi tĩnh.
- RBAC rõ: owner/admin/manager/staff/member.

## P8 - Hoàn thiện triển khai

- Docker frontend production cho `https://chat.vpsttt.com`.
- ENV rõ ràng: `NEXT_PUBLIC_API_BASE_URL=https://chat.vpsttt.com`.
- CI chạy typecheck, lint, build, Docker build web/admin.
- Health check frontend và smoke test login.

## P9 - QA bắt buộc

- Test E2E: register, login, search friend, send request, accept request, direct chat, send file, pin message.
- Test quyền: B không sửa được tin của A; user thường không vào admin.
- Test responsive: desktop, laptop 1366, tablet, mobile.
- Test dark mode toàn bộ input, modal, dropdown, composer.

