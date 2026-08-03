# Admin Panel: hướng dẫn sử dụng và roadmap vận hành

Admin Panel là giao diện dành cho quản trị viên workspace và operator của hệ
thống WebTui Chat. Với bản self-host mặc định, truy cập:

```text
https://chat.company.com/admin
```

Tài liệu này mô tả chức năng đang có, cách điều hướng, quy tắc RBAC, các
guardrail cần tuân thủ và roadmap hoàn thiện. UI chỉ hỗ trợ người dùng thao tác;
backend vẫn là lớp bắt buộc phải xác thực quyền cho mọi request.

## Điều hướng

Sidebar được chia thành bốn nhóm để giảm số mục phẳng và giúp operator tìm đúng
ngữ cảnh. Trên desktop có thể thu gọn sidebar; lựa chọn này được giữ trên trình
duyệt hiện tại. Trên mobile, sidebar trở thành drawer và có thể đóng bằng nút
đóng, backdrop hoặc phím `Escape`.

### Giám sát

#### Tổng quan

- chỉ số thành viên hoạt động, kênh, tin nhắn và file;
- hoạt động hệ thống và các kênh có nhiều hoạt động;
- audit log gần nhất nếu tài khoản có `audit.view`;
- health check của backend và các dependency được backend công bố.

Các số liệu tổng quan là ảnh chụp theo workspace đang chọn, không thay thế
Grafana/Prometheus trong điều tra sự cố dài hạn.

#### Push notification

- queue depth, số job chờ và đang xử lý;
- số job gửi thành công, bỏ qua và dead-letter trong 24 giờ;
- delivery rate, provider outcome và tuổi job cũ nhất;
- biểu đồ delivery theo giờ;
- dead-letter gần nhất và thao tác replay khi có `notification.manage`.

Trang Push tự làm mới mỗi 15 giây khi tab đang hoạt động. Token thiết bị,
subscription và payload không được trả về dashboard. Replay tạo job mới và giữ
job cũ làm bằng chứng điều tra; thao tác này không sửa lịch sử.

### Workspace

#### Tin nhắn

- tìm theo nội dung, kênh hoặc người gửi;
- lọc theo loại tin nhắn và người gửi;
- xem tối đa tập bản ghi mới nhất mà admin API trả về;
- xuất tập kết quả đang lọc thành CSV.

Dashboard không phải công cụ eDiscovery. Việc tìm kiếm trên toàn bộ lịch sử,
retention, legal hold và moderation nằm trong roadmap.

#### Kênh

- thống kê tổng số kênh công khai, riêng tư và phiên bot riêng;
- tìm theo tên hoặc slug;
- lọc loại và trạng thái;
- xem số thành viên, số tin nhắn và thời điểm cập nhật.

#### Người dùng

- tìm và lọc tài khoản theo trạng thái;
- khóa hoặc mở khóa tài khoản khi có `user.manage`;
- tạo lời mời theo email/role khi có `workspace.invite_user`;
- thêm người dùng vào workspace và đổi trạng thái membership khi có
  `workspace.manage`;
- xem danh sách thành viên khi có `workspace.view_members`.

Invite token chỉ hiển thị một lần. Sao chép token sang kênh truyền an toàn ngay
sau khi tạo; không đưa token vào ticket công khai, log hoặc ảnh chụp màn hình.

#### Vai trò và quyền

- xem role hệ thống và role tùy chỉnh;
- tạo role từ permission catalog;
- gán hoặc gỡ role của thành viên;
- xem audit log liên quan nếu có `audit.view`.

Áp dụng nguyên tắc đặc quyền tối thiểu: tạo role theo nhiệm vụ cụ thể, không cấp
`workspace.manage` hoặc quyền quản lý integration chỉ để giải quyết một thao tác
tạm thời.

### Mở rộng

#### Tích hợp

- tạo và thu hồi API token theo scope;
- tạo, bật/tắt và xóa incoming webhook;
- tạo, bật/tắt, xóa và gửi test outgoing webhook;
- xem delivery log của outgoing webhook.

API token và webhook secret mới chỉ hiển thị một lần. Không dùng chung token
giữa môi trường hoặc integration. Khi nghi ngờ lộ secret, tạo secret mới, cập
nhật consumer, kiểm tra delivery rồi mới thu hồi secret cũ.

#### Automation

- xem template tương thích với zone hiện tại;
- cài automation bằng config và secret reference;
- bật/tắt hoặc gỡ installation;
- xem trạng thái runtime của installation.

