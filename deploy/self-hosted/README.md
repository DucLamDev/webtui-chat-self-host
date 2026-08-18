# VPSTTT Chat self-hosted

Tài liệu này mô tả cách một doanh nghiệp nhỏ tự cài VPSTTT Chat trên VPS/domain
của họ. Mô hình hiện tại giống hướng Nextcloud Talk ở điểm client bắt đầu bằng
server URL và discovery capability, nhưng khác ở điểm VPSTTT Chat không có
control plane tự dựng VPS cho khách hàng.

Điểm cần nói rõ với customer: nhập `chat.example.com` trong web/mobile chỉ để
kết nối tới instance đã được cài sẵn. Việc tự host vẫn cần người vận hành chuẩn
bị VPS, DNS, Docker, firewall, backup và cập nhật.

## Mô hình triển khai

Mỗi customer sở hữu một instance độc lập:

- một domain cố định, ví dụ `chat.company.com`;
- một PostgreSQL, Redis, RabbitMQ, storage local và coturn riêng;
- một bộ secret riêng trong `deploy/self-hosted/.env`;
- web, admin, API và WebSocket cùng origin qua Caddy;
- không lưu dữ liệu chat trên hạ tầng trung tâm VPSTTT.

Trong backend vẫn có khái niệm `zone`, nhưng ở self-hosted nó là ranh giới nội
bộ của chính instance đó, không phải tenant SaaS dùng chung. Khi API khởi động
với `DEPLOYMENT_MODE=self_hosted`, backend tự cấu hình zone/workspace đầu tiên
theo `INSTANCE_DOMAIN` và `INSTANCE_NAME`.

## Yêu cầu hạ tầng

- Linux VPS có IPv4 public, khuyến nghị Ubuntu 22.04/24.04 LTS.
- Tối thiểu 4 vCPU, 8 GB RAM, 40 GB SSD cho nhóm nhỏ; tăng disk theo file/media.
- Docker Engine và Docker Compose v2.
- Domain/subdomain riêng, ví dụ `chat.company.com`.
- DNS `A` của domain trỏ vào IPv4 public của VPS trước khi cài.
- Firewall mở TCP `80`, `443`, `3478`, `8443`, `8444`; UDP `443`, `3478`, `10000`, `49160-49200`.
- Không có Nginx/Apache/Caddy khác chiếm cổng `80` hoặc `443` nếu dùng compose mặc định.

## Luồng triển khai cho customer

1. Chọn domain chat, ví dụ `chat.company.com`.
2. Tạo DNS `A chat.company.com -> <IPv4 VPS>`.
3. Cài Docker và Docker Compose v2 trên VPS.
4. Clone source hoặc tải bản release VPSTTT Chat.
5. Chạy installer self-hosted.
6. Mở `https://download.webtui.vn`, nhập `chat.company.com`.
7. Portal kiểm tra discovery rồi chuyển tới màn đăng ký trên `chat.company.com`.
8. Tài khoản đầu tiên trở thành workspace owner. Trước khi có owner, đăng ký
   luôn ở chế độ `open` để tránh khóa nhầm server mới.
9. Owner có thể đổi sang `invite_only`/`closed`, mời nhân viên hoặc cấu hình SSO/OIDC.
10. Người dùng cài mobile/desktop, nhập cùng domain `chat.company.com` để đăng nhập.

## Cài mới

### Cách nhanh nhất cho VPS Ubuntu mới

Nếu VPS là Ubuntu 22.04/24.04 mới, có thể dùng bootstrap để tự cài gói nền,
Docker, Docker Compose v2, UFW firewall rồi chạy installer self-hosted:

```sh
sudo sh deploy/self-hosted/bootstrap-ubuntu.sh \
  --domain chat.company.com \
  --email admin@company.com \
  --name "Company Chat"
```

Nếu customer chạy trực tiếp từ một máy trống và muốn script tự clone source:

```sh
curl -fsSL https://raw.githubusercontent.com/DucLamDev/webtui-chat-self-host/master/deploy/self-hosted/bootstrap-ubuntu.sh \
  | sudo sh -s -- \
    --domain chat.company.com \
    --email admin@company.com \
    --name "Company Chat" \
    --repo-url https://github.com/DucLamDev/webtui-chat-self-host.git
```

