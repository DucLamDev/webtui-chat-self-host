# Phase F2 - Design system và app shell

Phase F2 đã chuyển giao diện từ dạng JSX đóng cứng sang app shell có component, props, state và tương tác thật. Dữ liệu nghiệp vụ protected vẫn chưa gọi REST thật vì auth/session thuộc F4, nhưng cấu trúc hiện tại đã sẵn sàng để F3/F4 thay data source bằng backend API mà không viết lại UI.

## Trạng thái task

| Task | Trạng thái | Kết quả |
|---|---|---|
| F2.1 | Hoàn thành | Thêm `components.json` shadcn-compatible; PostCSS plugin đang để rỗng vì UI dùng CSS tokens thuần. |
| F2.2 | Hoàn thành | CSS tokens sống trong `@webtui/ui/styles.css`. |
| F2.3 | Hoàn thành | Tạo `NavigationRail` dùng chung cho web/admin. |
| F2.4 | Hoàn thành | Tạo channel/conversation shell dữ liệu hóa trong `ChatWorkspace`. |
| F2.5 | Hoàn thành | Tạo chat main shell có header, timeline, composer và state gửi tin nhắn. |
| F2.6 | Hoàn thành | Tạo right detail panel có tabs `Đã ghim`, `Ảnh`, `File`. |
| F2.7 | Hoàn thành | Tạo admin shell có dashboard, bảng người dùng và panel cấu hình. |
| F2.8 | Hoàn thành | Thêm UI primitives: Button, Input, Avatar, Badge, Card, SegmentedControl, Tooltip. |
| F2.9 | Hoàn thành | Thêm EmptyState, ErrorState, Skeleton, Toast. |
| F2.10 | Hoàn thành | Giữ responsive desktop/tablet/mobile theo breakpoint hiện có. |
| F2.11 | Hoàn thành một phần | Đã build kiểm tra; screenshot regression nên làm khi dev server chạy thường trực trong IDE. |

## Điểm đã sửa so với F1

- `apps/web/src/app/page.tsx` không còn chứa dữ liệu và JSX lớn; chỉ render `ChatWorkspace`.
- `apps/admin/src/app/page.tsx` không còn chứa dữ liệu và JSX lớn; chỉ render `AdminDashboard`.
- Snapshot source F2 đã được loại bỏ ở F3/F4. Web/admin hiện lấy dữ liệu qua TanStack Query và `@webtui/api-client`.
- Component chính giữ layout F2 nhưng thao tác production đi qua API:
  - Web: chọn kênh, lọc kênh, tìm kiếm, tạo kênh nhanh, gửi tin nhắn, upload/attach/download file, đổi tab panel phải.
  - Admin: chọn workspace, tìm kiếm người dùng, lọc trạng thái, xem stats/health/settings/users từ backend.
- Hai app đều gọi endpoint readiness công khai qua `@webtui/api-client`, không gọi `fetch` trực tiếp trong component.

## File chính

| Khu vực | File |
|---|---|
| UI primitives | `frontend/packages/ui/src/components/*` |
| UI patterns | `frontend/packages/ui/src/patterns/navigation-rail.tsx` |
| API health client | `frontend/packages/api-client/src/health-client.ts` |
| Web chat shell | `frontend/apps/web/src/features/chat/components/chat-workspace.tsx` |
| Admin dashboard | `frontend/apps/admin/src/features/dashboard/components/admin-dashboard.tsx` |

## Kiểm tra đã chạy

| Lệnh | Trạng thái |
|---|---|
| `npm.cmd install` | Thành công, lockfile đã cập nhật. |
| `npm.cmd run typecheck` | Thành công. |
| `npm.cmd run lint` | Thành công. |
| `npm.cmd run build:web` | Thành công. |
| `npm.cmd run build:admin` | Thành công. |

## Ranh giới với API backend

F2 đã có public API touchpoint qua `/ready`. Các API nghiệp vụ như workspace, channel, message, file, RBAC, admin stats đều cần JWT và sẽ được nối ở F3/F4 theo thứ tự:

1. F3 mở rộng `@webtui/api-client` thành module client typed.
2. F4 triển khai auth/session để có access token.
3. F5-F11 tách query/mutation hiện có thành feature hooks/use-case nhỏ hơn.
4. Component giữ layout/model F2, nhưng không khôi phục dữ liệu mẫu hoặc snapshot source.

## Handoff cho F3

- Không đưa dữ liệu mẫu vào component mới.
- Không gọi `fetch` trong component.
- Tạo client module theo backend map: `authClient`, `workspacesClient`, `channelsClient`, `messagesClient`, `filesClient`, `rbacClient`, `adminClient`.
- Ưu tiên test cho `HttpClient`, unwrap envelope, API error và query key factory.
- Giữ trạng thái tương tác F2 khi nối API: optimistic message, selected channel, right panel tab và filter người dùng.