Ưu tiên secret reference từ secret manager; không nhập plaintext secret vào
config JSON.

#### Bot

- tạo bot;
- cài bot ở phạm vi workspace hoặc một kênh;
- xem installation của bot đang chọn;
- gửi tin nhắn test tới kênh.

Tin nhắn test đi qua API thật. Chỉ gửi vào kênh thử nghiệm nếu bot có thể kích
hoạt automation hoặc webhook bên ngoài.

### Vận hành

#### Tác vụ định kỳ

- tạo cronjob với schedule, runner và payload;
- bật, tạm dừng, chạy ngay hoặc xóa job;
- xem trạng thái, thời lượng và log các lần chạy.

Kiểm tra timezone và payload trước khi bật job. Với tác vụ có side effect, nên
tạo ở trạng thái tạm dừng, chạy thử một lần rồi mới kích hoạt lịch.

#### Sao lưu

- tạo backup job cho local storage, MinIO hoặc S3;
- đặt lịch và trạng thái;
- chạy backup thủ công;
- xem lịch sử, dung lượng, checksum/object key hoặc lỗi.

Trang này quản lý job của backend. Backup off-site Restic và quy trình restore
có guardrail được mô tả riêng tại
[Backup off-site và restore](offsite-backup-restore.md). UI restore wizard và
restore drill định kỳ chưa phải chức năng hoàn chỉnh của Admin Panel.

#### Cài đặt

- xem health check;
- cập nhật zone, logo và chế độ đăng ký;
- quản lý domain, deployment request, quota và OIDC provider theo loại zone;
- đọc và cập nhật workspace setting dạng typed JSON.

Thay đổi domain, OIDC, quota hard-limit, vòng đời zone hoặc raw setting có thể
ảnh hưởng toàn bộ người dùng. Hãy chụp lại cấu hình trước thay đổi và kiểm tra
đăng nhập bằng một phiên admin thứ hai trước khi đóng phiên hiện tại.

## Deep-link và workspace context

Section và workspace được giữ trong query string. Ví dụ:

```text
/admin?section=push&workspace=<workspace-id>
/admin?section=users&workspace=<workspace-id>
/admin?section=settings&workspace=<workspace-id>
```

- `section` không hợp lệ tự quay về `overview`;
- đổi section dùng browser history, nên Back/Forward hoạt động;
- đổi workspace giữ nguyên section hiện tại;
- có thể bookmark hoặc gửi deep-link, nhưng người nhận vẫn phải có quyền trên
  workspace đó;
- luôn kiểm tra tên workspace trong heading trước khi chạy, xóa hoặc thay đổi
  cấu hình.

Deep-link không chứa credential. Không thêm invite token, API token, webhook
secret hoặc thông tin nhạy cảm vào URL.

## RBAC và an toàn thao tác

### Permission đang được UI sử dụng

| Permission | Phạm vi UI |
| --- | --- |
| `admin.view` | Mở dashboard và đọc dữ liệu quản trị cơ bản |
| `audit.view` | Xem audit log |
| `workspace.view_members` | Xem danh sách thành viên |
| `workspace.invite_user` | Tạo lời mời |
| `workspace.manage` | Membership, setting, zone/domain/quota/OIDC và một số automation |
| `user.manage` | Khóa hoặc mở khóa tài khoản |
| `message.manage` | Đọc nội dung tin nhắn trong công cụ kiểm duyệt admin |
| `role.manage` | Tạo, gán và gỡ role |
| `api_token.manage` | Tạo và thu hồi API token |
| `webhook.manage` | Quản lý webhook và delivery test |
| `bot.manage` | Quản lý bot và installation |
| `notification.manage` | Replay push dead-letter |
| `cronjob.manage` | Quản lý và chạy cronjob |
| `backup.manage` | Tạo và chạy backup job |

Một số automation chấp nhận `workspace.manage`, `bot.manage` hoặc
`webhook.manage` tùy loại. UI ẩn hoặc disable control không đủ để bảo vệ dữ
liệu; API phải kiểm tra lại workspace membership và permission ở server.
Directory user thường chỉ thấy tài khoản có workspace active chung; metadata IP và
thiết bị chỉ trả cho chính chủ hoặc người có `user.manage`. API role kiểm tra lại
zone, workspace và `admin.view`/`role.manage` trước khi đọc dữ liệu.

### Guardrail bắt buộc