Khi có release ổn định, nên thay `master` trong URL raw bằng tag release để
bootstrap luôn dùng đúng phiên bản đã kiểm thử.
Bootstrap sẽ mở các port cần thiết: `22`, `80`, `443`, `3478/tcp`,
`8443/tcp`, `8444/tcp`, `3478/udp`, `443/udp`, `10000/udp` và `49160-49200/udp`.

Ví dụ trên VPS:

```sh
git clone https://github.com/DucLamDev/webtui-chat-self-host.git vpsttt-chat
cd vpsttt-chat
sh deploy/self-hosted/install.sh \
  --domain chat.company.com \
  --email admin@company.com \
  --name "Company Chat"
```

Installer mặc định cho phép origin `https://download.webtui.vn` của portal gọi
discovery từ browser. Người dùng mở portal tại `https://download.webtui.vn`.
`WEBTUI_APP_LINK_HOST=chat.vpsttt.com` là host tĩnh do publisher kiểm soát và đã
khai báo trong official binary. Domain customer như `chat.company.com` được nhập
thủ công trong app; customer **không** cần và không được proxy `assetlinks.json`
hoặc AASA của official app trên domain của mình. Matcher Caddy chỉ phục vụ hai
association file khi chính deployment đó sở hữu publisher host. Một custom-
branded binary phải dùng manifest, signing identity, portal và host association
riêng, đồng bộ với nhau.

Bản phát hành đầu chỉ có CH-Play nên installer đặt
`ENABLE_IOS_ASSOCIATION=false`: Android `assetlinks.json` phải trả 200, còn AASA
phải trả 404/410 fail-closed. Chỉ đổi cờ thành `true` sau khi publisher đã có app
iOS ký thật, Apple Team/Bundle ID và portal AASA khớp; lúc đó `check.sh` mới yêu
cầu AASA trả 200.

Installer cũng ghim `TERMS_VERSION=2026-08-07` và
`PRIVACY_POLICY_VERSION=2026-08-07`, đúng với bộ policy đang công bố trên
publisher portal. API production sẽ từ chối khởi động nếu thiếu hoặc dùng giá
trị placeholder. Không đổi hai version độc lập với portal; nếu policy được cập
nhật, phát hành nội dung portal và cấu hình backend cùng một release.
Nếu vận hành portal riêng, truyền origin HTTPS không có path:

```sh
sh deploy/self-hosted/install.sh \
  --domain chat.company.com \
  --email admin@company.com \
  --name "Company Chat" \
  --portal-origin https://portal.example.com
```

Installer sẽ:

- kiểm tra domain hợp lệ và DNS `A` trỏ về IPv4 của VPS;
- tự phát sinh secret cho PostgreSQL, Redis, RabbitMQ, JWT, webhook, OIDC và TURN;
- ghi secret vào `deploy/self-hosted/.env` với quyền file `0600`;
- build image API, worker, web và admin;
- không build hay mount portal; stack self-host chỉ chứa dịch vụ thuộc instance;
- chạy migration database;
- bật PostgreSQL, Redis, RabbitMQ, API, worker, web, admin, Caddy và coturn;
- chờ `https://chat.company.com/ready` sẵn sàng.

Nếu VPS có nhiều IPv4 hoặc lệnh tự dò IP không đúng, truyền IP public thủ công:

```sh
sh deploy/self-hosted/install.sh \
  --domain chat.company.com \
  --email admin@company.com \
  --name "Company Chat" \
  --external-ip 203.0.113.10
```

Nếu DNS vừa đổi và chưa propagate nhưng bạn đã chắc record đúng, có thể bỏ qua
preflight DNS:

```sh
sh deploy/self-hosted/install.sh \
  --domain chat.company.com \
  --email admin@company.com \
  --name "Company Chat" \
  --skip-dns-check
```

Nếu gặp lỗi dạng:

```text
DNS for chat.company.com must contain A record <IPv4 VPS> before installation.
Resolved IPv4 addresses: none
```

hãy kiểm tra lại trên VPS:

```sh
curl -4 https://api.ipify.org
getent ahostsv4 chat.company.com
```

Nếu IP public của VPS trùng với bản ghi `A` trong DNS panel nhưng resolver của VPS
chưa cập nhật, chạy lại installer với `--skip-dns-check`. Nếu domain vẫn chưa trỏ
đúng ra public DNS, Caddy/Let's Encrypt sẽ chưa thể cấp TLS và bước chờ
`https://chat.company.com/ready` sẽ thất bại.

