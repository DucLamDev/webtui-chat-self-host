# Ma trận chức năng WebTui Chat

Cập nhật: 2026-07-23

Quy ước:

- **Hoàn thành**: có backend và luồng sử dụng thực tế trên web hoặc có endpoint vận hành đầy đủ.
- **Một phần**: đã có nền tảng/API nhưng còn thiếu UI, worker hoặc tích hợp production.
- **Chưa có**: chưa nên quảng bá là tính năng hoạt động.
- **Ngoài Phase 1**: cần dự án hoặc hạ tầng riêng, không giả lập bằng dữ liệu tĩnh.

## 1. Authentication & User Management

| Chức năng | Trạng thái | Ghi chú |
|---|---|---|
| Đăng nhập Username/Email | Hoàn thành | `FindUserByIdentifier`, JWT access/refresh. |
| SSO OIDC theo zone | Hoàn thành | Authorization code + PKCE, nonce/state replay protection, JIT policy và UI web/admin; cần cấu hình IdP thật cho từng zone. |
| LDAP | Chưa có | Cần directory connector, sync lifecycle và mapping group/role riêng. |
| 2FA/MFA | Chưa có | Cần TOTP/WebAuthn, recovery code và quy trình khóa tài khoản. |
| Remember Login | Hoàn thành | Có lựa chọn lưu lâu dài hoặc chỉ trong phiên tab/browser. |
| CAPTCHA | Chưa có | Cần provider và site/secret key; nên kích hoạt theo rủi ro thay vì mọi lần đăng nhập. |
| Quản lý Session | Hoàn thành | Liệt kê thiết bị, IP, hạn dùng, thu hồi một hoặc tất cả phiên. |
| Giới hạn thiết bị | Một phần | Đã theo dõi session/device nhưng chưa có policy số thiết bị tối đa. |
| Hồ sơ người dùng | Hoàn thành | Tên, avatar upload/URL, số điện thoại và trạng thái. |
| Role & Permission | Hoàn thành | RBAC theo workspace, role assignment và permission guard. |

## 2. Workspace

| Chức năng | Trạng thái | Ghi chú |
|---|---|---|
| Nhiều workspace | Hoàn thành | API create/list/update/archive và chuyển workspace trên web. |
| Thành viên độc lập | Hoàn thành | Membership, role và dữ liệu đều scope theo workspace. |
| Bot/API/Webhook riêng | Hoàn thành | Token, bot installation và webhook đều có `workspace_id`. |
| Lưu trữ riêng | Một phần | Object key và quyền truy cập tách theo workspace; chưa cấp bucket vật lý riêng cho từng tenant. |

## 3. Chat và tin nhắn

| Chức năng | Trạng thái | Ghi chú |
|---|---|---|
| Chat 1-1 | Hoàn thành | Direct conversation và realtime. |
| Nhóm | Một phần | Schema có channel `group`; chưa có UX quản trị nhóm riêng như Messenger. |
| Channel | Hoàn thành | Public/private, member, invite, join request và owner approval. |
| Thread/Reply | Hoàn thành | `parent_id`, `thread_root_id`, panel luồng và composer riêng. |
| Forward | Hoàn thành | Chuyển tiếp sang kênh/direct khác, giữ nội dung và attachment, kiểm tra membership hai đầu. |
| Reaction/Pin/Mention | Hoàn thành | Có API, realtime/cache update và UI. |
| Chỉnh sửa/thu hồi | Hoàn thành | Kiểm tra chủ sở hữu; quản trị có thể xóa theo permission. |
| Text | Hoàn thành | Tối đa 8.000 ký tự. |
| Markdown | Hoàn thành cơ bản | Bold, inline code, code block và link HTTP(S); render an toàn không dùng `dangerouslySetInnerHTML`. |
| Hình ảnh/File/Voice | Hoàn thành | Upload, paste ảnh, preview/cache ảnh, audio recorder và attachment. |
| Video | Hoàn thành cơ bản | Upload/download và video player HTML5; chưa có transcoding/thumbnail server-side. |
| Emoji | Hoàn thành | Picker và reaction. |
| Sticker | Chưa có | Cần catalog/asset service và picker. |
| Poll/Task | Chưa có | Cần model, quyền cập nhật và UI kết quả/trạng thái. |
| System Message | Một phần | Schema hỗ trợ `system`, `bot`, `event`; chưa có đầy đủ producer cho mọi nghiệp vụ. |

