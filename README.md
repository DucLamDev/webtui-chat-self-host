# VPSTTT Chat — self-hosted

Nền tảng chat cộng tác đa nền tảng gồm web, admin, mobile và desktop. Mỗi tổ
chức có thể chạy một instance độc lập trên domain của mình; dữ liệu hội thoại,
file và khóa hệ thống nằm trên hạ tầng do tổ chức quản lý.

> Trạng thái hiện tại: phù hợp để triển khai một VPS đơn cho nhóm nhỏ và vừa.
> Web Push, push relay, backup off-site và observability đã có dưới dạng cấu hình/profile
> opt-in; quickstart vẫn giữ chúng tắt cho đến khi operator cấp credential. High
> availability chưa nằm trong stack mặc định; xem [các giới hạn và lộ trình](docs/planning/self-host-feature-roadmap.md).

Cuộc gọi nhóm dùng Jitsi được đóng gói trong stack self-host; installer tự tạo
cấu hình và không chuyển media cuộc họp qua một dịch vụ Jitsi công cộng.

## Bắt đầu nhanh

### Yêu cầu

- Ubuntu 22.04 hoặc 24.04 LTS, IPv4 public;
- tối thiểu 4 vCPU, 8 GB RAM, 40 GB SSD;
- một domain đã có bản ghi `A` trỏ về VPS;
- mở TCP `80`, `443`, `3478`, `8443` và UDP `443`, `3478`, `10000`, `49160-49200`;
- Docker Engine và Docker Compose v2 (bootstrap có thể tự cài).

### Cài trên VPS Ubuntu mới

```sh
git clone https://github.com/DucLamDev/webtui-chat-self-host.git vpsttt-chat
cd vpsttt-chat
sudo sh deploy/self-hosted/bootstrap-ubuntu.sh \
  --domain chat.company.com \
  --email admin@company.com \
  --name "Company Chat"
```

Nếu Docker đã sẵn sàng, chạy thẳng installer:

```sh
sh deploy/self-hosted/install.sh \
  --domain chat.company.com \
  --email admin@company.com \
  --name "Company Chat"
```

Installer kiểm tra DNS, sinh secret bằng nguồn ngẫu nhiên, đặt quyền `0600` cho
`deploy/self-hosted/.env`, build image, chạy migration và chờ endpoint `/ready`.
Không dùng `--force` trên instance đã có dữ liệu.

Sau khi hoàn tất:

- web: `https://chat.company.com`;
- admin: `https://chat.company.com/admin`;
- health: `https://chat.company.com/ready`;
- WebSocket: `wss://chat.company.com/ws`;
- discovery cho client: `https://chat.company.com/api/v1/discovery?domain=chat.company.com`.

Tài khoản đăng ký đầu tiên trở thành workspace owner. Sau đó nên đổi chế độ
đăng ký từ `open` sang `invite_only` hoặc `closed` trong Admin Panel.

Hướng dẫn đầy đủ, gồm lỗi DNS/TLS và tùy chọn installer, nằm tại
[deploy/self-hosted/README.md](deploy/self-hosted/README.md).

## Kiến trúc mặc định

| Thành phần | Vai trò | Public Internet |
| --- | --- | --- |
| Caddy | TLS, HTTP/3, reverse proxy | `80`, `443` |
| Web / Admin | giao diện người dùng và quản trị | qua Caddy |
| Go API / Worker | REST, WebSocket, job nền | qua Caddy / nội bộ |
| PostgreSQL | dữ liệu nghiệp vụ | không |
| Redis / RabbitMQ | realtime đa node, hàng đợi | không |
| Storage volume | file tải lên và backup local | file được Caddy phục vụ có kiểm soát |
| coturn | relay media WebRTC | `3478`, dải UDP media |

Stack dùng một origin để đơn giản hóa TLS, CORS và kết nối client. TURN credential
được cấp ngắn hạn qua API đã xác thực; `TURN_SHARED_SECRET` không đi vào bundle
web/mobile.

## Push notification

Không có một cấu hình push duy nhất phù hợp cho mọi kiểu self-host. Chọn đúng
một chế độ:

| Trường hợp | Cấu hình nên dùng |
| --- | --- |
| App iOS/Android official trên store | relay do publisher vận hành, token riêng cho từng instance |
| App mobile do tổ chức tự build và ký | FCM service account và APNs key của chính tổ chức |
| Không chấp nhận dịch vụ push ngoài | để trống toàn bộ cấu hình; vẫn có WebSocket và notification center khi online |

Các biến relay mặc định để trống. Chỉ điền `PUSH_RELAY_URL`, `PUSH_RELAY_TOKEN`
và `PUSH_RELAY_INSTANCE_ID` sau khi publisher cấp đủ bộ; không dùng giá trị mẫu
và không dùng chung token giữa các VPS. Khóa FCM/APNs của app official không
được phân phát cho customer.

Hiện trạng theo client:

- mobile: đăng ký FCM token; iOS đăng ký thêm APNs VoIP token cho cuộc gọi đến;
- desktop: notification native khi tiến trình desktop đang chạy;
- web: service worker + Web Push/VAPID theo từng instance, mặc định tắt và chỉ
  xin quyền sau thao tác opt-in; khi chưa cấu hình vẫn dùng notification center
  và realtime lúc tab đang chạy.

Thiết kế, cấu hình direct/relay, kiểm thử thiết bị thật và ranh giới dữ liệu được
mô tả tại [docs/operations/push-notifications.md](docs/operations/push-notifications.md).