Chỉ dùng `--force` cho database/volume mới hoặc môi trường test. Không dùng
`--force` để sửa instance đang có dữ liệu.

## Sau khi cài xong

Kiểm tra các URL chính:

- Web app: `https://chat.company.com`
- Admin Panel: `https://chat.company.com/admin`
- API info: `https://chat.company.com/api`
- Health check: `https://chat.company.com/ready`
- Discovery: `https://chat.company.com/api/v1/discovery?domain=chat.company.com`
- WebSocket: `wss://chat.company.com/ws`

Mở `https://download.webtui.vn`, nhập `chat.company.com` và tiếp tục tại màn
đăng ký mà portal điều hướng tới. Backend sẽ cấp owner cho tài khoản đầu tiên
nếu workspace chưa có owner. Sau đó, owner có thể đổi chính sách trong mục
**Cài đặt → Thương hiệu & truy cập**. Có thể mở thẳng web customer khi cần khôi phục
hoặc vận hành nội bộ; password và token trong cả hai trường hợp đều chỉ gửi tới
instance customer.

## Mobile và desktop

Người dùng cuối không cần biết API URL. Họ chỉ cần nhập domain:

```text
chat.company.com
```

Mobile thực hiện:

1. chuẩn hóa thành `https://chat.company.com`;
2. gọi `GET /api/v1/discovery?domain=chat.company.com` trên chính domain đó;
3. đọc `runtime.api_base_url`, `runtime.ws_base_url` và `capabilities`;
4. lưu base URL của instance vào secure storage;
5. gửi login/register tới `/api/v1/auth/*` trên instance đó;
6. lưu refresh token trong secure storage và cache local theo workspace;
7. cô lập token/workspace/cache theo `instance_id + origin`; khi chuyển server,
   app cất phiên cũ trong secure storage và chỉ khôi phục đúng phiên của instance
   đã discovery lại thành công. Dữ liệu server A không bao giờ được gửi sang B.

Với self-hosted, mobile không gọi endpoint claim/provision domain. Nếu discovery
trả `ZONE_NOT_FOUND`, `TLS` lỗi hoặc không có JSON discovery hợp lệ, nghĩa là
admin chưa cài instance hoặc DNS/TLS chưa sẵn sàng.

### Discovery compatibility contract

Hai URL sau phải mô tả cùng một instance; URL well-known trả DTO trực tiếp,
còn API v1 bọc DTO tại `data.discovery`:

```sh
curl -fsS "https://chat.company.com/.well-known/vpsttt-chat"
curl -fsS "https://chat.company.com/api/v1/discovery?domain=chat.company.com"
```

Contract mobile production yêu cầu `instance_id` là UUID và bằng `zone.id`,
`runtime.api_contract_version=1`, `runtime.minimum_supported_mobile_version` là
SemVer, cùng các capability `moderation`, `reporting`, `blocking`,
`account_deletion`, `legal_acceptance`. `runtime.server_version` và legacy
`runtime.app_version` chỉ là release identifier; giá trị `self-hosted` của bản
cài cũ vẫn hợp lệ.

`instance_id` lấy từ zone UUID đã lưu trong PostgreSQL, không lấy từ domain hay
`.env`: update và đổi domain không làm ID thay đổi. Restore phải phục hồi cả
database để giữ identity. Cài database mới tạo identity mới; mobile phải coi đó
là instance khác và không dùng lại token/cache cũ. Installer đặt
`MOBILE_MIN_VERSION=1.0.0`; chỉ tăng giá trị này khi chủ động ngừng hỗ trợ mobile
cũ, vì client thấp hơn sẽ từ chối kết nối.

## Push notification

Đường direct FCM/APNs cho custom-branded binary vẫn được hỗ trợ. Ngay trước khi
gọi provider, worker lấy `workspace.zone_id` từ PostgreSQL, clone payload và ghi
đè `instance_id`; giá trị thiếu hoặc giả mạo trong notification job không được
tin cậy. Vì vậy direct và publisher relay đều phát cùng identity với discovery.

