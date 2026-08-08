# Production go/no-go checklist

Checklist này áp dụng trước khi mở instance cho người dùng thật hoặc gửi mobile
app lên Google Play/App Store. Mỗi mục chưa đạt phải có owner và ngày xử lý; không
đánh dấu đạt chỉ vì chức năng “chạy được trên máy dev”.

## Hạ tầng self-host

- [ ] Domain, DNS, TLS và firewall đã kiểm tra từ mạng ngoài.
- [ ] Chỉ Caddy/coturn public; database, Redis, RabbitMQ không public.
- [ ] `.env` quyền `0600`, secret ngẫu nhiên, không có `CHANGE_ME`.
- [ ] `RATE_LIMIT_ENABLED=true`, CORS chỉ chứa origin cần thiết.
- [ ] `/ready`, container restart, CPU/RAM/disk và certificate có alert.
- [ ] Backup mã hóa ngoài VPS; restore drill thành công và có RPO/RTO.
- [ ] Bucket off-site tách primary storage, versioning/immutability và IAM tối thiểu đã bật.
- [ ] Restic password có escrow độc lập; `verify`/checksum và tuổi snapshot có alert.
- [ ] Container DR chỉ load `offsite-backup.env`; `.env` không mount trừ khi operator opt-in có chủ đích.
- [ ] Push relay/migrator dùng allowlist Compose; không thêm lại `env_file: .env` cho hai role này.
- [ ] Restore drill dùng snapshot ID cụ thể, safety snapshot và rollback đã được ghi nhận.
- [ ] Update/rollback/incident owner và maintenance window đã xác định.

## Notification và realtime

- [ ] Đã chọn relay, direct hoặc disabled; không có cấu hình một nửa.
- [ ] Relay token là duy nhất theo instance và relay có rate limit/audit/revoke.
- [ ] Firebase/APNs config khớp package/bundle/provisioning của binary.
- [ ] Android và iPhone thật nhận foreground, background, force-stop/call invite.
- [ ] Logout/xóa tài khoản revoke device; token hỏng bị tự revoke.
- [ ] Queue không có job `dead` bất thường; retry không tạo notification lặp.
- [ ] Catch-up sau offline hoạt động dù push bị chặn.
- [ ] Nếu bật Web Push: VAPID key đã backup, private key chỉ cấp cho worker,
  browser opt-in thủ công và đã test nhận/click/revoke khi đóng tab.

## Kiểm thử source

```sh
cd backend
go test ./...

cd ../frontend
npm ci
npm run typecheck
npm run lint
npm run test:unit
npm run build:web
npm run build:admin

cd ../deploy/self-hosted
docker compose --env-file .env.example -f compose.yml config >/dev/null
```

- [ ] OpenAPI contract test, migration up/down policy và smoke test đều đạt.
- [ ] Không có secret/private key/keystore/provisioning profile trong git hoặc artifact public.
- [ ] Dependency/security scan không còn finding critical/high chưa xử lý.
- [ ] Release build dùng version/build number mới và artifact có checksum/signature.

## Mobile store

- [ ] Application ID/bundle ID, icon, app name, signing và store owner là production.
- [ ] Android target API, AAB release signing, Play App Signing và Data safety đã hoàn tất.
- [ ] iOS capabilities, APNs/PushKit entitlement, privacy manifest và App Privacy đã hoàn tất.
- [ ] Permission camera/micro/photo/notification có purpose string đúng chức năng.
- [ ] Login, đăng ký, SSO, offline, file, call và deep link test trên device matrix.
- [ ] Reviewer có tài khoản demo/hướng dẫn và backend review vẫn hoạt động.
- [ ] Privacy policy, terms, support URL và contact là URL public ổn định.
- [ ] Nếu app tạo account: có self-delete bằng `DELETE /api/v1/users/me`, xác nhận
      trong app và URL công khai `/account-deletion` cho yêu cầu ngoài app.
- [ ] Chính sách deletion nói rõ ownership transfer, retention pháp lý và backup expiry.
- [ ] Hai well-known URL App/Universal Links trả HTTP 200 trực tiếp, không redirect:
      `/.well-known/assetlinks.json` và `/.well-known/apple-app-site-association`.
- [ ] Đã test report message/user, block/unblock, blocked DM/call và hàng đợi moderator bằng hai tài khoản thật.
- [ ] Owner/admin có `moderation.manage`; member thường không đọc hoặc xử lý được report.
- [ ] Moderator xóa được nội dung vi phạm qua `message.manage`, đóng report với resolution note,
      và alert hàng đợi theo SLO triage 4h/24h, đóng trong 72h.
- [ ] Store disclosure nói đúng hiện trạng: human moderation + removal/blocking; backend chưa có
      automated objectionable-content filter.
- [ ] Prometheus/Alertmanager loads all three moderation SLO rules, a real receiver
      passes an alert drill, and `MODERATION_EVIDENCE_RETENTION_DAYS=365` matches
      the public portal policy while the worker retention task is healthy.
- [ ] Đăng ký mobile gửi đúng phiên bản Terms/AUP và Privacy từ `/api/v1/auth/legal-documents`;
      bản ghi xuất hiện trong `user_legal_acceptances`.
- [ ] `TERMS_VERSION` và `PRIVACY_POLICY_VERSION` khớp policy version portal; Google JIT signup
      thiếu consent trả `LEGAL_ACCEPTANCE_REQUIRED`, còn Google user hiện hữu vẫn đăng nhập được.
- [ ] Với user thuộc nhiều workspace, GET/POST legal acceptance gửi explicit `workspace_id`,
      response echo đúng workspace, và workspace khác zone/non-member bị từ chối 403.
- [ ] Worker hẹn giờ đã test: suspend/delete sender, revoke `message.send`, đổi policy version,
      hoặc block DM trước giờ phát đều chuyển job sang `cancelled`; lỗi DB tạm thời vẫn retry.
- [ ] Khách phòng công khai phải check Terms/AUP + Privacy hiện hành trước khi join;
      migration 40 đã revoke grant cũ và bản ghi guest mới có version/timestamp/IP/User-Agent thật.
- [ ] OIDC user mới được pre-provision/đăng ký chuẩn trước; `jit_provisioning=true` vẫn fail-closed
      với `OIDC_JIT_LEGAL_ACCEPTANCE_REQUIRED` cho đến khi redirect flow lưu được versioned consent.

## Quyền riêng tư và pháp lý

- [ ] Inventory dữ liệu khớp thực tế: account, message, file, device/push token,
      audit, IP/log, crash/analytics và bên xử lý thứ ba.
- [ ] Privacy policy nêu push relay/provider, mục đích, retention và contact.
- [ ] Có cơ chế export/xóa dữ liệu và SLA xử lý yêu cầu.
- [ ] Log/metrics không chứa access token, raw push token hoặc nội dung nhạy cảm.
- [ ] Điều khoản/DPA/consent đáp ứng thị trường mục tiêu; legal owner đã duyệt.

## Go/no-go

Chỉ **GO** khi:

1. không còn blocker bảo mật, mất dữ liệu, auth, privacy hoặc account deletion;
2. backup đã restore thật;
3. push/realtime đã test trên thiết bị thật theo mode phát hành;
4. build reproducible, ký đúng và monitoring trực ca;
5. mọi giới hạn còn lại được mô tả trung thực cho người dùng/reviewer.
