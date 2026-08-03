# Runbook backup off-site S3 và restore

Runbook này áp dụng cho stack Docker Compose trong `deploy/self-hosted`. Mục
tiêu là có một bản disaster-recovery nằm ngoài VPS, mã hóa phía client và có
thể kiểm tra/khôi phục bằng quy trình lặp lại được. Tính năng **mặc định tắt**;
quickstart không build hoặc chạy container backup khi chưa bật profile.

## Phạm vi và bảo đảm

Mỗi snapshot chứa:

- `database.dump`: `pg_dump` custom format, không lưu owner/ACL;
- `storage/`: bản staging của toàn bộ file local hoặc object trong bucket
  MinIO/S3 chính;
- `config/compose.yml` và `config/Caddyfile` nếu có;
- `.env` chỉ khi operator chủ động bật override mount cho đúng lệnh backup như
  hướng dẫn bên dưới;
- `manifest.json` có schema version, bundle ID, phiên bản PostgreSQL/app và
  nguồn storage;
- `checksums.json` có kích thước và SHA-256 của từng file.

Restic cung cấp mã hóa xác thực phía client, nén, deduplication, kiểm tra pack,
snapshot và multipart upload cho S3-compatible. Restore còn kiểm tra lại
manifest, inventory, SHA-256, định dạng archive PostgreSQL và từ chối symlink,
special file, đường dẫn tuyệt đối hoặc thành phần `..`.

`pg_dump` cho một snapshot PostgreSQL nhất quán dù hệ thống đang có ghi. Tuy
nhiên database và object storage không có một transaction chung. Dùng
`./backup.sh backup --maintenance` khi cần một recovery point nghiêm ngặt giữa
DB và file; lệnh dừng `api`/`worker` trong lúc staging rồi chỉ bật lại những
service trước đó đang chạy. Scheduler tạo online backup để giảm downtime.

Không nằm trong snapshot:

- Redis cache/presence và RabbitMQ queue; đây không phải nguồn dữ liệu chuẩn;
- PostgreSQL cluster roles/global objects; Compose tạo user/database trước khi
  restore và dump được nạp với `--no-owner`;
- Ollama model, Caddy certificate cache và Coturn state;
- point-in-time recovery/WAL. Nếu RPO nhỏ hơn chu kỳ snapshot, cần thêm WAL
  archive hoặc managed PostgreSQL có PITR.

## Kiến trúc và ranh giới quyền

Compose có hai profile riêng:

| Service | Profile | Mount storage | Mục đích |
| --- | --- | --- | --- |
| `offsite-backup` | `backup` | read-only | plan/init/backup/list/verify/prune/schedule |
| `offsite-restore` | `restore` | read-write | chỉ được wrapper restore gọi sau maintenance và xác nhận |

API/worker không nhận credential của repository off-site. Ngược lại, container
backup/restore cũng không nạp toàn bộ `.env` của ứng dụng: Compose chỉ map rõ
`DATABASE_URL`, provider/credential storage chính và metadata instance cần cho
backup. Chúng chỉ mount read-only từng file `compose.yml`, `Caddyfile`, không
mount cả thư mục deploy và không mount Docker socket. Cả hai chỉ dùng
`backup_data` làm staging; cần theo dõi dung lượng volume này.

## Chuẩn bị S3 hoặc MinIO

1. Tạo bucket riêng, tốt nhất ở account/provider/region khác VPS và khác bucket
   file chính. Tool từ chối dùng đúng cùng endpoint + bucket với primary storage
   để tránh snapshot tự chứa repository của chính nó.
2. Bật bucket versioning; nếu có Object Lock/immutability, đặt retention theo
   chính sách tổ chức. Lưu ý `restic forget --prune` cần quyền xóa; Object Lock
   có thể làm prune thất bại cho đến khi hết thời gian khóa.