Realtime foreground dùng WebSocket trực tiếp tới instance. Khi app ở nền hoặc bị
OS kill, push cần FCM/APNs. Một app official dùng chung được ký với một cấu hình
push cố định; không thể yêu cầu mỗi customer đưa Firebase riêng vào binary đã
phát hành trên store.

Production phải chọn rõ một trong ba chính sách:

- không dùng push relay: self-host thuần, nhưng notification nền bị giới hạn;
- dùng push relay do publisher vận hành cho app official;
- customer tự build/sign app riêng với Firebase/APNs riêng.

Không phân phát service account của Firebase project dùng cho app official tới
các instance customer. Dù chọn chính sách nào, mobile vẫn phải catch-up bằng
sync cursor khi mở lại để không mất sự kiện.

Relay client mặc định tắt. Chỉ điền đủ `PUSH_RELAY_URL`, `PUSH_RELAY_TOKEN` và
`PUSH_RELAY_INSTANCE_ID` khi publisher đã cấp token riêng cho instance. ID này
phải bằng UUID lowercase tại `data.discovery.instance_id` (cũng bằng
`data.discovery.zone.id`); key tương ứng trong `PUSH_RELAY_PUBLISHERS` trên
publisher phải giống hệt, không dùng slug/domain. Worker sẽ từ chối khởi động
nếu ID cấu hình khác zone UUID đã lưu trong database.
Khi dùng bundled Caddy, URL là
`https://relay.publisher.example/push-relay/v1/deliveries`.
Repository cũng có relay server tự host với queue PostgreSQL, auth publisher,
idempotency, rate limit và retry; service này chỉ chạy khi bật compose profile
`push-relay` và `PUSH_RELAY_SERVER_ENABLED=true`. Relay và migrator không load
toàn bộ `.env`; Compose chỉ cấp allowlist database/provider/relay cần cho từng
role, nên JWT, webhook, bot/OIDC, TURN và storage secret không nằm trong relay.

Web Push/VAPID theo từng instance cũng mặc định tắt. Sau khi tạo VAPID key và bật
`WEB_PUSH_ENABLED`, browser chỉ xin quyền từ thao tác opt-in của người dùng;
service worker có thể nhận notification khi tab đóng. Mobile/web vẫn cần sync
cursor để không mất sự kiện. Xem hướng dẫn cấu hình, xoay key, hợp đồng relay và
kiểm thử thiết bị thật tại
[`docs/operations/push-notifications.md`](../../docs/operations/push-notifications.md).

## Bot AI theo nghiệp vụ của tổ chức

Owner tạo bot tại `Kênh & Bot`, cài bot vào kênh, chọn provider và tạo flow.
Flow chỉ chạy sau khi được xuất bản. Trigger hỗ trợ:

- `{"type":"mention"}`: chạy khi tin nhắn nhắc `@slug-bot`;
- `{"type":"keyword","keywords":["nghỉ phép","chấm công"]}`;
- `{"type":"command","prefix":"/hr"}`;
- `{"type":"all"}`: nhận mọi tin nhắn trong kênh đã cài bot.

Ollama và LocalAI trong mạng Docker dùng được ngay khi endpoint nằm trong
`BOT_AI_ALLOWED_HOSTS`. Với endpoint tương thích OpenAI hoặc webhook bên ngoài,
thêm đúng hostname vào biến này. API key không lưu trực tiếp trong database:

```dotenv
BOT_AI_ALLOWED_HOSTS=ollama,local-ai,ai.company.com
BOT_AI_OPENAI_KEY=replace-with-a-secret
```

Sau đó nhập `env://BOT_AI_OPENAI_KEY` vào `Secret reference`. Runtime chỉ cho
đọc biến môi trường bắt đầu bằng `BOT_AI_`, chặn URL có credentials và chặn
hostname công khai chưa nằm trong allowlist.

## Cuộc gọi và media

Cuộc gọi 1:1 dùng WebRTC và coturn trong stack. Mặc định installer tạo:

- STUN: `stun:chat.company.com:3478`
- TURN UDP/TCP: `turn:chat.company.com:3478`
- port media UDP: `49160-49200`

TURN dùng cơ chế coturn REST với username/HMAC ngắn hạn được cấp qua endpoint
`GET /api/v1/calls/ice-servers` có xác thực. Secret `TURN_SHARED_SECRET` chỉ nằm
ở API và coturn, không xuất hiện trong discovery, HTML hay bundle client. Khi xoay
secret, cập nhật cùng lúc API/coturn và restart hai service; client tự lấy
credential mới, không cần build lại ứng dụng.

