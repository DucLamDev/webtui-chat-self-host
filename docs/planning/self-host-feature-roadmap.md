# Roadmap chức năng phù hợp cho VPSTTT Chat self-host

Đây là danh sách đề xuất, không phải cam kết release. Ưu tiên dựa trên bốn tiêu
chí: an toàn dữ liệu, khả năng vận hành, trải nghiệm đa nền tảng và chi phí duy
trì cho tổ chức tự host.

Luồng account deletion cơ bản đã có trong source: API hard-delete, chuyển quyền
atomic, UI web/mobile và trang public. Phần còn lại trước release là kiểm thử DB
production, xác nhận retention/backup với operator và duyệt pháp lý/store form;
đó là release gate, không phải một capability còn thiếu.

## Đã tích hợp vào source

- push relay publisher độc lập với PostgreSQL queue, token từng instance,
  idempotency, rate limit, retry/lease và hợp đồng OpenAPI;
- Web Push/VAPID từng instance với subscription lifecycle, consent UI, service
  worker, mã hóa payload và tự thu hồi endpoint `404/410`;
- Admin Push dashboard cho queue, delivery rate, provider, dead-letter và replay
  có permission/audit;
- backup Restic off-site S3-compatible, mã hóa, retention, checksum, scheduler,
  staged restore và rollback tự động khi health check thất bại;
- OpenTelemetry trace, Prometheus histogram p95/p99, Grafana dashboard và
  Alertmanager rules cho API, push và backup;
- offline message outbox và delta sync có idempotency, lease đa tab, backoff,
  server timestamp, tombstone và durable cursor.

Các mục trên vẫn cần credential thật và release drill trên hạ tầng đích. Source
hoàn thành không thay thế cross-browser/device test, restore drill, DPA/privacy
review hay cấu hình nơi nhận alert của operator.

## P0 — release gate còn lại

| Chức năng | Giá trị | Điều kiện hoàn thành |
| --- | --- | --- |
| Provider/relay certification | xác nhận push thật trước khi phát hành app official | FCM/APNs sandbox + production device test, token revoke drill, privacy/DPA review |
| Browser/backup/alert drill | xác nhận các profile opt-in trên hạ tầng thật | Chrome/Firefox/Safari test, restore sang host cô lập, webhook/email alert test |
| Release/security gate | giảm build lỗi và lộ secret | CI test/build, SBOM, dependency/container scan, secret scan, signed artifact, migration smoke |

## P1 — tăng giá trị cho doanh nghiệp

| Nhóm | Chức năng đề xuất | Ghi chú kiến trúc |
| --- | --- | --- |
| Tìm kiếm | PostgreSQL full-text trước, OpenSearch khi dữ liệu lớn | phân quyền theo workspace/channel, index async, retention-aware |
| File | S3/MinIO production, multipart resume, antivirus/quarantine, preview | signed URL ngắn hạn, quota, MIME sniffing, lifecycle policy |
| Secret/data hardening | mã hóa push token ở cấp ứng dụng, Docker secrets, key rotation | encryption key tách database/backup, versioned envelope, zero-downtime re-encrypt |
| Identity | OIDC hoàn chỉnh, SAML/SCIM, group sync, MFA/passkey | JIT provisioning, deprovision, break-glass admin, audit |
| Quản trị dữ liệu | retention theo workspace/channel, export, legal hold | tách “xóa người dùng” khỏi nghĩa vụ giữ record doanh nghiệp |
| Mobile offline nâng cao | attachment resume, background transfer và quota | mở rộng nền outbox/idempotency/cursor đã có; không giữ `File` tùy tiện trong bộ nhớ |
| Desktop | background lifecycle, launch-on-login policy, update rings | không hứa notification khi process đã thoát nếu chưa có OS push |
| Cuộc gọi nhóm | SFU (LiveKit/Janus/Jitsi self-host), screen share, recording | consent, quota, TURN load, recording retention |
| Moderation | report, role moderator, spam/rate policy, attachment policy | audit có lý do, tránh quyền admin quá rộng |
| Workflow | approval, form, scheduled automation, webhook template | sandbox runner, allowlist outbound, secret vault, retry/idempotency |

## P2 — mở rộng quy mô và khác biệt sản phẩm

- High availability: API/worker nhiều node, PostgreSQL managed/replica, Redis và
  RabbitMQ HA, object storage, zero-downtime migration, disaster recovery đa vùng.
- Federation có allowlist giữa các tổ chức: identity mapping, chống spam, policy
  trust, moderation và data residency phải thiết kế trước protocol.
- Mã hóa đầu cuối tùy chọn: quản lý key đa thiết bị, backup/recovery, search,
  moderation, bot và legal hold là trade-off bắt buộc phải công khai.
- AI nội bộ có RAG: connector tài liệu, permission-aware retrieval, citation,
  redaction PII, budget/quota và evaluation; mặc định ưu tiên model self-host.
- Bot/app marketplace riêng của tổ chức: manifest permission, signing, review,
  sandbox, version pin và kill switch.
- DLP/compliance: classification label, watermark, download restriction, audit
  export, eDiscovery và policy theo vùng dữ liệu.
- Analytics riêng tư: adoption, delivery latency, call quality và storage growth;
  aggregate mặc định, opt-in cho telemetry ra ngoài instance.

## Thứ tự triển khai khuyến nghị

1. Chạy provider/browser/restore/alert drill và hoàn thiện release/security gate.
2. Search cùng S3/antivirus và mã hóa token/secret rotation.
3. Identity lifecycle (MFA/SCIM) và retention/export/legal hold.
4. Attachment background sync và desktop update rings.
5. SFU/group call, workflow và các hạng mục P2 theo nhu cầu khách hàng thật.

## Nguyên tắc nhận một chức năng vào roadmap

- Có user story và operator story; tự host không chỉ là UI cho end user.
- Có threat model, dữ liệu thu thập/retention và cách backup/restore.
- Có permission/audit, quota/rate limit và hành vi khi dependency lỗi.
- Có API contract, migration forward/backward và test đa nền tảng cần thiết.
- Có metric thành công: p95 latency, delivery rate, crash-free session, RPO/RTO
  hoặc thời gian hoàn thành tác vụ; không dùng “cảm giác mượt” làm tiêu chí duy nhất.
- Không quảng bá capability trước khi đường happy path, failure path, vận hành và
  tài liệu đều hoàn thành.
