# Kế hoạch cải thiện Push, Realtime, Session và File Transfer

Ngày nghiên cứu: 2026-07-25
Phạm vi: `webtui-chat-self-host`, `webtui-chat-mobile`, `webtui-chat-desktop` và dịch vụ phía Vendor cần bổ sung.

## 1. Kết luận điều hành

Bốn nhận xét ban đầu đều hợp lý, nhưng trạng thái mã nguồn hiện tại đã đi xa hơn
sơ đồ được đánh giá:

| Hạng mục | Đã có trong mã nguồn | Khoảng trống thực tế | Ưu tiên |
|---|---|---|---|
| Push mobile | Mobile lấy/refresh FCM token; instance lưu device; worker gửi FCM HTTP v1, có retry | Firebase service account vẫn phải nằm ở customer instance; chưa có Vendor Push Gateway; chưa xử lý invalid token theo từng device | P0 trước khi phát hành official app |
| Realtime reconnect | Web/Desktop reconnect có backoff và reload query; Mobile có `/sync`, cursor và ack | Web/Desktop chưa dùng cursor; WebSocket không mang event ID/cursor; event có thể bị drop im lặng; cursor dùng chung bảng outbox có lỗi hết hạn | P0 vì liên quan tính đúng dữ liệu |
| Session/revocation | Refresh token được hash, rotate và kiểm tra `revoked_at`; có revoke/revoke-all; Mobile xóa token khi đổi server | Không thể revoke server cũ ngay khi server offline; access JWT chưa gắn `session_id` nên còn hiệu lực tới khi hết TTL; rotation chưa phát hiện replay theo token family | P1 security hardening |
| File/media | Có local/MinIO/S3, upload queue và stream qua API | Chưa upload trực tiếp; API vẫn nhận và chuyển toàn bộ byte sang object storage; chưa có initiate/complete/abort và presigned URL | P1 scalability |

Thứ tự khuyến nghị:

1. Chốt contract/version/capability và định danh instance.
2. Làm Realtime Sync v2 trước để có nền dữ liệu bền vững cho mọi platform.
3. Dựng Vendor Push Gateway và chuyển official app sang relay.
4. Hoàn thiện session family, queued revocation và access-token revocation.
5. Thêm direct upload bằng presigned URL, giữ proxy upload làm fallback.
6. Chaos test, security test, canary và rollout theo capability.

Không nên viết lại toàn bộ hệ thống. Mobile sync, notification jobs, push device,
object-store adapter và upload queue hiện tại đều có thể tái sử dụng.

## 2. Bằng chứng từ mã nguồn hiện tại

### 2.1 Push

- Mobile đăng ký device tại `POST /api/v1/mobile/devices`, lắng nghe
  `FirebaseMessaging.instance.onTokenRefresh` và gửi `push_provider=fcm` trong
  `webtui-chat-mobile/lib/core/notifications/push_notification_service.dart`.
- Customer worker đang ký OAuth bằng Firebase service account rồi gọi trực tiếp
  `https://fcm.googleapis.com/v1/projects/.../messages:send` tại
  `backend/internal/modules/notifications/infrastructure/fcm/sender.go`.
- Bộ cài self-host hiện công khai hai cấu hình
  `FIREBASE_PROJECT_ID` và `FIREBASE_SERVICE_ACCOUNT_JSON_BASE64`.
- Notification job đã có trạng thái/retry, nhưng một job gửi tuần tự tới tất cả
  token. Một token lỗi làm dừng vòng lặp và retry cả job; lỗi
  `UNREGISTERED/INVALID_ARGUMENT` chưa tự revoke device tương ứng.
- iOS hiện cũng nhận FCM registration token. FCM chịu trách nhiệm chuyển tiếp
  sang APNs sau khi Vendor cấu hình APNs key cho Firebase project của app.

### 2.2 Realtime

- `platform/websocket.Event` chỉ có `type`, `room`, `user_id`, `payload` và
  `timestamp`; chưa có `event_id` hoặc cursor.
- Khi send channel của client đầy, `Manager.Broadcast` đi vào nhánh `default`
  và bỏ event mà không báo client.
- Web/Desktop dùng chung React app. Khi online/focus/resume, hook
  `use-channel-realtime.ts` chỉ invalidate REST query và reconnect với backoff
  tối đa 15 giây.
- Backend đã có `GET /workspaces/{workspace_id}/sync` và `/sync/ack`; Mobile đã
  phân trang và lưu cursor.
- Mobile hiện ack/lưu cursor ngay trong use case lấy dữ liệu, trước khi lớp UI
  invalidate và đọc lại state. Semantics nên đổi thành “apply thành công rồi
  mới ack”.