- Không chia sẻ tài khoản admin; cấp role theo người dùng và truy vết qua audit.
- Dùng tài khoản khác với tài khoản chat hằng ngày cho thao tác vận hành nhạy
  cảm nếu mô hình tổ chức cho phép.
- Với secret chỉ hiển thị một lần, lưu thẳng vào secret manager rồi đóng phần
  hiển thị.
- Với replay/run-now, kiểm tra idempotency và side effect của consumer.
- Với archive/suspend zone, xóa domain/OIDC/webhook/automation/cronjob hoặc thu
  hồi token, cần confirmation dialog nêu rõ tên đối tượng và hậu quả.
- Archive zone nên yêu cầu nhập lại zone slug; thay đổi quota hard-limit phải
  hiển thị usage hiện tại và cảnh báo nếu limit mới thấp hơn usage.
- Mọi thay đổi cấu hình quan trọng phải tạo audit event phía backend.

UI hiện yêu cầu xác nhận trình duyệt trước các thao tác chính như thu hồi role/token,
xóa webhook/automation/cronjob/domain/OIDC và suspend/archive zone. Typed
confirmation theo zone slug cùng dialog dùng chung hiển thị blast radius vẫn là
hạng mục P0; operator phải luôn kiểm tra lại workspace và tên đối tượng.

## Hiệu năng và trạng thái dữ liệu

Dashboard dùng query theo section: ngoài workspace và permission context, dữ
liệu của một module chỉ tải khi module đó đang mở. Điều này tránh tải đồng thời
messages, integrations, bots, cronjob và backup ngay ở trang Tổng quan.

- cache query dùng chế độ `offlineFirst` và giữ dữ liệu không dùng trong một
  khoảng ngắn để quay lại section nhanh hơn;
- hover hoặc focus mục điều hướng sẽ làm ấm query chính của section nếu người dùng
  có quyền, giảm thời gian chờ sau khi mở trang;
- nút làm mới chỉ refetch các query đang active;
- Push có polling riêng 15 giây và dừng polling nền;
- tìm kiếm global chỉ xuất hiện tại Tin nhắn, Kênh và Người dùng;
- đổi section xóa search cũ để không vô tình lọc màn hình mới.

Không hiểu skeleton hoặc số `0` là bằng chứng hệ thống khỏe. Khi query lỗi, UI
phải phân biệt rõ `loading`, `empty`, `permission denied`, `offline`, `stale` và
`error`, đồng thời cung cấp retry. Hoàn thiện trạng thái này trên toàn bộ module
là P0.

Admin Panel yêu cầu endpoint tổng hợp `/admin/messages` và không còn fan-out một
request cho từng channel. Với workspace lớn, vẫn cần server-side pagination/filter
trước khi coi màn hình là công cụ tìm kiếm toàn hệ thống.

## Mobile và bàn phím

### Mobile

- mở sidebar bằng nút menu ở header;
- chạm backdrop hoặc nút đóng để đóng drawer;
- bảng rộng có thể cuộn ngang; luôn kiểm tra cột thao tác trước khi bấm;
- không dùng mobile cho archive zone, thay đổi quota hard-limit hoặc thao tác
  restore cho tới khi typed confirmation và responsive action sheet hoàn tất.

### Bàn phím

- dùng `Tab` và `Shift+Tab` để di chuyển qua control;
- nhấn `/` để focus ô tìm kiếm tại Tin nhắn, Kênh hoặc Người dùng khi con trỏ
  không ở trong input;
- nhấn `Escape` để đóng menu mobile;
- dùng liên kết “Bỏ qua điều hướng” khi focus đầu trang để tới nội dung chính;
- dùng browser Back/Forward để chuyển section đã mở bằng deep-link.

Mọi hành động chỉ có icon phải có accessible name; bảng phải dùng semantic
`table`, `th`, `td`; biểu đồ phải có tóm tắt dữ liệu ngoài màu sắc và tooltip.
Các yêu cầu này là tiêu chí nghiệm thu, không chỉ là phần trang trí UI.

## Hướng dẫn sử dụng nhanh

1. Mở `/admin` và đăng nhập bằng tài khoản được cấp `admin.view`.
2. Chọn đúng workspace ở header; xác nhận lại tên workspace trong heading.
3. Mở nhóm chức năng trong sidebar. Có thể bookmark URL sau khi chọn section.
4. Dùng `/` để tìm nhanh trong các trang có search hoặc dùng bộ lọc trong trang.
5. Nhấn nút làm mới nếu cần dữ liệu ngay; Push tự cập nhật mỗi 15 giây.
6. Nếu control bị disable, kiểm tra permission notice thay vì thử bằng tài khoản
   có quyền rộng hơn.
