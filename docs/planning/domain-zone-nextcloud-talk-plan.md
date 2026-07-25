# VPSTTT Chat domain-first zone: kết quả Phase 0-8

> Lưu ý trạng thái: tài liệu này là lịch sử thiết kế cho hướng shared SaaS /
> control-plane domain claim. Định hướng sản phẩm hiện tại là self-hosted
> open-source cho từng customer, xem `deploy/self-hosted/README.md` và
> `docs/planning/mobile-self-hosted-flow.md`. Trong self-hosted, người dùng
> nhập domain để discovery instance đã được cài trước; mobile không gọi luồng
> `/api/v1/zones/claims` để tự tạo tenant.

Tài liệu này mô tả kiến trúc và trạng thái triển khai VPSTTT Chat theo hướng
server/domain-first tương tự Nextcloud Talk. Người dùng bắt đầu bằng domain,
ứng dụng discovery runtime của domain đó, rồi đăng nhập vào một zone tách biệt.

## Mục tiêu và phạm vi

- Zone nội bộ VPSTTT giữ riêng nghiệp vụ VPS, proxy, hosting, domain, gia hạn,
  ticket và server alert.
- Khách hàng có domain riêng, ví dụ `chat.abc.com`, web/API/WebSocket cùng chạy
  qua domain đó.
- Dữ liệu, token, session, realtime, audit, bot và automation được scope theo
  zone của domain.
- Customer zone không được gọi API order/billing hard-code của VPSTTT.
- Phase 0-8 dùng shared SaaS với cô lập logic. Schema vẫn mở cho
  `dedicated_compose`, `dedicated_k8s` và database riêng ở phase hạ tầng sau.

## Tham chiếu Nextcloud

Thiết kế lấy các nguyên tắc, không sao chép nguyên trạng API của Nextcloud:

- client bắt đầu bằng server URL và discovery/capabilities;
- login gắn với server đã chọn;
- tính năng được công bố bằng capability thay vì client tự giả định;
- bot/webhook có secret, chữ ký và phạm vi cài đặt riêng;
- WebSocket/signaling sử dụng runtime URL do server trả về.

Nguồn chính:

- https://nextcloud-talk.readthedocs.io/en/stable/capabilities/
- https://nextcloud-talk.readthedocs.io/en/stable/bots/
- https://nextcloud-talk.readthedocs.io/en/stable/conversation/
- https://nextcloud-talk.readthedocs.io/en/stable/chat/
- https://docs.nextcloud.com/server/latest/developer_manual/client_apis/LoginFlow/index.html
- https://docs.nextcloud.com/server/latest/admin_manual/configuration_user/user_auth_oidc.html
- https://openid.net/specs/openid-connect-core-1_0.html
- https://www.rfc-editor.org/info/rfc7636/

## Mô hình dữ liệu

`zone` là ranh giới cô lập cao hơn `workspace`.

- `vpsttt_internal`: zone nội bộ có quyền dùng nghiệp vụ VPSTTT.
- `customer_saas`: tenant dùng shared stack và tách bằng `zone_id`.
- `customer_dedicated`: tenant dành cho stack/database riêng trong tương lai.

Các bảng điều khiển:

- `zones`: loại zone, trạng thái, registration mode và workspace chính.
- `zone_domains`: domain, DNS challenge, trạng thái verify và TLS.
- `zone_deployments`: runtime URL, deployment/database mode, storage và Redis.
- `automation_templates`: template theo `zone_kind`.
- `automation_installations`: config và `secret_ref` theo zone/workspace.
- `zone_deployment_requests`: yêu cầu chuyển runtime/database mode có idempotency.
- `zone_quotas`: giới hạn và chế độ enforcement theo zone.
- `zone_oidc_providers`: hợp đồng cấu hình OIDC, chỉ lưu `client_secret_ref`.
- `zone_oidc_login_states`: state hash, PKCE verifier mã hóa, nonce và TTL dùng một lần.
- `zone_oidc_login_results`: completion code hash và claims đã verify, TTL dùng một lần.
- `zone_oidc_identities`: liên kết `(provider, subject)` ổn định với user trong zone.

