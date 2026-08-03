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