7. Sau mutation, chờ thông báo thành công và kiểm tra lại bảng/audit trước khi
   thực hiện bước phụ thuộc tiếp theo.
8. Khi gặp lỗi, thử refetch một lần. Nếu vẫn lỗi, kiểm tra API health,
   observability dashboard và audit log trước khi lặp lại mutation.

### Triage push nhanh

1. Kiểm tra queue depth và tuổi job cũ nhất.
2. So sánh delivery rate với số dead-letter và failed đang retry.
3. Xác định provider có outcome bất thường.
4. Mở lỗi dead-letter đã được redacted.
5. Chỉ replay sau khi nguyên nhân provider/config đã được xử lý.
6. Theo dõi job replay mới; không dùng replay liên tục như cơ chế retry thủ công.

### Kiểm tra backup nhanh

1. Xác nhận target và schedule của job.
2. Chạy thủ công nếu cần kiểm tra credential/storage.
3. Chờ run hoàn tất và kiểm tra byte size, checksum/object key.
4. Đối chiếu snapshot off-site bằng Restic.
5. Thực hiện restore drill theo runbook; “backup thành công” chưa chứng minh dữ
   liệu có thể khôi phục.

## Roadmap

### P0 — an toàn và đủ điều kiện production

- thay xác nhận trình duyệt bằng dialog dùng chung cho revoke/delete/suspend/archive;
  thêm typed confirmation cho archive zone và thao tác có blast radius lớn;
- hoàn thiện trạng thái loading/error/empty/offline/stale nhất quán trên các
  module còn lại; Tổng quan, workspace và zone đã tách lỗi khỏi trạng thái rỗng;
- server-side pagination, filter và sort cho users/messages/channels/audit;
  bổ sung tổng số bản ghi và cursor ổn định cho export;
- chuyển các `div role="table"` còn lại sang semantic table, bổ sung focus trap
  cho mobile drawer và axe/keyboard regression test;
- chuyển refresh token admin sang HttpOnly/SameSite cookie nếu deployment model
  hỗ trợ; access token không được persist lâu dài trong local storage;
- Playwright authenticated fixture cho mọi nhóm, permission matrix, mobile,
  destructive confirmation và accessibility regression;
- restore wizard có chọn snapshot rõ ràng, checksum, safety snapshot, maintenance
  mode, health check và automatic rollback theo runbook hiện có;
- trung tâm cảnh báo tối thiểu: active alerts, p95/p99, API/worker/relay health,
  push queue và backup stale.

### P1 — tăng năng suất vận hành

- audit explorer có filter thời gian/actor/action/entity, diff và export;
- bulk user import/export, resend/revoke invite, khóa có thời hạn, revoke session
  và thiết bị;
- permission matrix, effective-permission preview, clone/edit/delete role;
- quản trị channel: archive, owner, membership, retention và dung lượng;
- push detail theo provider, test notification, bulk replay có giới hạn và lịch
  sử replay;
- rotate API token/webhook secret, last-used, IP allowlist và per-token rate
  limit;
- bot/automation run log, versioning, test sandbox, secret reference validation
  và rollback;
- backup restore drill history, RPO/RTO, retention visualization và cảnh báo
  dung lượng;
- OIDC connection test, domain/TLS diagnostic và kiểm tra login trước khi bật
  provider.

### P2 — quản trị enterprise

- SCIM/LDAP provisioning, group-to-role mapping và access review định kỳ;
- MFA enforcement, session policy, IP/network policy và phát hiện đăng nhập bất
  thường;
- content report/moderation, quarantine, retention, legal hold và eDiscovery;
- feature flags và staged rollout theo zone/workspace/user cohort;
- storage analytics, cleanup policy và dự báo quota;
- self-host update center: phiên bản, migration chờ chạy, release note,
  certificate và maintenance window;
- approval workflow bốn mắt cho thay đổi quyền, archive, restore và secret
  rotation;
- custom dashboard/SLO, error budget và incident timeline liên kết trace/log.

## Tài liệu liên quan

- [Observability, p95/p99 và cảnh báo](observability.md)
- [Push notification](push-notifications.md)
- [Backup off-site và restore](offsite-backup-restore.md)
- [Checklist production](production-checklist.md)
- [Auth, user và RBAC](../../backend/docs/auth-rbac.md)
- [Security baseline](../../backend/docs/security-baseline.md)