`users` hiện là identity cấp platform và có thể tham gia nhiều zone khi được mời hoặc được
policy cho phép. Quyền nhìn thấy dữ liệu luôn đi qua membership, session và resource thuộc
zone. Vì vậy Phase 0-8 cung cấp logical tenant isolation; user directory hoặc database vật lý
riêng cho từng khách hàng vẫn thuộc phase dedicated infrastructure.

## Luồng tạo zone mới

1. Người dùng nhập `chat.abc.com` ở màn đăng nhập/đăng ký.
2. Client thử discovery domain qua `/.well-known/vpsttt-chat` hoặc
   `GET /api/v1/discovery?domain=chat.abc.com`.
3. Nếu domain chưa tồn tại, client đăng ký identity ở control plane.
4. Client gọi `POST /api/v1/zones/claims`.
5. Backend trả bản ghi định tuyến A/AAAA tới edge VPSTTT và TXT name
   `_vpsttt-chat.chat.abc.com` có value bắt đầu bằng `vpsttt-chat-verification=`.
6. Chủ domain tạo cả bản ghi định tuyến và TXT, sau đó gọi endpoint verify.
7. Backend kiểm tra TXT, sau đó trong một transaction:
   - kích hoạt zone và domain;
   - tạo workspace mặc định, owner membership và role;
   - tạo các kênh trung lập `general` và `announcements`;
   - tạo shared deployment với URL theo domain;
   - gán storage bucket và Redis prefix riêng;
   - ghi audit log.
8. Caddy chỉ cấp chứng chỉ khi internal ask endpoint xác nhận domain đang active.
9. Client xóa session control-plane, chuyển runtime sang domain mới và yêu cầu
   người dùng đăng nhập vào zone vừa tạo.

## Trạng thái Phase 0-8

### Phase 0: quyết định kiến trúc

Hoàn tất:

- chốt zone là isolation boundary phía trên workspace;
- chốt VPSTTT và customer dùng template nghiệp vụ khác nhau;
- chốt shared SaaS trước, dedicated deployment sau;
- chốt discovery/capabilities public theo domain.

### Phase 1: registry và discovery

Hoàn tất:

- migration tạo zone/domain/deployment/automation registry;
- seed zone nội bộ VPSTTT và template theo loại zone;
- migration repair bảo đảm zone nội bộ luôn có primary workspace và hai kênh mặc định,
  kể cả khi registry được migrate trước khi hệ thống có workspace;
- đăng ký đầu tiên vào internal zone nhận `owner_id` và role `workspace_owner` trong cùng
  transaction; các đăng ký tiếp theo không thể chiếm lại quyền sở hữu;
- endpoint `/.well-known/vpsttt-chat`, discovery và capabilities;
- runtime trả web/API/WS/admin URL cùng workspace mặc định.

### Phase 2: cô lập zone

Hoàn tất:

- mọi workspace bắt buộc có `zone_id`;
- access token và session chứa `zone_id`, `workspace_id`, `domain`;
- token phải khớp domain đã resolve từ request host;
- register, login, Google login, refresh, session và `/me` theo zone;
- users, contacts, notifications, push devices và RBAC theo zone;
- WebSocket user room có prefix zone; join workspace phải thuộc token zone;
- call event, contact event và notification không phát chéo zone;
- audit log tự điền zone từ workspace;
- order bot/API của VPSTTT chỉ hoạt động trong `vpsttt_internal`;
- customer workspace không seed bot hoặc nội dung nghiệp vụ VPSTTT.

### Phase 3: claim, provisioning và TLS động

Hoàn tất:

- claim domain public hợp lệ, normalize và chống claim trùng;
- DNS TXT challenge có TTL, attempt count, last error và verify idempotent;
- provisioning transaction tạo zone runtime hoàn chỉnh;
- Caddy on-demand TLS dùng endpoint ask fail-closed và shared secret;
- catch-all HTTPS proxy giữ nguyên host cho API, web và WebSocket;
- same-origin CORS động chỉ chấp nhận origin khớp request host/protocol;
- Compose hỗ trợ `TLS_PROXY_MODE=caddy`; Nginx/certbot cũ vẫn là profile tĩnh.

### Phase 4: client domain-first và automation theo zone

Hoàn tất:

- web auth lưu domain/runtime đã discovery và luôn gửi domain khi refresh;
- web production chuyển sang `web_base_url` của zone trước password/Google login và sau
  verify domain, tránh lưu token customer trên origin control-plane;
- đăng ký domain mới có màn hướng dẫn TXT và thao tác verify;
- API client và realtime tự dùng `api_base_url`/`ws_base_url` của zone;
- backend phục vụ cả `/ws` và `/api/v1/ws`, nên discovery WebSocket chạy trực tiếp hoặc qua ingress;
- local development ưu tiên zone `vpsttt_internal` duy nhất và giữ nguyên port API/WS từ env frontend;
- registration mode hỗ trợ `open`, `invite_only`, `closed`;
- automation template được lọc theo `zone_kind`;
- create/list/update/enable/disable/delete installation theo zone;
- config được validate theo schema và không cho secret inline;
- JSON Schema được compile theo Draft 7, kiểm tra type/format/items ở create và update;
  external `$ref` bị chặn, secret scanner đi đệ quy qua cả object và array;
- `secret_ref` chỉ nhận provider allowlist;
- Admin có màn cài, bật/tắt và gỡ automation theo zone hiện tại;
- outgoing webhook ký `HMAC-SHA256(timestamp + "." + body)`;
- signing secret được AES-GCM bằng `WEBHOOK_SIGNING_SECRET`, không lưu plaintext;
- webhook sender không follow redirect và chặn private/loopback/link-local IP;
- webhook legacy không thể khôi phục khóa được tự chuyển sang `disabled`.

### Phase 5: zone control plane

Hoàn tất:

- API và Admin UI xem/cập nhật zone hiện tại;
- thêm domain phụ, DNS verify, chuyển primary và soft-delete domain;
- suspend/resume/archive zone có audit log và revoke session;
- suspended zone chỉ cho phép đăng nhập owner hiện hữu và gọi đúng endpoint recovery để resume;
  đăng ký mới, tạo Google identity mới và mọi API nghiệp vụ vẫn bị khóa;
- lifecycle đồng bộ workspace, deployment và automation webhook runtime;
- deployment request có `Idempotency-Key`, trạng thái bền vững và audit log;
- shared runtime hỗ trợ nhiều alias domain nhưng vẫn trả URL theo Host đang truy cập.

Lưu ý: `dedicated_compose` và `dedicated_k8s` hiện là deployment request cho external
provisioner. Hệ thống không công bố stack dedicated là sẵn sàng nếu chưa có driver hạ tầng.

### Phase 6: executable automation runtime

Hoàn tất:

- template `customer-basic-webhook-bot` dùng runtime `outgoing_webhook`;
- install automation tạo outgoing webhook trong cùng transaction;
- signing secret được tạo ngẫu nhiên, chỉ trả một lần và lưu AES-GCM envelope;
- enable/disable/update/delete installation đồng bộ runtime webhook;
- outbox worker tạo delivery thật, retry và ghi delivery log bằng cơ chế webhook sẵn có;
- `event_types` lọc event, `channel_slug` lọc đúng channel trong workspace;
- suspend/archive zone tắt runtime; resume chỉ bật lại installation đang `enabled`;
- Admin hiển thị runtime readiness và one-time signing secret.

### Phase 7: enterprise guardrails

Hoàn tất:

- JWT bắt buộc Host phải là active domain của token zone, kể cả khi discovery thất bại;
- API token và incoming webhook bị bind vào Host-zone, không thể gọi chéo tenant;
- API token chỉ authenticate khi workspace và zone vẫn active;
- quota theo zone cho workspace, member, storage, automation và webhook;
- hard quota được enforce bằng transaction lock và PostgreSQL trigger; monitor mode chỉ báo usage;
- Admin có quota usage/limit editor và API trả lỗi `ZONE_QUOTA_EXCEEDED`;
- OIDC provider contract theo zone gồm HTTPS issuer, client ID, scopes, claim mapping và
  `client_secret_ref`; không nhận client secret inline;
- capability `oidc_configuration=true`; `sso` chỉ được công bố khi runtime OIDC và ít nhất
  một provider của zone có secret reference khả dụng;
