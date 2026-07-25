# Kiến trúc Portal - Self-hosted Instance - Client

Ngày cập nhật: 2026-07-24

## Quyết định kiến trúc

WebTUI Chat dùng ba lớp tách biệt:

| Lớp | Domain ví dụ | Trách nhiệm |
|---|---|---|
| Portal trung tâm | `https://chat.vpsttt.com/portal` | Nhập domain, kiểm tra discovery, download center, tài liệu |
| Instance customer | `https://chat.company.com` | Web chat, API, PostgreSQL, Redis, RabbitMQ, storage, WebSocket, TURN, token |
| Client dùng chung | Desktop/Mobile WebTUI Chat | Nhập domain, discovery, lưu runtime và đăng nhập trực tiếp vào instance |

Portal không provision VPS chỉ bằng một domain. Customer phải cài instance trước,
vì domain không cung cấp quyền SSH, cloud API hay quyền tạo tài nguyên trên VPS.
Sau khi cài, portal xác nhận instance và đưa người dùng tới đúng trang đăng ký hoặc
đăng nhập.

## Contract URL của một instance

Với domain `chat.company.com`:

| Thành phần | URL |
|---|---|
| Web chat | `https://chat.company.com` |
| Thông tin API | `https://chat.company.com/api` |
| API nghiệp vụ v1 | `https://chat.company.com/api/v1/...` |
| Discovery | `https://chat.company.com/api/v1/discovery?domain=chat.company.com` |
| Well-known discovery | `https://chat.company.com/.well-known/vpsttt-chat` |
| WebSocket | `wss://chat.company.com/ws` |
| Admin | `https://chat.company.com/admin` |

`runtime.api_base_url` là origin `https://chat.company.com`. Client nối các route
`/api/v1/...` vào origin này. Không đặt `api_base_url` thành
`https://chat.company.com/api`, vì các client hiện dùng route tuyệt đối và sẽ tạo
URL sai nếu base URL chứa path.

## Luồng cài và kích hoạt lần đầu

```text
Admin customer
  |
  | 1. Chuẩn bị VPS + DNS + firewall
  | 2. Chạy install.sh với domain và tên công ty
  v
Instance customer đã sẵn sàng
  |
  | GET /api/v1/discovery
  v
Portal kiểm tra self_hosted + active + ready + TLS
  |
  | registration_mode=open
  | redirect https://chat.company.com/?auth=register&source=portal
  v
Web chat trên domain customer
  |
  | POST /api/v1/auth/register (không cần domain trong body)
  v
Tài khoản đầu tiên = workspace_owner
  |
  | backend chuyển registration_mode=invite_only
  v
Instance bắt đầu hoạt động
```

Mật khẩu và token chỉ đi giữa browser/client với `chat.company.com`. Portal không
proxy form đăng ký và không nhận JWT.

## Luồng web

Web app được build và chạy trong compose của customer:

1. Customer mở portal và nhập `chat.company.com`.
2. Portal gọi discovery trực tiếp từ browser.
3. Nếu instance chưa có owner, portal chuyển tới `?auth=register`.
4. Nếu instance đã kích hoạt, portal chuyển tới `?auth=login`.
5. Trên browser, web app khóa server theo hostname hiện tại và không cho sửa
   domain trong form auth.
6. API request đi cùng origin tới `/api/v1`.

Tên hiển thị lấy từ `INSTANCE_NAME`/`zone.name`, nên mỗi customer có thương hiệu
công ty trên web mà không cần fork source.

## Luồng desktop

Desktop là một binary Tauri dùng chung:

1. Người dùng nhập domain công ty.
2. App chuẩn hóa domain và gọi discovery.
3. App lưu `domain`, `api_base_url`, `ws_base_url` và capability.
4. UI Tauri vẫn chạy local; app không điều hướng WebView sang website customer.
5. Login/register gọi trực tiếp API instance.
6. Token được lưu trong secure storage của desktop.

Desktop download luôn đến từ download center trung tâm. Customer không cần build
desktop app riêng.

## Luồng mobile

Mobile là một app Flutter dùng chung trên store/APK:

1. Người dùng nhập `chat.company.com`.
2. App gọi discovery và xác minh:
   - domain response khớp domain đã nhập;
   - `zone.status=active`;
   - `deployment.status=ready`;
   - `capabilities.self_hosted=true`;
   - API/WebSocket dùng HTTPS/WSS và cùng hostname.
3. App lưu API origin và WebSocket URL trong secure storage.
4. Login/register gửi request tới instance đã chọn; domain không còn là trường
   đăng ký tenant trong request body.
5. Nếu `registration_mode=invite_only|closed`, app không cho đăng ký mở.
6. Realtime dùng đúng `runtime.ws_base_url`, không dùng host build-time.

Một customer không có một mobile app riêng. MVP chỉ giữ một server active; khi
đổi customer thành công, app xóa token, workspace và cache phiên cũ trước khi
kích hoạt runtime mới. Multi-account/multi-server là giai đoạn sau.

## Cài instance customer

```sh
sh deploy/self-hosted/install.sh \
  --domain chat.company.com \
  --email admin@company.com \
  --name "Company Chat"
```

Nếu portal chạy ở domain khác:

```sh
sh deploy/self-hosted/install.sh \
  --domain chat.company.com \
  --email admin@company.com \
  --name "Company Chat" \
  --portal-origin https://portal.example.com
```

Installer thêm portal origin vào CORS để browser portal được phép gọi discovery.
Với portal chính tại `https://chat.vpsttt.com/portal`, origin cần cấu hình là
`https://chat.vpsttt.com` vì CORS origin không bao gồm path.
Không dùng `CORS_ALLOWED_ORIGINS=*` trên production.

## Triển khai portal

Portal là service stateless riêng:

```sh
cd portal/deploy
cp .env.example .env
docker compose --env-file .env -f compose.yml up -d --build
```

Portal hiện thực hiện domain onboarding, download center và documentation. Các
module quản lý công ty, license, heartbeat và version inventory trong bảng chức
năng là control plane giai đoạn tiếp theo. Chúng phải dùng database trung tâm
riêng và chỉ lưu metadata vận hành, không dùng database chat của customer.

## Phạm vi triển khai hiện tại

| Nhóm trong bảng chức năng | Trạng thái MVP |
|---|---|
| Portal nhập/kiểm tra domain, download, documentation | Đã triển khai |
| Installer, Compose, TLS/Caddy, API, auth, WebSocket và storage self-host | Đã có nền tảng trong repo, installer đã nối portal origin |
| Web auth theo hostname instance | Đã refactor |
| Desktop server selector/discovery/runtime | Đã refactor để giữ UI local và dùng API/WS customer |
| Mobile server selector/discovery/API/WS/secure storage | Đã refactor cho một server active |
| Company registry, license, heartbeat, version inventory | Chưa triển khai; control plane giai đoạn kế tiếp |
| Push nền cho một official mobile app | Chưa chốt chính sách relay/build riêng |
| Marketplace, backup từ portal, monitoring tập trung | Chưa triển khai |

## Push notification mobile

WebSocket đủ khi app đang foreground. Khi app bị background/kill, một official
app dùng chung cần một trong ba chính sách:

1. Không push relay: self-host thuần, nhưng notification nền bị giới hạn.
2. WebTUI push relay tối thiểu: UX tốt hơn, cần công bố metadata/privacy rõ ràng.
3. Customer tự build app với Firebase/APNs riêng: self-host tuyệt đối nhưng khó
   vận hành cho doanh nghiệp nhỏ.

Đây là quyết định sản phẩm độc lập với domain discovery; không phát service
account Firebase của official app cho từng customer.

## Tiêu chí nghiệm thu

- Domain chưa cài WebTUI Chat không vượt qua portal/mobile discovery.
- Domain hợp lệ chuyển đúng tới register khi `open`, login khi `invite_only`.
- Portal không nhận password, access token hoặc refresh token.
- Self-host auth bỏ qua domain/Host giả và dùng `INSTANCE_DOMAIN` đã cấu hình.
- Web browser không cho đổi instance khỏi hostname hiện tại.
- Desktop giữ UI local sau discovery.
- Mobile lưu và dùng cả API lẫn WebSocket của instance.
- API, database, file và realtime của customer không đi qua portal trung tâm.
