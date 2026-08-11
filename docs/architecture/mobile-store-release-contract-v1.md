# Mobile store release contract v1 cho self-host

Official Android package là một universal AAB do publisher ký. Customer tự host
API/database/storage và user nhập domain thủ công; không có AAB, Data Safety form
hay Android App Link riêng theo từng customer.

## Contract bắt buộc của instance

- Public TLS, `/ready` và `GET /api/v1/discovery?domain=<domain>` hoạt động.
- Discovery trả runtime HTTPS/WSS an toàn và `data.discovery.zone.id` là UUID
  identity bất biến của instance.
- Legal Policy Contract v1: `/api/v1/auth/legal-documents` trả đúng Terms (gồm
  Terms of Use + AUP) và Privacy document/version khớp portal publisher; backend
  lưu explicit versioned acceptance, hỗ trợ report/block/moderation và deletion.
- Operator chịu trách nhiệm account, UGC, moderation, retention, backup và data
  request trên instance; publisher chịu trách nhiệm binary/SDK, public policies,
  store listing, reference/reviewer instance và relay họ trực tiếp vận hành.

## App Links

`WEBTUI_APP_LINK_HOST=chat.vpsttt.com` là publisher host tĩnh trong official
manifest. Domain customer được nhập thủ công và không phục vụ association file
của official package. Chỉ custom-branded build có package/signing/Firebase/portal
riêng mới khai báo App Links riêng.

## Official push relay

Customer dùng:

```dotenv
PUSH_RELAY_URL=https://relay.publisher.example/push-relay/v1/deliveries
PUSH_RELAY_TOKEN=<unique-32+-character-token>
PUSH_RELAY_INSTANCE_ID=<exact-data.discovery.zone.id-uuid>
```

Publisher dùng cùng UUID làm key:

```dotenv
PUSH_RELAY_SERVER_ENABLED=true
PUSH_RELAY_PUBLISHERS=<exact-zone-uuid>=<same-token>
FIREBASE_PROJECT_ID=<official-project>
FIREBASE_SERVICE_ACCOUNT_JSON_BASE64=<secret>
```

Không dùng customer slug/domain làm relay identity và không giao official FCM
service account cho customer. Xem runbook chi tiết tại
`docs/operations/push-notifications.md`.

## Điều kiện để official app kết nối ổn định

Customer phải deploy backend version tương thích trước khi bật feature mobile
mới và giữ migration/rollback an toàn. Publisher chỉ upload AAB lên Play; backend
và database không nằm trong bundle. Reference/reviewer instance của publisher
phải luôn online, có reusable accounts không OTP/geo gate, seeded UGC, moderator
và deletion-test account để Google kiểm tra toàn bộ flow.
