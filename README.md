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
- Firewall mở TCP `80`, `443`, `3478`; UDP `443`, `3478`, `49160-49200`.
- Không có Nginx/Apache/Caddy khác chiếm cổng `80` hoặc `443` nếu dùng compose mặc định.

## Luồng triển khai cho customer

1. Chọn domain chat, ví dụ `chat.company.com`.
2. Tạo DNS `A chat.company.com -> <IPv4 VPS>`.
3. Cài Docker và Docker Compose v2 trên VPS.
4. Clone source hoặc tải bản release VPSTTT Chat.
5. Chạy installer self-hosted.
6. Mở `https://chat.vpsttt.com/portal`, nhập `chat.company.com`.
7. Portal kiểm tra discovery rồi chuyển tới màn đăng ký trên `chat.company.com`.
8. Tài khoản đầu tiên trở thành workspace owner; sau đó đăng ký mở chuyển sang
   `invite_only`.
9. Owner mời nhân viên hoặc cấu hình SSO/OIDC nếu cần.
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
curl -fsSL https://raw.githubusercontent.com/<org>/<repo>/<tag-or-branch>/deploy/self-hosted/bootstrap-ubuntu.sh \
  | sudo sh -s -- \
    --domain chat.company.com \
    --email admin@company.com \
    --name "Company Chat" \
    --repo-url https://github.com/<org>/<repo>.git
```

Thay `<org>/<repo>/<tag-or-branch>` bằng repository/release thật khi phát hành.
Bootstrap sẽ mở các port cần thiết: `22`, `80`, `443`, `3478/tcp`,
`3478/udp`, `443/udp` và `49160-49200/udp`.

Ví dụ trên VPS:

```sh
git clone <repository-url> vpsttt-chat
cd vpsttt-chat
sh deploy/self-hosted/install.sh \
  --domain chat.company.com \
  --email admin@company.com \
  --name "Company Chat"
```

Installer mặc định cho phép origin `https://chat.vpsttt.com` của portal gọi
discovery từ browser. Người dùng mở portal tại `https://chat.vpsttt.com/portal`.
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

Mở `https://chat.vpsttt.com/portal`, nhập `chat.company.com` và tiếp tục tại màn
đăng ký mà portal điều hướng tới. Backend sẽ cấp owner cho tài khoản đầu tiên
nếu workspace chưa có owner, rồi chuyển registration mode sang `invite_only` để
người lạ không tự đăng ký thêm. Có thể mở thẳng web customer khi cần khôi phục
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
7. xóa token/workspace/cache cũ nếu chuyển sang một instance khác.

Với self-hosted, mobile không gọi endpoint claim/provision domain. Nếu discovery
trả `ZONE_NOT_FOUND`, `TLS` lỗi hoặc không có JSON discovery hợp lệ, nghĩa là
admin chưa cài instance hoặc DNS/TLS chưa sẵn sàng.

## Push notification

Realtime foreground dùng WebSocket trực tiếp tới instance. Khi app ở nền hoặc bị
OS kill, push cần FCM/APNs. Một app official dùng chung được ký với một cấu hình
push cố định; không thể yêu cầu mỗi customer đưa Firebase riêng vào binary đã
phát hành trên store.

MVP phải chọn rõ một trong ba chính sách:

- không dùng push relay: self-host thuần, nhưng notification nền bị giới hạn;
- dùng push relay tối thiểu do WebTUI vận hành cho app official;
- customer tự build/sign app riêng với Firebase/APNs riêng.

Không phân phát service account của Firebase project dùng cho app official tới
các instance customer. Dù chọn chính sách nào, mobile vẫn phải catch-up bằng
sync cursor khi mở lại để không mất sự kiện.

Đây là phần khác biệt lớn so với web: web có thể sống nhờ WebSocket khi tab mở,
mobile cần push + sync cursor để không mất sự kiện sau background.

## Cuộc gọi và media

Cuộc gọi 1:1 dùng WebRTC và coturn trong stack. Mặc định installer tạo:

- STUN: `stun:chat.company.com:3478`
- TURN UDP/TCP: `turn:chat.company.com:3478`
- port media UDP: `49160-49200`

TURN đang dùng credential tĩnh riêng của instance. Nếu credential bị lộ, đổi
`TURN_PASSWORD`, cập nhật `NEXT_PUBLIC_RTC_ICE_SERVERS` và `RTC_ICE_SERVERS`,
sau đó build lại web/admin và restart stack.

Nhóm gọi quy mô lớn hoặc ghi hình cần bổ sung SFU/HPB chuyên dụng; compose mặc
định chỉ nhắm nhóm nhỏ và cuộc gọi 1:1.

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

Backup:

```sh
sh deploy/self-hosted/backup.sh
```

Update:

```sh
sh deploy/self-hosted/update.sh
```

Restore:

```sh
sh deploy/self-hosted/restore.sh /absolute/path/to/backup --yes
```

Backup cần bao gồm PostgreSQL dump, storage volume và file `.env`. Nếu mất
`.env`, secret JWT/webhook/TURN/OIDC cũ không thể khôi phục nguyên trạng.

## Bảo mật tối thiểu

- Không commit hoặc gửi `deploy/self-hosted/.env` cho VPSTTT.
- Chỉ cấp SSH cho người vận hành thật sự cần.
- Bật firewall và chỉ mở các port đã liệt kê.
- Backup ra nơi khác VPS, có mã hóa.
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
