# Runbook vận hành self-host

Runbook này dành cho instance một VPS dùng Docker Compose. Mục tiêu là giúp
operator biết phải kiểm tra gì, backup gì và phản ứng thế nào khi có sự cố.

## Trách nhiệm

Operator của tổ chức chịu trách nhiệm VPS, DNS, TLS, firewall, cập nhật, backup,
khóa secret, tài khoản quản trị và privacy policy. Publisher chỉ chịu trách
nhiệm dịch vụ trung tâm nào được ký hợp đồng rõ ràng, ví dụ download portal hoặc
push relay; dữ liệu chat không tự động được sao lưu lên publisher.

## Checklist sau khi cài

- [ ] `sh deploy/self-hosted/check.sh` hoàn tất không lỗi.
- [ ] `https://<domain>/ready` trả `200` từ mạng ngoài VPS.
- [ ] PostgreSQL, Redis và RabbitMQ không có port public.
- [ ] Caddy cấp certificate đúng domain; HTTP tự chuyển sang HTTPS.
- [ ] Owner đầu tiên đăng nhập được và đã đổi registration thành
      `invite_only`/`closed` nếu không cần đăng ký công khai.
- [ ] Gửi message, upload/download file và gọi thử qua mạng 4G khác VPS.
- [ ] Chọn rõ mode push; không để cấu hình relay dở dang.
- [ ] Backup đầu tiên đã được copy sang nơi khác VPS và mã hóa.
- [ ] Alert disk, RAM, CPU, certificate, `/ready` và container restart đã bật.
- [ ] Privacy policy, terms/support và URL account deletion đã public trước khi
      phân phối app cho người dùng bên ngoài tổ chức.

## Nhịp vận hành

### Hàng ngày

```sh
sh deploy/self-hosted/check.sh
cd deploy/self-hosted
docker compose --env-file .env -f compose.yml logs --since=24h api worker caddy
docker system df
df -h
```

Kiểm tra container restart, lỗi migration, job `dead`, `401/403` từ push relay,
certificate và tốc độ tăng storage.

### Hàng tuần

```sh
cd deploy/self-hosted
./backup.sh backup
./backup.sh verify
```

- kiểm tra scheduler có snapshot mới đúng RPO trong bucket ngoài VPS;
- kiểm tra Restic repository, pack subset, manifest và checksum;
- rà user admin, session bất thường và audit log;
- cập nhật bản vá OS/Docker theo maintenance window.

### Hàng tháng

- restore backup mới nhất vào một VPS test tách biệt;
- đo RPO/RTO thực tế và ghi kết quả;
- kiểm tra dung lượng database/storage, retention và log;
- rà quyền SSH, token relay, API token, webhook và OIDC client;
- kiểm tra luồng xóa tài khoản cùng thời hạn hết hiệu lực trong backup;
- thử notification background và cuộc gọi trên thiết bị Android/iOS thật.

## Backup và restore

`backup.sh` tạo PostgreSQL custom dump và snapshot file/object storage trong một
repository Restic S3-compatible mã hóa phía client. Bundle có manifest phiên bản,
inventory và SHA-256 từng file. `.env` mặc định không được đưa vào snapshot; nếu
operator chủ động bật, restore vẫn không tự áp dụng file cấu hình này.

Restore production yêu cầu snapshot ID hexadecimal cụ thể, `--apply` và chuỗi
xác nhận chứa chính ID đó:

```sh
cd deploy/self-hosted
./restore.sh 0123abcd --apply --confirm RESTORE:0123abcd
```

Wrapper stage/verify target trước maintenance, tạo safety snapshot mới khi mọi
writer đã dừng, giữ cây local storage cũ đến sau migration/health check và cố
rollback nếu apply thất bại. Sau restore vẫn phải chạy `check.sh`, đăng nhập, mở
message/file cũ, tạo message/file mới và kiểm tra worker. Một snapshot chưa từng
restore thành công trên host cô lập chưa được xem là backup đáng tin cậy.

Thiết lập bucket, retention, lịch, verify subset, restore host mới và xử lý
rollback nằm trong [runbook backup off-site](offsite-backup-restore.md).

## Update an toàn

1. Đọc release notes và migration.
2. Xác nhận backup ngoài VPS vừa hoàn tất.
3. Chọn maintenance window và thông báo người dùng.
4. Chạy:

```sh
sh deploy/self-hosted/update.sh
```

5. Chạy smoke test web, admin, mobile discovery, message, file, call và push.
6. Theo dõi log ít nhất 15 phút.