3. Cấp credential riêng với phạm vi bucket/prefix. Runtime cần list/get/put và
   delete để retention hoạt động; credential khởi tạo còn có thể cần tạo bucket
   nếu bucket chưa tồn tại. Production nên tạo bucket trước và không cấp quyền
   tạo bucket.
4. Tạo password Restic ngẫu nhiên tối thiểu 32 byte và lưu thêm một bản trong
   password manager/offline escrow. Mất password đồng nghĩa mất khả năng đọc
   backup. Không dùng cùng password S3, database hoặc JWT.
5. Sao chép `deploy/self-hosted/offsite-backup.env.example` thành
   `deploy/self-hosted/offsite-backup.env`, đặt quyền `0600`, rồi chỉnh cấu hình
   trong file riêng này. Không đặt credential off-site trong `.env`: chỉ hai
   service backup/restore nhận file secret này, API và worker không nhận nó.
6. Không commit `.env`, `offsite-backup.env`, password file hay access key.

Ví dụ AWS S3:

```dotenv
OFFSITE_BACKUP_ENABLED=true
OFFSITE_S3_ENDPOINT=s3.ap-southeast-1.amazonaws.com
OFFSITE_S3_REGION=ap-southeast-1
OFFSITE_S3_BUCKET=company-webtui-backups
OFFSITE_S3_PREFIX=production/chat.example.com
OFFSITE_S3_BUCKET_LOOKUP=auto
OFFSITE_S3_ACCESS_KEY_ID=...
OFFSITE_S3_SECRET_ACCESS_KEY=...
OFFSITE_RESTIC_PASSWORD=...
```

Ví dụ MinIO S3-compatible:

```dotenv
OFFSITE_BACKUP_ENABLED=true
OFFSITE_S3_ENDPOINT=https://backup-minio.example.net:9000
OFFSITE_S3_REGION=us-east-1
OFFSITE_S3_BUCKET=webtui-disaster-recovery
OFFSITE_S3_PREFIX=production/chat.example.com
OFFSITE_S3_BUCKET_LOOKUP=path
OFFSITE_S3_ACCESS_KEY_ID=...
OFFSITE_S3_SECRET_ACCESS_KEY=...
OFFSITE_RESTIC_PASSWORD=...
```

Ưu tiên `OFFSITE_RESTIC_PASSWORD_FILE` trỏ tới secret file được mount bằng một
Compose override chỉ tồn tại trên server. `OFFSITE_RESTIC_PASSWORD` là fallback
thuận tiện cho instance nhỏ nhưng xuất hiện trong environment của container.

Nếu primary storage cũng là MinIO/S3, tool dùng `rclone` tải object vào staging
trước khi Restic upload sang bucket độc lập. `OFFSITE_SOURCE_S3_PREFIX` giới hạn
prefix nguồn; để trống nghĩa là toàn bucket chính.

## Khởi tạo và backup đầu tiên

Tại `deploy/self-hosted`:

```sh
cp offsite-backup.env.example offsite-backup.env
chmod 600 offsite-backup.env
# Chỉnh offsite-backup.env trước khi tiếp tục.
./backup.sh plan
./backup.sh backup --dry-run
./backup.sh init
./backup.sh backup --maintenance
./backup.sh list
```

`plan` chỉ in cấu hình đã che secret. `backup --dry-run` có thể đọc kích thước
DB/storage để kiểm tra staging nhưng không tạo dump, repository hay object mới.
`init` là lệnh riêng, không tự chạy khi backup, để một endpoint/prefix gõ nhầm
không âm thầm tạo repository mới.

### Opt-in đưa `.env` vào repository mã hóa

Mặc định hai service DR **không thể đọc `.env` qua filesystem**, và
`OFFSITE_BACKUP_INCLUDE_INSTANCE_ENV` không nằm trong file cấu hình mặc định.
Chỉ khi chính sách recovery thực sự cần backup secret, bật override cho đúng
lệnh cần chạy:

```sh
cd deploy/self-hosted
OFFSITE_BACKUP_INCLUDE_INSTANCE_ENV=true ./backup.sh backup --maintenance
```

Script chỉ lúc đó mới thêm `compose.include-instance-env.yml`, mount `.env`
read-only vào container backup và ghi nó thành `config/instance.env` bên trong
repository đã mã hóa. Với scheduler, dùng cùng biến khi tạo/recreate service:

```sh
OFFSITE_BACKUP_INCLUDE_INSTANCE_ENV=true ./backup.sh schedule-start
```

Không thêm biến này vào `offsite-backup.env` và không sửa Compose base để mount
`.env`; làm vậy sẽ biến opt-in thành quyền truy cập thường trực. Restore vẫn
không bao giờ tự áp dụng `instance.env`. Nếu cần safety snapshot trong lúc
restore cũng chứa `.env`, truyền biến explicit cho lệnh restore tương ứng.

Backup kiểm tra dung lượng trống theo kích thước DB + storage, headroom và
`OFFSITE_BACKUP_MIN_FREE_BYTES`. Với dữ liệu lớn, dành staging ít nhất tổng dung
lượng logic DB + object, cộng headroom. Restic chỉ upload sau khi bundle local
đã hoàn tất và tự kiểm tra.

## Scheduler và retention

Mặc định:

```dotenv
OFFSITE_BACKUP_INTERVAL_SECONDS=86400
OFFSITE_BACKUP_RUN_ON_START=false
OFFSITE_BACKUP_KEEP_DAILY=7
OFFSITE_BACKUP_KEEP_WEEKLY=4
OFFSITE_BACKUP_KEEP_MONTHLY=12
OFFSITE_BACKUP_KEEP_YEARLY=3
OFFSITE_BACKUP_RETENTION_ENABLED=true
```

Bật scheduler opt-in:

```sh
./backup.sh schedule-start
./backup.sh schedule-logs
./backup.sh schedule-stop
```

Scheduler chờ đủ interval trước lần đầu trừ khi
`OFFSITE_BACKUP_RUN_ON_START=true`. Một file lock ngăn hai tác vụ cùng node chạy
đồng thời; Restic lock tiếp tục bảo vệ repository khi có nhiều host.

Mỗi lần backup thật (không phải dry-run) ghi best-effort một dòng `backup_runs`
với `backup_job_id=NULL`, `backup_type=full` và trạng thái
`running -> success/failed`. Snapshot URL, logical byte size và root manifest
checksum được ghi khi có, nên metric/alert backup dùng chung phản ánh cả scheduler
off-site. Lỗi ghi telemetry chỉ được log cảnh báo và không che kết quả pg_dump/S3.

Xem trước và áp dụng retention thủ công:

```sh
./backup.sh prune --dry-run
./backup.sh prune
```

Không đặt lifecycle của bucket xóa object Restic tùy ý. Hãy để Restic quản lý
snapshot/pack; lifecycle chỉ nên xử lý version cũ theo chính sách đã kiểm thử.

## List và verify

```sh
./backup.sh list
./backup.sh list --json
./backup.sh verify
./backup.sh verify 0123abcd
```

- `verify` không có ID chạy `restic check` và đọc ngẫu nhiên tỷ lệ pack theo
  `OFFSITE_BACKUP_VERIFY_READ_DATA_SUBSET` (mặc định `5%`);
- `verify ID` còn restore snapshot vào staging, kiểm tra archive và SHA-256 từng
  file rồi xóa staging test;
- chạy verify metadata hằng tuần, full/subset luân phiên theo ngân sách egress,
  và restore drill sang host cô lập ít nhất hằng tháng.

## Restore production có guardrail

Không restore trực tiếp chỉ vì `list` nhìn thấy snapshot. Trước maintenance:

1. ghi ticket/owner, snapshot ID, RPO dự kiến và rollback owner;
2. xác nhận bucket/password đọc được và staging đủ dung lượng;
3. thử snapshot trên host cô lập nếu thời gian cho phép;
4. thông báo maintenance và ngăn deploy/update song song.

Lệnh production yêu cầu cả `--apply` và chuỗi xác nhận chứa đúng snapshot ID:

```sh
./restore.sh 0123abcd \
  --apply \
  --confirm RESTORE:0123abcd
```

Không chấp nhận alias `latest`, path hay URL. Wrapper tự động:

1. download target vào thư mục staging mới;
2. kiểm tra Restic + manifest + checksum + `pg_restore --list`;
3. dừng các service ứng dụng trước đó đang chạy và scheduler;
4. tạo một safety snapshot mới trong maintenance mode, sau đó stage/verify nó;
5. sinh confirmation nội bộ riêng cho DB và storage;
6. drop/recreate database rồi `pg_restore --exit-on-error`;
7. thay storage; với local storage, dữ liệu cũ được giữ trong cùng volume;
8. chạy migration forward của version code hiện tại;
9. bật lại service, chờ `/ready`, rồi mới xóa cây local cũ;
10. xóa staging nếu thành công. Dùng `--keep-staging` để giữ phục vụ điều tra.

Nếu một bước sau khi bắt đầu ghi thất bại, wrapper cố phục hồi database từ
safety snapshot và rollback local storage (hoặc sync safety snapshot về primary
S3), rồi mới bật lại service. Nếu rollback không hoàn tất, service được giữ ở
trạng thái stopped và staging không bị xóa. Không xóa volume hoặc chạy lại lệnh
ngẫu nhiên; đọc log và dùng safety snapshot được in ra cuối/ở stderr.

Bundle có thể chứa `instance.env` nhưng restore **không bao giờ** ghi đè `.env`,
`compose.yml` hoặc `Caddyfile`. Khi dựng host mới, so sánh thủ công, lấy secret từ
password manager rồi chạy `docker compose config` trước khi apply.

## Restore sang host mới

1. Checkout đúng release hoặc release mới hơn đã hỗ trợ migration tương ứng.
2. Tạo `.env` mới có domain/database credentials và `offsite-backup.env` riêng có
   credential/password Restic.
3. Khởi động `postgres` và chạy `./backup.sh list` để kiểm tra repository.
4. Dùng restore command với snapshot ID cụ thể.
5. Nếu đổi domain, cập nhật `.env` thủ công sau restore rồi rebuild web/admin vì
   public URL được đưa vào build frontend.
6. Chạy `check.sh`, đăng nhập, đọc file/message cũ, upload/gửi message mới và
   kiểm tra worker/push/call.

## RPO, RTO và giám sát

- RPO snapshot mặc định: tối đa 24 giờ; điều chỉnh interval theo dữ liệu và chi
  phí. Muốn RPO phút cần PITR/WAL, không chỉ tăng tần suất full snapshot.
- RTO phụ thuộc download + checksum + pg_restore + copy object. Đo trên dữ liệu
  gần kích thước production, không ước lượng từ database dev.
- Alert khi scheduler không có snapshot mới quá `interval + grace`, container
  restart liên tục, `prune/check` lỗi, staging gần đầy hoặc bucket tăng bất thường.
- Ghi snapshot ID, thời gian, kích thước, kết quả verify/restore drill và người
  xác nhận vào CMDB/ticket; không ghi secret.

## Giới hạn kiểm thử trong repository

CI kiểm tra Python unit test, path traversal/checksum rules, shell syntax và
Compose config. CI không có credential AWS/MinIO production, không đo multipart
trên object lớn và không thực hiện destructive restore vào PostgreSQL thật. Trước
go-live, operator phải chạy integration drill với bucket test, PostgreSQL test và
storage có dữ liệu đại diện; một snapshot chưa từng restore thành công chưa được
xem là backup đáng tin cậy.