Admin Panel có trang **Push** theo dõi queue depth, delivery rate 24 giờ, tuổi job
cũ nhất, phân bổ provider và dead-letter đã che dữ liệu nhạy cảm. Người có quyền
`notification.manage` có thể replay một dead-letter theo cơ chế idempotent.

Web client có outbox IndexedDB tách theo server/tài khoản/workspace. Tin nhắn text
được gửi lại với `client_message_id`, backoff và lease đa tab; delta sync áp dụng
server timestamp, tombstone và cursor bền vững để tránh hồi sinh dữ liệu cũ. Xem
[chính sách offline và conflict](docs/operations/offline-outbox-and-sync.md).

## Vận hành

```sh
# Kiểm tra container, HTTPS, chế độ push và hàng đợi notification
sh deploy/self-hosted/check.sh

# Theo dõi log quan trọng
cd deploy/self-hosted
docker compose --env-file .env -f compose.yml logs -f api worker caddy

# Backup off-site trước khi update (sau khi cấu hình S3/MinIO và init một lần)
./backup.sh backup --maintenance
sh update.sh

# Restore có chủ đích từ snapshot ID cụ thể
./restore.sh 0123abcd --apply --confirm RESTORE:0123abcd

# Bật tracing, dashboard p95/p99 và cảnh báo nội bộ (Grafana chỉ bind localhost)
docker compose --env-file .env -f compose.yml --profile observability up -d
```

Backup off-site chứa database, file/object storage, manifest và checksum trong
repository Restic mã hóa phía client. Tính năng mặc định tắt và không làm đổi
quickstart. Cấu hình bucket, retention, scheduler, verify và restore guardrail
nằm trong [runbook backup/restore](docs/operations/offsite-backup-restore.md);
quy trình ngày/tuần/tháng, xoay secret và xử lý sự cố nằm trong
[runbook vận hành](docs/operations/self-host-runbook.md).

Profile observability gồm OpenTelemetry Collector, Tempo, Prometheus,
Alertmanager và Grafana. Cấu hình dashboard, ngưỡng p95/p99, push/backup alert và
cách nối webhook/email riêng nằm trong
[runbook observability](docs/operations/observability.md).

Trước mỗi bản production, dùng
[checklist phát hành](docs/operations/production-checklist.md) làm điều kiện go/no-go.

## Bảo mật và quyền riêng tư

- Không commit `.env`, Firebase service account, APNs `.p8`, keystore hoặc token relay.
- Chỉ public Caddy và coturn; không public PostgreSQL, Redis hay RabbitMQ.
- Bật rate limit, giữ CORS theo đúng domain, backup có mã hóa và giới hạn SSH.
- Thu hồi session, thiết bị push và tài khoản ngay khi nhân sự rời tổ chức.
- Payload qua push relay có thể chứa tiêu đề/nội dung preview; tắt preview cho
  hội thoại nhạy cảm và công bố bên xử lý dữ liệu trong privacy policy.

Bản phát hành có chức năng tạo tài khoản phải có luồng tự xóa bằng
`DELETE /api/v1/users/me`, màn xác nhận trong app và một URL công khai như
`https://download.example.com/account-deletion` để người không còn cài app vẫn
gửi yêu cầu. Nếu user đang là owner, request nhận email một thành viên active và
chuyển quyền sở hữu trong cùng transaction trước khi xóa, tránh instance bị
chiếm quyền. Sau commit, middleware từ chối JWT cũ và WebSocket của user được
ngắt trên các API node qua Redis (kèm active-user check dự phòng). Chính sách
phải nói rõ dữ liệu xóa ngay, dữ liệu giữ lại theo nghĩa vụ pháp lý và thời hạn
backup hết hiệu lực.

## Phát triển local

Backend:

```sh
cd backend
go test ./...
go run ./cmd/migrate up
go run ./cmd/api
```

Frontend:

```sh
cd frontend
npm ci
npm run typecheck
npm run test:unit
npm run dev:web
```

Xem [hướng dẫn backend local](backend/docs/local-run.md),
[tổng quan kiến trúc](docs/architecture/overview.md) và
[mục lục tài liệu](docs/README.md).

## Giấy phép

Repository hiện chưa có file `LICENSE`. Trước khi public hoặc mời bên ngoài
self-host, chủ dự án phải chọn giấy phép rõ ràng (ví dụ Apache-2.0/MIT nếu muốn
permissive, hoặc AGPL-3.0/commercial nếu muốn ràng buộc bản sửa và phân phối).
Không nên để người dùng tự suy đoán quyền sử dụng từ việc source đang public.

## Cấu trúc repository

```text
backend/               Go API, worker, migration và OpenAPI
frontend/apps/web/      ứng dụng chat web
frontend/apps/admin/    Admin Panel
frontend/packages/      API client, types, UI và platform abstraction
deploy/self-hosted/     compose, Caddy, installer, backup/update/restore
docs/                   kiến trúc, vận hành và roadmap
```

## Chức năng có thể phát triển tiếp

Các hướng tiếp theo phù hợp nhất là tìm kiếm full-text/OpenSearch, retention và
legal hold, SSO/SCIM, SFU cho gọi nhóm lớn, high availability đa node, antivirus/
DLP cho file, federation giữa instance và mobile background sync có attachment.
Danh sách có thứ tự, giá trị và dependency tại
[docs/planning/self-host-feature-roadmap.md](docs/planning/self-host-feature-roadmap.md).