- Cursor hiện là UUID của `outbox_events`; job cleanup xóa outbox
  `published/dead` sau 30 ngày. Nếu client gửi một UUID đã bị xóa, CTE
  `cursor_event` rỗng và query hiện trả rỗng thay vì báo cursor hết hạn hoặc
  bootstrap lại.
- WebSocket manager nằm trong memory của một API node. Khi scale nhiều node,
  realtime cần fan-out qua broker; durable sync phải vẫn là nguồn khôi phục.

### 2.3 Session

- Backend lưu hash refresh token, kiểm tra session active/revoked/expired và
  rotate refresh token sau mỗi lần refresh.
- Mobile dùng secure storage và đã xóa access token, refresh token, workspace
  scope khi discovery sang API origin khác.
- Access token mặc định sống 15 phút. Claims chưa có `sid`/session ID; middleware
  chỉ verify chữ ký, expiry và zone/domain, nên revoke session chưa vô hiệu hóa
  access token ngay lập tức.
- Bảng session chỉ giữ refresh-token hash hiện tại. Khi một token cũ bị replay,
  backend thấy “không tồn tại” nhưng không có đủ token-family history để thu hồi
  token mới đang active.
- Logout khi server offline chỉ có thể xóa local state. Không hệ thống nào có
  thể cam kết remote revoke tức thì khi authorization server không thể nhận
  request; cần mô tả đúng guarantee và có hàng đợi revoke sau khi server trở lại.

### 2.4 File/media

- API dùng `c.FormFile`, mở multipart body, tính SHA-256 qua `io.TeeReader`, rồi
  gọi `store.Put`. Dữ liệu được stream thay vì đọc toàn bộ vào RAM, nhưng vẫn đi
  qua network, disk multipart tạm và process API.
- Adapter MinIO/S3 hiện chỉ có `Put`, `Get`, `Delete`, `Health`; chưa có
  `PresignPut`, `Stat`, multipart initiate/complete/abort.
- Giới hạn hiện tại là 100 MiB. Direct single PUT đã giảm tải đáng kể; multipart
  upload chỉ cần bật ngay nếu muốn resume tốt trên mạng mobile hoặc tăng giới
  hạn file.
- Schema đã có các trạng thái `uploading`, `uploaded`, `processing`, `ready`,
  `failed`, `deleted` và cron cleanup upload lỗi, nên phù hợp với flow
  initiate/complete.

## 3. Các quyết định kiến trúc đề xuất

### ADR-01: Push của official app đi qua Vendor Gateway

Customer instance không giữ Firebase/APNs signing credential của Vendor.
Gateway là trusted server environment sở hữu Firebase service account và APNs
configuration của official app.

Vẫn giữ ba mode để không làm mất tính self-host:

- `relay`: mặc định cho official app; instance gọi Vendor Push Gateway.
- `direct`: customer-branded build dùng Firebase/APNs riêng của customer.
- `disabled`: self-host thuần, không có background push.

### ADR-02: WebSocket là transport best-effort, event log là source of truth

Mọi sự kiện làm thay đổi dữ liệu bền vững phải có event ID và cursor trong một
durable workspace event log. WebSocket có thể trễ, trùng hoặc mất; client luôn
phục hồi qua `/sync`.

Typing, ICE candidate và các tín hiệu tức thời được phân loại là ephemeral,
không replay. Trạng thái kết thúc cuộc gọi, message, reaction, pin, attachment,
read state và notification là durable.

### ADR-03: Tách client event log khỏi job outbox

Không tiếp tục dùng `outbox_events` vừa làm hàng đợi worker vừa làm lịch sử sync.
Hai loại dữ liệu có retention và lifecycle khác nhau.

Thêm `workspace_events` cho client replay; outbox có thể tham chiếu
`workspace_event_id` để publish broker/WebSocket/notification.

### ADR-04: Một active server, credential được scope theo instance

Client chỉ có một server active tại một thời điểm. Credential của server A
không bao giờ được gắn vào request tới server B.

Thêm immutable `instance_id` vào discovery. Secure storage và pending
revocation được key theo `instance_id + canonical_api_origin`, không dùng key
token toàn cục cho các dữ liệu chờ xử lý.

### ADR-05: Object storage dùng capability-based direct upload

API chỉ cấp quyền upload ngắn hạn cho một object key ngẫu nhiên, sau đó xác minh
object và commit metadata. Presigned URL là bearer capability: TTL ngắn, key
không đoán được, header/checksum được ký, không ghi URL vào log.

Local storage hoặc object store không hỗ trợ presign tiếp tục dùng proxy
multipart hiện tại.