- `federation=false` bị ép fail-closed, không cho metadata tùy ý công bố tính năng chưa có runtime.

### Phase 8: OIDC SSO runtime theo zone

Hoàn tất:

- discovery chỉ công bố tên và ID provider đang sẵn sàng, không lộ issuer, client ID hoặc secret ref;
- authorization-code flow dùng OIDC discovery, PKCE S256, nonce và state ngẫu nhiên;
- state và completion code chỉ lưu dạng SHA-256, có TTL ngắn và consume nguyên tử một lần;
- PKCE verifier được mã hóa AES-GCM với AAD trước khi lưu;
- callback chỉ redirect về relative `return_to` cùng origin; access/refresh token của VPSTTT
  không xuất hiện trên URL;
- issuer, audience, chữ ký, expiry và nonce của ID token được verify bằng OIDC verifier;
- client secret chỉ resolve từ alias `env://name` có trong allowlist `OIDC_CLIENT_SECRETS`;
  provider không thể dùng tên biến môi trường tùy ý để đọc secret hệ thống;
- liên kết identity dùng cặp `(provider_id, subject)`, hỗ trợ JIT membership/user theo policy
  của từng provider và có tùy chọn bắt buộc `email_verified`;
- web và Admin Panel hỗ trợ discovery provider, chọn provider, callback completion và session;
- Admin có thể bật/tắt JIT, verified-email policy và provider theo zone;
- Admin có thể sửa issuer, client ID, scope, claim mapping, status và rotate/xóa
  `client_secret_ref` mà không đọc lại secret;
- cấu hình `OIDC_CLIENT_SECRETS` không có state secret hoặc alias sai định dạng làm startup
  fail-fast thay vì để runtime ở trạng thái nửa cấu hình;
- login OIDC phát hành session/token VPSTTT thông thường nên tiếp tục chịu Host-zone binding,
  lifecycle, membership, RBAC và audit như password login;
- outbound OIDC discovery/token/JWKS dùng HTTP client public-only để chặn SSRF tới private,
  loopback, link-local, CGNAT và metadata endpoint.

## API chính

Public:

- `GET /.well-known/vpsttt-chat`
- `GET /api/v1/discovery?domain=...`
- `GET /api/v1/capabilities?domain=...`
- `GET /api/v1/auth/oidc/providers?domain=...`
- `POST /api/v1/auth/oidc/start`
- `GET /api/v1/auth/oidc/callback`
- `POST /api/v1/auth/oidc/complete`

Domain onboarding:

- `POST /api/v1/zones/claims`
- `GET /api/v1/zones/claims/{domain_id}`
- `POST /api/v1/zones/claims/{domain_id}/verify`

Automation:

- `GET /api/v1/zones/current/automation-templates`
- `GET /api/v1/zones/current/automation-installations`
- `POST /api/v1/zones/current/automation-installations`
- `PATCH /api/v1/zones/current/automation-installations/{installation_id}`
- `DELETE /api/v1/zones/current/automation-installations/{installation_id}`

Zone control plane:

- `GET|PATCH /api/v1/zones/current`
- `POST /api/v1/zones/current/lifecycle`
- `POST /api/v1/zones/current/domains`
- `POST /api/v1/zones/current/domains/{domain_id}/primary`
- `DELETE /api/v1/zones/current/domains/{domain_id}`
- `GET|POST /api/v1/zones/current/deployment-requests`
- `GET|PUT /api/v1/zones/current/quota`
- `GET|POST /api/v1/zones/current/oidc-providers`
- `PATCH|DELETE /api/v1/zones/current/oidc-providers/{provider_id}`

Internal:

- `GET /internal/tenancy/caddy-ask?token=...&domain=...`

## Invariant bảo mật

- Không nhận `zone_id` tùy ý từ body để quyết định tenant hiện tại.
- Host resolve zone, token xác nhận zone, resource xác nhận workspace thuộc zone.
- Customer zone không được dùng order service của VPSTTT.
- Credential automation chỉ lưu ở secret manager và được tham chiếu bằng
  `secret_ref`.
- Outgoing webhook signing secret chỉ trả một lần và lưu dưới dạng AES-GCM
  envelope có version.