## 4. Quản lý file

| Chức năng | Trạng thái | Ghi chú |
|---|---|---|
| Upload/Download | Hoàn thành | Local hoặc MinIO/S3, checksum SHA-256 và permission theo channel. |
| Preview | Một phần | Ảnh và audio có preview; video/PDF/Office chưa có viewer riêng. |
| Version | Hoàn thành API | Có `file_versions`; UI quản lý lịch sử version còn thiếu. |
| Virus Scan | Chưa có | Cần ClamAV/ICAP worker và trạng thái quarantine. |
| Download Log | Hoàn thành | Ghi `file.download` vào audit log. |
| Link hết hạn | Chưa có | Download hiện qua API có auth; chưa có signed share link TTL. |

## 5. Notification và Search

| Chức năng | Trạng thái | Ghi chú |
|---|---|---|
| Realtime | Hoàn thành | WebSocket cho chat/typing, polling fallback cho notification/presence. |
| Desktop | Hoàn thành | Browser Notification sau khi người dùng cấp quyền. |
| Email | Chưa có | Có notification job model nhưng chưa có mail sender/provider. |
| Web Push | Chưa có | Chưa có service worker, push subscription và VAPID. |
| Mute/Mention only | Một phần | Channel member có trạng thái muted và notification hiện tập trung mention; chưa có preference API đầy đủ. |
| Tìm tin nhắn | Hoàn thành | PostgreSQL FTS, chỉ tìm trong channel người dùng được phép xem. |
| Lọc ngày/người gửi/loại/kênh | Hoàn thành | Bộ lọc backend và web đã được bổ sung. |
| Tìm file | Hoàn thành cơ bản | Lọc tên/MIME trong danh sách file được phép xem. |
| Tìm user/channel | Hoàn thành cơ bản | User search và lọc channel/direct trên web. |

## 6. Bot, API, Webhook và Scheduler

| Chức năng | Trạng thái | Ghi chú |
|---|---|---|
| Bot API | Hoàn thành | Bot, installation và gửi message theo channel. |
| Slash Command | Chưa có | Cần command registry, parser và permission scope. |
| Incoming/Outgoing Webhook | Hoàn thành | Có secret, event routing và delivery worker. |
| Retry/Signature/Logs | Hoàn thành | Retry có backoff, HMAC signature và lịch sử delivery. |
| Replay | Chưa có | Chưa có endpoint phát lại một delivery đã chọn. |
| Workflow | Một phần | Ghép webhook + cronjob được; chưa có workflow designer/state machine. |
| OAuth cho bot/app | Chưa có | API token hiện dùng scope nội bộ. |
| Rate Limit | Hoàn thành cơ bản | Middleware HTTP rate limit; chưa có quota riêng theo bot/app/gói dịch vụ. |
| Cronjob | Hoàn thành | CRUD, lịch chạy, run-now, lock và lịch sử run. |
| Reminder/Cleanup/Sync | Một phần | Có cron engine tổng quát; chưa có các domain workflow dựng sẵn. |

## 7. AI Assistant

Tóm tắt, dịch, sinh nội dung, OCR, RAG và tra cứu tài liệu đều **chưa có**. Các mục này cần model provider, policy dữ liệu, quota, vector store và luồng đánh giá chất lượng; không nên gửi dữ liệu doanh nghiệp sang dịch vụ AI khi chưa có cấu hình/đồng ý rõ ràng.

## 8. Admin, Audit và Monitoring