## 4. Thiết kế và backlog chi tiết

### A. Vendor Push Notification Gateway

#### A.1 Luồng mục tiêu

```text
Mobile official app
  -> lấy/refresh FCM registration token
  -> Customer Instance: register device
  -> Vendor Gateway: bind token, nhận push_handle

Customer notification outbox
  -> Gateway sender bằng instance credential
  -> Vendor Push Gateway
  -> FCM HTTP v1
  -> APNs hoặc Android transport
  -> Mobile official app
```

Customer instance lưu `push_handle` và token hash. Raw registration token chỉ
được giữ nếu đang chạy `direct` mode; ở `relay` mode, xóa raw token sau khi bind
thành công.

#### A.2 Contract Gateway v1

Các endpoint server-to-server:

```text
PUT    /v1/installations/{device_id}
POST   /v1/deliveries
GET    /v1/deliveries/{idempotency_key}
DELETE /v1/installations/{device_id}
```

Request bind tối thiểu:

```json
{
  "instance_id": "immutable-instance-uuid",
  "provider": "fcm",
  "platform": "android",
  "registration_token": "...",
  "app_id": "com.webtui.chat",
  "app_version": "1.0.0"
}
```

Response không trả lại token:

```json
{
  "push_handle": "ph_opaque_random",
  "status": "active",
  "updated_at": "2026-07-25T00:00:00Z"
}
```

Delivery request:

```json
{
  "instance_id": "immutable-instance-uuid",
  "idempotency_key": "notification-job-id:device-id",
  "push_handle": "ph_opaque_random",
  "event_type": "message_created",
  "data": {
    "workspace_id": "...",
    "channel_id": "...",
    "message_id": "..."
  },
  "display": {
    "title": "WebTUI Chat",
    "body": "Bạn có tin nhắn mới"
  },
  "ttl_seconds": 35
}
```

Không gửi email, user ID, nội dung tin nhắn hoặc tên file theo mặc định.
`display.body` có preview chỉ khi user và tenant cùng bật policy tương ứng.

#### A.3 Xác thực và chống abuse

- Mỗi instance có `client_id` và secret riêng, cấp qua quy trình activation của
  Vendor; không dùng chung Firebase secret.
- Request ký HMAC trên method, path, body hash, timestamp và nonce; chấp nhận
  clock skew tối đa 5 phút.
- Gateway lưu nonce ngắn hạn để chống replay và dedupe bằng
  `instance_id + idempotency_key`.
- Rate limit/quota theo instance, giới hạn payload và allowlist event type.
- `push_handle` phải thuộc đúng instance; instance không thể gửi tới handle của
  tenant khác.
- Secret rotation có overlap window; log phải redact Authorization, signature,
  registration token, push handle và notification body.
- Có thể nâng lên mTLS hoặc OAuth client credentials sau MVP mà không đổi
  delivery semantics.

#### A.4 Thay đổi trong các repo

Vendor service mới:

- Tạo private repo/service `webtui-push-gateway`.
- Adapter FCM HTTP v1/Admin SDK; retry theo `Retry-After`; phân loại lỗi
  retryable/permanent.
- Bảng installation được mã hóa token at rest; delivery dedupe có TTL.
- Dashboard theo instance: accepted, delivered-to-FCM, invalid token, throttled,
  latency; không log message content.

`webtui-chat-self-host`:

- Thêm `PUSH_MODE=relay|direct|disabled`,
  `PUSH_GATEWAY_URL`, `PUSH_GATEWAY_CLIENT_ID`,
  `PUSH_GATEWAY_CLIENT_SECRET`.
- Thêm relay implementation cho interface `PushSender`; giữ FCM sender hiện tại
  cho `direct`.
- Migrate `push_devices`: `push_handle`, `token_hash`, `provider_status`,
  `last_delivery_error`, `last_delivery_at`; raw token nullable.
- Tách notification job theo device/handle để một token lỗi không chặn các
  device khác.
- Khi Gateway/FCM trả invalid/unregistered, revoke device; với 429/5xx retry có
  exponential backoff + jitter; tôn trọng `Retry-After`.
- Discovery công bố `push.mode`, `push.gateway_available` và version contract.

`webtui-chat-mobile`:

- Giữ lấy token và `onTokenRefresh`.
- Xử lý registration state rõ ràng: `pending`, `active`, `permission_denied`,
  `relay_unavailable`.
- Re-register khi instance, app version hoặc FCM token thay đổi.
- Không coi push registration lỗi là lỗi login/chat.
- Test physical Android/iOS cho foreground, background, terminated, tap/deep
  link, token rotation, reinstall và notification permission.