Không update khi repository có tracked changes chưa commit. Không rollback code
qua migration đã chạy nếu release không công bố down-migration an toàn; ưu tiên
restore theo runbook của release.

## Xoay secret

| Secret | Ảnh hưởng | Cách xoay |
| --- | --- | --- |
| `PUSH_RELAY_TOKEN` | push nền | cửa sổ bảo trì ngắn: cập nhật/restart relay rồi customer worker; cấu hình hiện chỉ nhận một token/UUID, không có dual-token overlap |
| Firebase/APNs key | push app custom | tạo key mới, cập nhật worker, test thiết bị thật, thu hồi key cũ |
| `TURN_SHARED_SECRET` | cuộc gọi đang/chuẩn bị kết nối | cập nhật API và coturn cùng lúc, restart hai service |
| JWT access/refresh | mọi session | maintenance window; người dùng có thể phải đăng nhập lại |
| webhook/OIDC/Bot AI | integration tương ứng | phối hợp phía đối tác, test rồi thu hồi key cũ |
| database/Redis/RabbitMQ password | kết nối nội bộ | cập nhật service phụ thuộc theo thứ tự, tránh restart nửa chừng |

Không dùng cùng secret giữa staging và production hoặc giữa hai customer.

## Chẩn đoán nhanh

### `/ready` lỗi

```sh
cd deploy/self-hosted
docker compose --env-file .env -f compose.yml ps
docker compose --env-file .env -f compose.yml logs --tail=200 api postgres redis rabbitmq
```

Kiểm tra disk full, healthcheck, migration và password/URL trong `.env`. Không
xóa volume để “thử sửa”.

### DNS/TLS lỗi

- so sánh IPv4 public với bản ghi `A`;
- chắc chắn port `80/443` tới đúng VPS và không có reverse proxy khác chiếm port;
- xem log Caddy và giới hạn cấp certificate;
- chỉ dùng `--skip-dns-check` khi bản ghi public thực sự đúng nhưng resolver local
  chưa cập nhật.

### Push chậm hoặc không đến

1. Chạy `check.sh` để xác nhận mode.
2. Xem `worker` log; `401/403` là credential, `429/5xx` là provider/relay tạm lỗi.
3. Kiểm tra queue `failed/dead`, permission của thiết bị và app Firebase config.
4. Test bằng thiết bị thật ở background/force-stop.
5. Kiểm tra sync catch-up; push chỉ là tín hiệu, không phải nguồn dữ liệu duy nhất.

Xem [runbook push chi tiết](push-notifications.md).

### Disk gần đầy

- dừng upload lớn hoặc đặt instance read-only theo quy trình nội bộ;
- xác định dung lượng PostgreSQL, Docker log, image cũ, backup local và storage;
- copy rồi xóa backup hết retention theo chính sách;
- mở rộng disk trước khi PostgreSQL hết chỗ;
- không chạy lệnh prune/xóa volume không xác định.

### Nghi lộ secret

1. Cô lập quyền SSH và lưu bằng chứng/audit log.
2. Xác định đúng secret, phạm vi và thời điểm.
3. Thu hồi/rotate từ bên phát hành trước khi công bố token mới.
4. Revoke session/API token/device nếu liên quan.
5. Kiểm tra truy cập dữ liệu và thực hiện nghĩa vụ thông báo sự cố.
6. Ghi postmortem và ngăn secret xuất hiện lại trong log/backup/ticket.

## Xóa tài khoản và dữ liệu

Release có tạo tài khoản phải cung cấp cả:

- luồng trong app gọi `DELETE /api/v1/users/me` sau bước xác nhận rõ hậu quả;
- URL web công khai như `/account-deletion` cho yêu cầu khi không còn cài app.

Runbook nội bộ phải xác định ownership transfer, dữ liệu message cần giữ theo
nghĩa vụ tổ chức, dữ liệu cá nhân được ẩn/xóa, token/session/device bị revoke và
khi nào bản ghi sẽ biến mất khỏi backup. Không hứa “xóa ngay mọi bản sao” nếu
backup immutable vẫn còn trong retention.

## Thông tin cần ghi cho mỗi instance

Lưu trong password manager/CMDB, không lưu trong repository:

- domain, IP, nhà cung cấp VPS và owner vận hành;
- version/tag đang chạy và ngày update gần nhất;
- vị trí backup, retention, kết quả restore drill, RPO/RTO;
- mode push và ngày xoay credential gần nhất (không ghi giá trị secret);
- contact bảo mật, support, privacy và account deletion;
- maintenance window và escalation path.