- OIDC state/completion code là single-use; PKCE verifier không lưu plaintext.
- OIDC client secret alias phải nằm trong allowlist runtime, không resolve trực tiếp từ process env.
- OIDC subject là định danh liên kết chính; email chỉ được dùng để link/JIT sau khi policy
  verified-email của provider được thỏa mãn.
- Endpoint Caddy ask yêu cầu secret và chỉ trả 204 cho public active domain.
- Shared SaaS là cô lập logic/application/database constraint, không phải một
  process hoặc database vật lý cho từng khách hàng.

## Runbook production

Biến bắt buộc:

```env
TLS_PROXY_MODE=caddy
LETSENCRYPT_EMAIL=admin@example.com
CADDY_ASK_SECRET=<random-secret>
CUSTOM_DOMAIN_DNS_TYPE=A
CUSTOM_DOMAIN_DNS_TARGET=<public-ip-of-edge>
WEBHOOK_SIGNING_SECRET=<random-secret-at-least-32-characters>
OIDC_STATE_SECRET=<random-secret-at-least-32-characters>
OIDC_CLIENT_SECRETS=company-sso=<oidc-client-secret>;partner-sso=<oidc-client-secret>
```

Mỗi provider cấu hình `client_secret_ref=env://company-sso`. Redirect URI phải đăng ký
chính xác tại IdP là:

```text
https://<zone-domain>/api/v1/auth/oidc/callback
```

Local development cho phép `http://localhost:<port>/api/v1/auth/oidc/callback`.
Để API `:8080` quay lại đúng frontend dev, `return_to` được phép là absolute
`http://localhost|127.0.0.1|[::1]:<port>/...` cùng hostname; production không có ngoại lệ này.
Các scheme `vault://`, `aws-secrets://`, `gcp-secrets://` và `azure-keyvault://` được chấp nhận ở
control plane để chuẩn bị migration, nhưng `sso=false` và login không chạy cho đến
khi adapter tương ứng được triển khai.

DNS của khách hàng:

- TXT `_vpsttt-chat.<domain>` dùng cho verify ownership;
- A/AAAA hoặc CNAME của `<domain>` trỏ tới public ingress chạy Caddy;
- port 80/443 phải tới được Caddy để ACME hoạt động.

Deploy:

```bash
cd deploy
./scripts/deploy-compose.sh
```

Sau migration `000023`, mọi outgoing webhook cũ bị `disabled` vì phiên bản cũ
đã băm mất signing key. Quản trị viên cần xóa và tạo lại webhook để nhận secret
mới.

## Xác minh đã chạy

- toàn bộ Go test;
- `go vet ./...`;
- toàn bộ frontend TypeScript typecheck;
- frontend lint, 42 unit test và production build cho web/Admin;
- Playwright smoke test màn đăng nhập Admin trên desktop/mobile và client-side hydration;
- Playwright public E2E: 7 pass, 3 authenticated scenarios được skip khi không cấp test account;
- OpenAPI parse test;
- Caddy 2.10 config validation;
- migration up/down/reapply trên PostgreSQL 18;
- migration repair internal workspace `000028` up/down/reapply;
- test token/session, cross-zone WebSocket, claim/verify/provision;
- test automation secret policy và lifecycle;
- test webhook AES-GCM, HMAC và outbound IP policy.
- test OIDC PKCE/nonce, state và completion replay, verified email, callback same-origin,
  secret alias allowlist và capability runtime-ready.
- test WebSocket discovery route, domain navigation/local runtime và JSON Schema automation.

## Phần ngoài Phase 8

- worker dựng `dedicated_compose`/`dedicated_k8s` thật;
- adapter Vault/AWS/GCP/Azure để resolve `secret_ref` lúc thực thi;
- workflow DAG/script execution engine ngoài outgoing webhook runtime;
- per-zone API request rate limit phân tán trên Redis;
- database RLS bổ sung như lớp phòng thủ thứ hai;
- user directory/database vật lý riêng cho từng zone; identity Phase 0-8 vẫn là platform identity;
- SCIM provisioning và group-to-role synchronization;
- federation peer protocol, trust handshake và remote conversation;
- native desktop OIDC callback qua system browser và app deep-link;
- mobile domain onboarding hoàn chỉnh.