#### A.5 Tiêu chí nghiệm thu

- Self-host official mode chạy background push mà không có Firebase service
  account trong customer instance.
- Secret của instance A không gửi được tới `push_handle` thuộc instance B.
- Cùng `idempotency_key` gọi nhiều lần chỉ tạo tối đa một FCM send logic.
- Một token `UNREGISTERED` bị revoke mà các device hợp lệ khác vẫn nhận.
- Gateway outage không chặn gửi message; job được retry và có DLQ/alert.
- P95 instance-to-FCM-accepted dưới 1 giây trong tải chuẩn; tỷ lệ 5xx của gateway
  dưới 0,1% theo cửa sổ 15 phút.
- Log/trace không chứa registration token, credential hoặc message preview.

### B. Realtime Sync v2 cho Mobile/Web/Desktop

#### B.1 Schema

Thêm migration:

```sql
CREATE TABLE workspace_events (
    sequence bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    event_id uuid NOT NULL DEFAULT gen_random_uuid() UNIQUE,
    workspace_id uuid NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    channel_id uuid,
    aggregate_type text NOT NULL,
    aggregate_id uuid NOT NULL,
    event_type text NOT NULL,
    event_version integer NOT NULL DEFAULT 1,
    payload jsonb NOT NULL DEFAULT '{}'::jsonb,
    occurred_at timestamptz NOT NULL DEFAULT now(),
    expires_at timestamptz NOT NULL
);

CREATE INDEX workspace_events_workspace_sequence_idx
ON workspace_events (workspace_id, sequence);
```

Sequence được dùng làm opaque monotonic cursor, không giả định gapless. Client
serialize cursor thành string để tránh giới hạn integer của JavaScript.

Đổi `mobile_sync_cursors` thành `sync_cursors`, bổ sung `platform`,
`client_version`, `last_sequence`, `last_seen_at`. Migration phải giữ lại cursor
mobile hiện có hoặc đánh dấu cần bootstrap một lần.

#### B.2 Event envelope v2

```json
{
  "version": 2,
  "event_id": "uuid",
  "cursor": "918273",
  "type": "MessageCreated",
  "workspace_id": "uuid",
  "room": "workspace:...:channel:...",
  "occurred_at": "2026-07-25T00:00:00Z",
  "payload": {}
}
```

Durable events bắt buộc có `event_id` và `cursor`. Ephemeral event có
`durable=false` và không được dùng để tiến cursor.

#### B.3 API

Giữ endpoint cũ cho mobile client cũ; bổ sung contract v2:

```text
GET  /api/v1/workspaces/{id}/sync?after={cursor}&until={cursor}&limit=200
POST /api/v1/workspaces/{id}/sync/ack
GET  /api/v1/workspaces/{id}/sync/bootstrap
```

Response:

```json
{
  "events": [],
  "next_cursor": "918273",
  "has_more": false,
  "high_watermark": "918300",
  "server_time": "2026-07-25T00:00:00Z"
}
```

Quy tắc:

- Cursor hợp lệ nhưng không có event mới: trả rỗng và giữ `next_cursor`.
- Cursor quá cũ/không còn trong retention: trả
  `410 SYNC_CURSOR_EXPIRED` với `min_cursor`, `high_watermark` và
  `recovery=bootstrap`.
- Cursor tương lai hoặc workspace khác: `400 INVALID_SYNC_CURSOR`.
- Ack chỉ tiến về phía trước; request ack cũ là idempotent no-op.
- Authorization lọc theo membership/room. Không trả event của channel user
  không còn quyền đọc; sự kiện thay đổi quyền phải buộc refresh access model.

#### B.4 Trình tự reconnect không tạo race

```text
1. Client mở WebSocket và join room với last_applied_cursor.
2. Server đăng ký room rồi trả Ready(high_watermark).
3. Client buffer durable live event tạm thời.
4. Client gọi /sync từ last_applied_cursor tới high_watermark.
5. Apply/dedupe event theo event_id; cập nhật REST/cache cần thiết.
6. Chỉ sau apply thành công mới lưu local cursor và gọi /sync/ack.
7. Sort/drain buffer, bỏ event <= last_applied_cursor.
8. Chuyển trạng thái realtime sang connected.
```

Nếu app chết ở bước 4-6, lần mở sau sẽ nhận lại event; reducer phải idempotent.
At-least-once + idempotency là guarantee, không tuyên bố exactly-once.

#### B.5 Server delivery

- Domain transaction ghi business state và `workspace_events` cùng transaction.
- Outbox tham chiếu workspace event và publish broker sau commit.
- Production nhiều API node phải dùng RabbitMQ/Redis stream để fan-out tới mỗi
  WebSocket node; event log trong PostgreSQL vẫn là source of truth.