Phòng nhóm, guest link, webinar và breakout room dùng Jitsi self-host làm SFU.
Stack mặc định đã gồm Jitsi Web, Prosody, Jicofo và Jitsi Videobridge. Installer
tự tạo mật khẩu nội bộ và phục vụ cuộc họp tại cùng domain qua cổng
`https://chat.company.com:8443`; không cần thêm DNS hay tự điền URL. Firewall VPS
cần cho phép TCP `8443` và UDP `10000` để media nhiều người đi qua Videobridge.
Sửa Word/Excel bằng ONLYOFFICE cần thêm TCP `8444`.

Nếu tổ chức chủ động dùng một cụm Jitsi khác, có thể ghi đè trong `.env`:

```dotenv
JITSI_BASE_URL=https://meet.chat.company.com
NEXT_PUBLIC_JITSI_BASE_URL=https://meet.chat.company.com
```

Sau đó build lại `api` và `web`. Backend trả media base URL cho web/mobile;
client dùng room key ngẫu nhiên, grid view, giơ tay, screen share, blur/đổi nền
và toolbar theo vai trò. Public link chỉ trả room key sau khi qua password và
lobby; xoay/thu hồi link cũng xoay room key.

Room key ngẫu nhiên của ứng dụng không được trả cho khách trước khi qua mật khẩu
và phòng chờ. Ghi hình/livestream vẫn cần thêm Jibri và chính sách
consent/retention riêng.

## Sửa Word/Excel nội bộ bằng ONLYOFFICE Community

Stack có sẵn `onlyoffice/documentserver` để mở và sửa trực tiếp file Word/Excel
trong chat. Mặc định tính năng tắt để tránh khởi động với secret mẫu; bật trong
`.env` như sau:

```dotenv
ONLYOFFICE_ENABLED=true
ONLYOFFICE_PUBLIC_URL=https://chat.company.com:8444
ONLYOFFICE_INTERNAL_URL=http://onlyoffice-document-server
ONLYOFFICE_API_INTERNAL_URL=https://chat.company.com
ONLYOFFICE_JWT_SECRET=replace-with-at-least-32-random-characters
ONLYOFFICE_SESSION_SECRET=replace-with-at-least-32-random-characters
```

Sau đó mở firewall TCP `8444` và restart stack:

```sh
cd deploy/self-hosted
docker compose --env-file .env -f compose.yml up -d onlyoffice-document-server api web caddy
```

Web client sẽ ưu tiên ONLYOFFICE cho `.doc`, `.docx`, `.xls`, `.xlsx`; nếu tính
năng chưa bật thì quay về editor text/CSV/MD hiện có. API ký URL tải file và
callback bằng token ngắn hạn, còn Document Server xác thực cấu hình editor bằng
JWT chung `ONLYOFFICE_JWT_SECRET`.

## Vận hành hằng ngày

Kiểm tra trạng thái:

```sh
sh deploy/self-hosted/check.sh
```

Xem log:

```sh
cd deploy/self-hosted
docker compose --env-file .env -f compose.yml logs -f api worker caddy
```

Bật stack quan sát tùy chọn (Grafana mặc định chỉ nghe `127.0.0.1:3300`):

```sh
docker compose --env-file .env -f compose.yml --profile observability up -d
ssh -L 3300:127.0.0.1:3300 operator@your-vps
```

Dashboard Grafana có HTTP p95/p99, queue/dead-letter push và trạng thái backup;
Admin Panel → **Push** cho phép operator xem chi tiết theo workspace và replay job
dead khi có `notification.manage`. Hãy nối Alertmanager tới webhook/email nội bộ
theo [runbook observability](../../docs/operations/observability.md); cấu hình mặc
định không gửi telemetry ra khỏi VPS.

Backup off-site đã mã hóa (mặc định tắt; cấu hình S3/MinIO trước):

```sh
cd deploy/self-hosted
cp offsite-backup.env.example offsite-backup.env
chmod 600 offsite-backup.env
# Điền bucket/credential/password rồi mới chạy:
./backup.sh plan
./backup.sh init
./backup.sh backup --maintenance
```

Update:

```sh
sh deploy/self-hosted/update.sh
```