| Chức năng | Trạng thái | Ghi chú |
|---|---|---|
| User/Message/Channel/File/Bot/Webhook stats | Hoàn thành | Dashboard lấy số liệu thật từ PostgreSQL. |
| Storage bytes, Top Channel, Top User | Chưa có | Types frontend có dự phòng nhưng query backend chưa trả các chỉ số này. |
| CPU/RAM/Disk | Chưa có | Nên lấy từ Prometheus/node-exporter thay vì đọc ad-hoc trong API. |
| Health/Database/Redis/Queue/Storage | Hoàn thành cơ bản | `/health`, `/ready`, admin health và metrics middleware. |
| Audit đăng nhập/quyền/file/xóa/forward | Hoàn thành | Có audit cho auth, RBAC và các thao tác nội dung quan trọng. |
| Audit API/Webhook | Một phần | Delivery log đầy đủ; audit actor cho mọi thay đổi cấu hình chưa phủ hết. |

## 9. API Gateway và Security

| Chức năng | Trạng thái | Ghi chú |
|---|---|---|
| REST/JWT/API Key | Hoàn thành | API v1, JWT và scoped API token. |
| OpenAPI/Swagger contract | Một phần | Có OpenAPI YAML và đã cập nhật search/forward; vẫn cần contract test để ngăn tài liệu lệch các route mới. |
| GraphQL | Chưa có | Không cần cho Phase 1 nếu REST đã đáp ứng client. |
| TLS | Hoàn thành ở cấu hình deploy | Nginx + Let's Encrypt; cần xác minh trên môi trường thật. |
| CSP/XSS/SQL injection | Hoàn thành cơ bản | Security headers, React escaping và query parameterized. |
| CSRF | Một phần | API chủ yếu dùng bearer token; cần đánh giá lại nếu chuyển refresh token sang cookie. |
| Secret Manager | Chưa có | Hiện dùng environment secret; production nên tích hợp Vault/KMS/secret store. |
| MFA | Chưa có | Trùng hạng mục 2FA phía trên. |

## 10. Backup và nền tảng client

| Chức năng | Trạng thái | Ghi chú |
|---|---|---|
| Database backup | Hoàn thành | `pg_dump`, lịch chạy, checksum, object storage và restore script. |
| Files/Redis/Config backup | Chưa có | Worker hiện chỉ chấp nhận loại `database`. |
| Restore one-click | Chưa có | Có script restore có kiểm soát; chưa có nút production do rủi ro phá hủy dữ liệu. |
| Desktop Windows/macOS/Linux | Ngoài Phase 1 | Chưa có Electron/Tauri app, auto-update, screen share, voice/video call. |
| Mobile Android/iOS | Ngoài Phase 1 | Chưa có app native, camera, biometric và native push. |

## 11. Enterprise Features

| Chức năng | Trạng thái | Ghi chú |
|---|---|---|
| Ticket | Chưa có | Màn hình web hiện là placeholder, chưa có domain/API ticket. |
| Approval/Calendar/Task | Chưa có | Chưa có model và workflow nghiệp vụ. |
| Knowledge Base | Chưa có | Search document table chưa phải hệ quản trị tri thức. |
| Announcement | Một phần | Có channel và system/event message; chưa có lịch phát hành/acknowledgement riêng. |
| Phòng ban | Hoàn thành | CRUD cây phòng ban, chống vòng lặp, tìm kiếm, trưởng phòng/thành viên và gán kênh theo phòng ban. |

## Thứ tự triển khai tiếp theo

1. Bảo mật tài khoản: CAPTCHA theo rủi ro, TOTP/WebAuthn, giới hạn thiết bị.
2. Notification production: preference/mute, email worker, Web Push + service worker.
3. File security: ClamAV quarantine, signed link TTL, preview video/PDF, UI version.
4. Enterprise core: Ticket + Approval + Task trước Calendar/Knowledge Base.
5. Observability: Prometheus, node-exporter, PostgreSQL/Redis exporters, dashboard CPU/RAM/Disk/top usage.
6. SSO tiếp theo: SCIM và group-to-role sync; LDAP là connector riêng sau OIDC.
7. AI, desktop và mobile là các workstream độc lập sau khi security/audit/backup đạt production readiness.