- Slow consumer không bị drop im lặng. Đóng socket với close reason
  `slow_consumer`/retryable để client reconnect và catch-up.
- Thêm ping/pong/idle timeout và reconnect jitter; backoff của client tuân thủ
  nguyên tắc RFC 6455.

#### B.6 Áp dụng theo platform

Shared TypeScript API client:

- Thêm `syncClient`, event v2 types, cursor store abstraction và reconnect
  coordinator.
- Hook Web/Desktop dùng coordinator thay cho chỉ `invalidateRealtimeQueries`.
- Desktop dùng cùng implementation React; cursor lưu trong platform storage
  scope `instance + workspace + device`.

Mobile:

- Đổi use case thành `fetch -> apply/invalidate -> persist/ack`.
- Tiếp tục dùng device ID hiện có.
- Bootstrap khi cursor expired; không hiển thị network-degraded vĩnh viễn do
  cursor cũ.

Reducer/apply:

- `MessageCreated/Updated/ReactionChanged/Pinned`: upsert theo message ID/version.
- `MessageDeleted`: tombstone/remove idempotent.
- `AttachmentCreated`: refresh attachment/media key tương ứng.
- Notification/read state: upsert theo ID.
- Typing/ICE: bỏ nếu miss; không catch-up.
- Call: phục hồi bằng current call state, không replay ICE/SDP cũ.

#### B.7 Retention và bootstrap

- Retention mặc định 30 ngày, cấu hình được.
- Purge theo absolute retention; không để một device bỏ lâu chặn cleanup vô hạn.
- Device quay lại sau retention phải bootstrap REST snapshot rồi nhận cursor
  high-watermark mới.
- Có cron xóa cursor device không hoạt động sau 90 ngày.
- Không dùng FK `ON DELETE SET NULL` làm tín hiệu ngầm; cursor expired phải là
  trạng thái API rõ ràng.

#### B.8 Tiêu chí nghiệm thu

- Mất mạng giữa hai message rồi reconnect: Web, Desktop và Mobile đều thấy đủ
  hai message mà không reload thủ công.
- Event trùng, đảo thứ tự hoặc client crash trước ack không làm duplicate state.
- Cursor cũ hơn retention tự bootstrap, không bị kẹt trả danh sách rỗng.
- Test ít nhất 1.000 event qua nhiều page và reconnect giữa các page.
- Test hai API/WebSocket node: event tạo ở node A tới client nối node B.
- Slow consumer bị disconnect có chủ đích và phục hồi đủ bằng `/sync`.
- P95 catch-up 200 event dưới 500 ms ở database baseline đã chốt; metric có
  `sync_lag_events`, `sync_lag_seconds`, `cursor_expired_total`.

### C. Session và revocation khi đổi server

#### C.1 Guarantee cần công bố

- Đổi server: local isolation là tức thì.
- Remote revocation: tức thì nếu server cũ reachable; eventual nếu server cũ
  offline; không thể cam kết tức thì khi authorization server không hoạt động.
- Credential server A không bao giờ được gửi tới API origin của server B.
- Revoke session phải chặn refresh ngay và chặn access token trong một cửa sổ
  đã định nghĩa.

#### C.2 Backend session hardening

Thêm claims:

- `sid`: session ID.
- `iss`: immutable instance ID/canonical issuer.
- `aud`: API audience.
- `jti`: access token ID nếu cần audit/revocation chi tiết.

Thêm refresh-token family:

```text
session_refresh_tokens
  id / token_hash / session_id / family_id
  issued_at / used_at / revoked_at / replaced_by
```

Refresh chạy trong transaction và lock session:

1. Token current, chưa dùng: đánh dấu used, tạo token kế tiếp.
2. Token đã dùng bị replay: revoke toàn session/family và audit security event.
3. Session revoked/expired/domain mismatch: không phát token mới.

Access-token revocation:

- Khi revoke session, ghi DB và cache `revoked:sid` với TTL bằng thời gian access
  token còn lại.
- Middleware kiểm tra `sid`; Redis/cache unavailable có policy rõ:
  sensitive endpoint fallback DB, endpoint thường chấp nhận short TTL hoặc
  fail-closed tùy deployment profile.
- Có thể giảm access TTL từ 15 xuống 5-10 phút nếu latency/cache budget cho phép.

#### C.3 Client server switch

Secure pending revocation record:

```json
{
  "instance_id": "...",
  "api_origin": "https://chat.company-a.com",
  "session_id": "...",
  "refresh_token": "encrypted-secure-storage-only",
  "expires_at": "...",
  "attempt_count": 0
}
```