Lệnh update chỉ tự tạo backup trước khi cập nhật nếu
`OFFSITE_BACKUP_ENABLED=true` trong `offsite-backup.env`. Cài đặt mặc định chưa
bật backup off-site vẫn cập nhật bình thường.

Khi nâng cấp một bản cài cũ, `update.sh` đổi `WEBTUI_APP_LINK_HOST` đang trống
hoặc còn bằng customer `INSTANCE_DOMAIN` sang `chat.vpsttt.com`, vì official
universal app chỉ xác minh host do publisher kiểm soát. Giá trị custom khác
instance domain được giữ nguyên. Nếu đang vận hành binary custom-branded đã ký
riêng và cần giữ association host (kể cả bằng instance domain), đặt
`PRESERVE_CUSTOM_APP_LINK_HOST=true` trong `.env` trước khi chạy update.

Tài khoản Admin Panel không dùng mật khẩu mặc định. Tài khoản chủ sở hữu đầu
tiên có thể đăng nhập Admin Panel bằng cùng username/mật khẩu của trang chat.
Operator có thể xem, tạo hoặc cấp lại mật khẩu quản trị bằng các lệnh sau:

```sh
sh deploy/self-hosted/admin-account.sh list
sh deploy/self-hosted/admin-account.sh create admin
sh deploy/self-hosted/admin-account.sh reset admin
```

Mật khẩu ngẫu nhiên chỉ được in một lần sau lệnh `create` hoặc `reset`.

Restore từ một snapshot ID cụ thể:

```sh
cd deploy/self-hosted
./restore.sh 0123abcd --apply --confirm RESTORE:0123abcd
```

Backup mới bao gồm PostgreSQL custom dump, file/object storage, manifest và
SHA-256 từng file trong một repository Restic mã hóa phía client. Scheduler chỉ
chạy khi bật Compose profile `backup`; quickstart không tự upload dữ liệu. File
`.env` mặc định không được đưa vào snapshot và không bao giờ tự ghi đè khi
restore. Credential off-site nằm riêng trong `offsite-backup.env` (tạo từ
`offsite-backup.env.example`), nên API/worker không nhận access key hoặc password
Restic. Container DR cũng không nạp toàn bộ `.env` hay mount cả thư mục deploy;
chỉ các biến app cần thiết và hai file `compose.yml`/`Caddyfile` được cấp rõ
ràng. `.env` chỉ được mount khi operator opt-in cho từng lệnh theo runbook. Lưu
secret cùng password Restic trong password manager/escrow độc lập.

Xem [runbook backup/restore đầy đủ](../../docs/operations/offsite-backup-restore.md)
để cấu hình retention, verify, safety snapshot, restore drill và giới hạn PITR.
Restore tự stage và kiểm checksum trước maintenance, tạo safety snapshot rồi tự
rollback khi migration/health check thất bại; thao tác phá hủy vẫn đòi snapshot
ID và chuỗi xác nhận chính xác, không tự chọn `latest`.

Web client có offline outbox cho tin nhắn text và delta-sync bền vững. File/voice
chưa được tự xếp hàng khi offline; xem [phạm vi và conflict policy](../../docs/operations/offline-outbox-and-sync.md).

## Bảo mật tối thiểu

- Không commit hoặc gửi `deploy/self-hosted/.env` cho VPSTTT.
- Chỉ cấp SSH cho người vận hành thật sự cần.
- Bật firewall và chỉ mở các port đã liệt kê.
- Backup ra bucket/account khác VPS, có mã hóa; verify và restore drill định kỳ.
- Theo dõi dung lượng disk vì file/media nằm trong storage của instance.
- Khi nhân sự rời công ty, revoke session và khóa tài khoản trong Admin Panel.
- Nếu public lên store mobile, cần chính sách privacy/account deletion riêng
  cho tổ chức hoặc cho sản phẩm phát hành chung.

## Khi nào không dùng self-hosted mặc định

- Khách muốn bấm domain là tự tạo server hoàn toàn mà không đụng VPS: cần xây
  control plane SaaS/provisioner riêng, không phải scope compose này.
- Khách cần HA, nhiều node, database managed hoặc Kubernetes: dùng blueprint
  dedicated infra riêng.
- Khách cần federation giữa nhiều công ty: capability hiện đang fail-closed.