Flow:

1. Thử logout/revoke ở server hiện tại với timeout ngắn.
2. Nếu network/server offline, chuyển refresh token cũ sang pending queue mã hóa.
3. Xóa ngay active access/refresh token, WebSocket, workspace cache và outbox
   không thuộc server mới.
4. Discovery/login server B dùng credential store mới.
5. Background/resume job thử revoke đúng origin server A; kiểm tra discovery
   `instance_id` khớp record trước khi gửi credential.
6. Thành công, token hết hạn hoặc server identity không còn khớp: xóa pending
   credential; identity mismatch tạo cảnh báo và không gửi token.

Web browser theo hostname không cần multi-server queue như Desktop/Mobile, nhưng
logout offline vẫn phải xóa local state. Desktop và Mobile cần cùng semantics
qua platform secure storage.

#### C.4 Tiêu chí nghiệm thu

- Đổi A -> B khi A offline không gửi bất kỳ header/token A nào tới B.
- Khi A online lại, pending logout revoke session A và xóa pending credential.
- Refresh token cũ bị replay làm revoke token family đang active.
- Revoke từ màn Privacy làm access token của device bị từ chối trong SLA đã
  chọn, mục tiêu dưới 5 giây khi cache hoạt động.
- App restart/process death giữa lúc đổi server không hydrate nhầm token A cho B.
- Log, crash report và analytics không chứa access/refresh token.

### D. Direct file/media upload

#### D.1 Flow v1

```text
1. Client -> API: initiate(name, size, mime, checksum, channel/message)
2. API: permission/quota/policy, tạo file status=uploading và object key random
3. API -> Client: presigned PUT/POST, headers, expires_at, upload_id
4. Client -> S3/MinIO: upload trực tiếp
5. Client -> API: complete(upload_id, checksum/etag)
6. API -> object store: HEAD/Stat và verify size/checksum/content type
7. API transaction: status=uploaded/ready, file version, attachment, audit/event
```

Endpoints:

```text
POST   /api/v1/workspaces/{id}/files/uploads
POST   /api/v1/workspaces/{id}/files/uploads/{upload_id}/complete
DELETE /api/v1/workspaces/{id}/files/uploads/{upload_id}
```

Initiate response:

```json
{
  "upload_id": "uuid",
  "file_id": "uuid",
  "strategy": "presigned_put",
  "method": "PUT",
  "url": "https://object-store/...",
  "headers": {
    "content-type": "image/jpeg",
    "x-amz-checksum-sha256": "base64..."
  },
  "expires_at": "2026-07-25T00:10:00Z"
}
```

Không trả bucket credential. Không đưa object key hoặc presigned URL vào log.

#### D.2 ObjectStore interface

Mở rộng adapter:

```text
PresignPut / PresignPost
Stat
Delete
CreateMultipart
PresignUploadPart
CompleteMultipart
AbortMultipart
```

Phase đầu chỉ bắt buộc `PresignPut/Post + Stat + Delete`. Với local provider,
API trả `strategy=proxy_multipart` và dùng endpoint hiện tại.

#### D.3 Chính sách an toàn

- TTL presigned 5-10 phút; object key UUID dưới prefix workspace.
- Ký `Content-Type`, checksum và các header bắt buộc.
- Ưu tiên presigned POST policy có `content-length-range` nếu provider tương
  thích; presigned PUT phải verify size bằng `HEAD` và xóa object sai ngay.
- Complete chỉ thành công một lần; request lặp là idempotent.
- File chưa complete không được attach/download/list như file ready.
- Quota được reserve ở initiate, release ở abort/expiry; chống user mở hàng
  nghìn upload rồi bỏ.
- CORS bucket chỉ allow exact customer web origins, method/header cần thiết;
  không dùng wildcard origin kèm credential.
- Lifecycle/cron xóa object orphan và abort multipart chưa complete.
- Antivirus/thumbnail/transcode, nếu có, chạy sau complete ở trạng thái
  `processing`; chỉ `ready` mới được download theo policy.

#### D.4 Single PUT và multipart

Với giới hạn 100 MiB hiện tại:

- Rollout direct single PUT trước để giảm tải API.
- Bật multipart khi file từ 100 MiB trở lên, hoặc sớm hơn cho mobile nếu product
  yêu cầu pause/resume và mạng yếu.
- Multipart lưu `provider_upload_id` và part ETag/checksum; retry riêng từng
  part, complete theo thứ tự; luôn có abort/lifecycle cleanup.

#### D.5 Client

Shared TypeScript:

- Upload queue hỗ trợ strategy do server trả về, progress, cancel, retry,
  expired URL -> re-initiate.
- Web/Desktop upload trực tiếp với CORS; Desktop vẫn dùng cùng queue.

Mobile:

- Dio/raw client upload trực tiếp, progress/cancel.
- Persist upload intent an toàn để retry; không lưu presigned URL quá expiry.
- Attachment chỉ chuyển `attached` sau complete + attach API thành công.

#### D.6 Tiêu chí nghiệm thu

- Với S3/MinIO, byte file không đi qua API container.
- Upload sai size/checksum không thể chuyển file sang `ready`.
- URL hết hạn, reuse, abort và complete lặp có kết quả xác định.
- Local storage vẫn upload được qua proxy mà không đổi UX.
- Browser CORS pass đúng origin và fail origin ngoài allowlist.
- 20 upload đồng thời không làm tăng API memory/network theo kích thước file.
- Orphan upload được xóa/abort trong SLA 24 giờ.

## 5. Nền tảng dùng chung cần làm trước

### FND-01: Immutable instance identity

- Sinh `INSTANCE_ID` khi cài đặt lần đầu, persist ngoài image/container.
- Discovery trả `instance_id`, `contract_version` và capabilities:
  `realtime_sync_v2`, `push_mode`, `direct_upload`.
- Không dùng domain làm identity duy nhất vì domain có thể được chuyển chủ.

### FND-02: Version và backward compatibility

- Mọi endpoint mới additive; endpoint multipart, `/sync` v1 và direct FCM mode
  cũ chưa xóa trong rollout đầu.
- Client đọc capability, không suy đoán theo server version.
- Server giữ event payload version; client bỏ qua event type/version chưa biết và
  trigger targeted refresh khi cần.

### FND-03: Observability

Metrics tối thiểu:

- Push: queue depth, age, gateway latency/status, invalid token, retries, DLQ.
- Realtime: connected clients, dropped/slow consumer, reconnect, cursor lag,
  catch-up page/latency, expired cursor.
- Auth: refresh success/failure/replay, revoked session cache, pending client
  revoke age.
- File: initiate/complete/abort, bytes proxy/direct, orphan count, checksum
  mismatch, object-store latency.

Alert theo SLO, không alert chỉ vì một customer instance đang offline có chủ ý.

## 6. Lộ trình thực hiện

Ước lượng dưới đây là engineer-days, chưa gồm thời gian chờ Apple/Google review.

| Phase | Nội dung | Phụ thuộc | Ước lượng | Kết quả |
|---|---|---|---:|---|
| 0 | ADR, instance ID, capability/version contract, privacy policy | Không | 3-5 | Contract khóa, migration plan |
| 1 | Workspace event log, Sync API v2, stale cursor/bootstrap, broker fan-out | Phase 0 | 7-10 | Backend realtime bền vững |
| 2 | Web/Desktop/Mobile sync coordinator, apply-then-ack, chaos tests | Phase 1 | 6-9 | Cả 3 platform không mất durable event |
| 3 | Vendor Push Gateway, activation/auth/quota, relay sender, token lifecycle | Phase 0 | 10-15 | Official app push không phát tán Firebase secret |
| 4 | Session claims/family/replay, revocation cache, queued offline revoke | Phase 0 | 6-9 | Server switch và revoke có guarantee rõ |
| 5 | Presign/stat API, S3/MinIO adapter, clients, CORS/orphan cleanup | Phase 0 | 8-12 | Media không đi qua API ở object-store mode |
| 6 | Load/security/chaos/canary, runbook và rollout | Phase 1-5 | 5-8 | Production readiness |

Nếu có 1 backend, 1 web/desktop, 1 mobile và 0,5 DevOps/SRE, các phase 1, 3, 4,
5 có thể chồng lấn sau Phase 0; thời gian lịch dự kiến 6-8 tuần. Làm tuần tự bởi
một người cần khoảng 10-13 tuần.

## 7. Chiến lược rollout

1. Deploy migration/schema và capability ở trạng thái off.
2. Deploy server hỗ trợ cả v1/v2; quan sát shadow metrics.
3. Canary Sync v2 cho Desktop nội bộ, sau đó Web, rồi Mobile.
4. Gateway chạy staging với Firebase project/app staging; physical-device test.
5. Bật `PUSH_MODE=relay` cho một tenant canary; giữ `direct/disabled` rollback.
6. Bật direct upload theo tenant/provider; file nhỏ hoặc provider local vẫn proxy.
7. Sau ít nhất hai release client và một retention window ổn định mới cân nhắc
   deprecate contract cũ.

Rollback:

- Capability server tắt ngay để client quay về invalidate/proxy/direct mode cũ.
- Migration chỉ additive trong hai release đầu.
- Không xóa cursor/outbox/file/session column cũ trong cùng release rollout.

## 8. Test plan bắt buộc

### Contract/unit

- OpenAPI schema và generated types cho sync/push/upload.
- Cursor monotonic/idempotent ack/stale cursor.
- Push HMAC, nonce, tenant binding, idempotency và error mapping.
- Refresh-family replay và revoked `sid`.
- Presign expiry, header/checksum/size và complete idempotency.

### Integration

- PostgreSQL + RabbitMQ + hai API nodes + hai WebSocket nodes.
- MinIO và AWS S3-compatible test matrix.
- Gateway fake FCM + FCM staging project.
- Upgrade từ database/client hiện tại, bao gồm cursor mobile cũ.

### Chaos/E2E

- Airplane mode, network flap, sleep/wake, process kill trước/sau ack.
- Gateway/FCM 429/500/timeout và token invalid.
- Server A offline -> đổi B -> A online lại.
- Object-store timeout giữa upload và complete; URL hết hạn; orphan cleanup.
- Slow WebSocket consumer và queue full.

### Security

- Cross-tenant push handle.
- Replay gateway signature/idempotency key.
- Token A gửi nhầm B, refresh-token replay và session revoke.
- Presigned URL leakage/reuse, object-key traversal, quota abuse, MIME spoofing.
- Log/trace/analytics secret scan.

## 9. Definition of Done toàn chương trình

- Không còn Firebase/APNs signing credential của official app trong customer
  instance.
- Web, Desktop và Mobile có cùng durable reconnect guarantee và cùng cursor
  contract.
- Session switch/revoke có local/remote guarantee được test và mô tả đúng.
- S3/MinIO upload trực tiếp, local provider fallback, không giảm authorization.
- OpenAPI, deployment guide, privacy disclosure, runbook, dashboard và alert đều
  được cập nhật.
- Backward compatibility được chứng minh bằng ít nhất một server cũ/client mới
  và server mới/client cũ trong support matrix.

## 10. Các mặc định đề xuất cần phê duyệt

Nếu chưa có quyết định sản phẩm khác, dùng các mặc định sau:

- Push official app: `relay`, payload không có message preview.
- Customer-branded build: cho phép `direct`.
- Durable event retention: 30 ngày; device cursor retention: 90 ngày.
- Access token TTL: 10 phút; revoke cache SLA dưới 5 giây.
- Presigned URL TTL: 10 phút.
- File tối đa: giữ 100 MiB; direct single PUT trước, multipart ở phase sau hoặc
  khi tăng giới hạn.
- Orphan upload cleanup: 24 giờ.

## 11. Nguồn kỹ thuật chính

- Firebase yêu cầu app server/trusted server environment giữ an toàn credential
  và registration token:
  <https://firebase.google.com/docs/cloud-messaging/server-environment>
- Flutter FCM trên Apple cần APNs authentication key trong Firebase project:
  <https://firebase.google.com/docs/cloud-messaging/flutter/get-started>
- Quản lý stale/invalid registration token:
  <https://firebase.google.com/docs/cloud-messaging/manage-tokens>
- WebSocket RFC 6455 yêu cầu reconnect có backoff nhưng không cung cấp replay
  ứng dụng:
  <https://datatracker.ietf.org/doc/html/rfc6455#section-7.2.3>
- OAuth token revocation:
  <https://datatracker.ietf.org/doc/html/rfc7009>
- OAuth 2.0 Security BCP về refresh-token binding, rotation/replay và revoke:
  <https://datatracker.ietf.org/doc/html/rfc9700#section-4.14>
- S3 presigned upload và checksum:
  <https://docs.aws.amazon.com/AmazonS3/latest/userguide/using-presigned-url.html>
- S3 CORS:
  <https://docs.aws.amazon.com/AmazonS3/latest/userguide/cors.html>
- S3 multipart/resume và cleanup:
  <https://docs.aws.amazon.com/AmazonS3/latest/userguide/mpuoverview.html>

## 12. Xác minh đã thực hiện khi lập kế hoạch

- Backend targeted tests pass cho sync, auth, files, notifications, FCM sender,
  push devices và WebSocket.
- 15 mobile targeted tests pass cho catch-up sync, auth/logout/refresh queue và
  realtime reducer/WebSocket URL.
- Frontend unit suite chưa chạy trong workspace hiện tại vì dependencies chưa
  được cài (`vitest` không có trên PATH). Đây là việc setup môi trường, không
  được tính là bằng chứng frontend fail.
