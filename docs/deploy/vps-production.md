# Deploy production lên VPS

Tài liệu này mô tả luồng deploy backend WebTui Chat lên VPS bằng Docker Compose và GitHub Actions.

Nếu cần checklist từng bước từ tạo SSH key, cấu hình GitHub Actions, cài Docker trên VPS, tạo `.env`, chạy deploy và test, đọc thêm [CI/CD từng bước với GitHub Actions và Docker Compose](cicd.md).

## Thông tin triển khai

- API backend: `https://chat.vpsttt.com`
- Frontend: `https://chat.vpsttt.com`
- VPS public IP: lưu trong `vps-info.md`
- User SSH: lưu trong `vps-info.md`
- Runtime backend: API + worker + PostgreSQL + Redis + Nginx
- Queue production: CloudAMQP qua `amqps`
- File storage production: MinIO/S3 ngoài VPS

Không commit mật khẩu VPS, mật khẩu RabbitMQ, secret JWT, secret PostgreSQL hoặc secret MinIO vào repo.

## Cấu hình RabbitMQ CloudAMQP

CloudAMQP trên ảnh dùng:

- Host cân bằng tải: `fuji.lmq.cloudamqp.com`
- Vhost: `btrvptkc`
- User: `btrvptkc`
- Port TLS: `5671`
- Scheme: `amqps`

Trong `.env` production trên VPS, cấu hình:

```env
RABBITMQ_ENABLED=true
RABBITMQ_URL=amqps://btrvptkc:THAY_BANG_MAT_KHAU_CLOUDAMQP@fuji.lmq.cloudamqp.com/btrvptkc
```

Không dùng container RabbitMQ trong `compose.prod.yml` vì production đang dùng RabbitMQ managed.

## Chuẩn bị VPS lần đầu

Đăng nhập VPS bằng user trong `vps-info.md`, sau đó cài Docker Engine và Docker Compose plugin.

Tạo thư mục deploy:

```bash
sudo mkdir -p /opt/webtui-chat
sudo chown -R "$USER":"$USER" /opt/webtui-chat
cd /opt/webtui-chat
```

Copy thư mục `deploy` từ repo lên VPS hoặc chạy workflow deploy một lần để workflow đồng bộ thư mục này.

Tạo file môi trường production:

```bash
cp deploy/.env.example .env
nano .env
```

Cần thay toàn bộ giá trị bắt đầu bằng `THAY_BANG_...`, đặc biệt:

- `POSTGRES_PASSWORD`
- `DATABASE_URL`
- `RABBITMQ_URL`
- `S3_SECRET_ACCESS_KEY`
- `JWT_ACCESS_SECRET`
- `JWT_REFRESH_SECRET`
- `WEBHOOK_SIGNING_SECRET`
- `WEBTUI_API_IMAGE`
- `WEBTUI_WORKER_IMAGE`

Để tài khoản mới sử dụng chat ngay sau khi đăng ký, cấu hình workspace mặc định trong `.env` production:

```env
REGISTRATION_DEFAULT_WORKSPACE_ID=UUID_WORKSPACE_PRODUCTION
```

Backend chỉ gán role hệ thống `workspace_member` và các kênh public thông thường; không tự cấp `workspace_admin`, `workspace_owner` hoặc kênh phiên bot riêng tư. Nếu chỉ có đúng một workspace active thì backend có thể tự nhận diện, nhưng production nên khai báo UUID rõ ràng để tránh chọn nhầm khi tạo thêm workspace.

## Khởi tạo HTTPS

Sau khi DNS `chat.vpsttt.com` và `chat.vpsttt.com` đã trỏ về IP VPS, có thể để `AUTO_INIT_TLS=true` trong `.env`; deploy script sẽ tự khởi tạo TLS lần đầu. Nếu muốn chạy thủ công:

```bash
cd /opt/webtui-chat
sh deploy/scripts/init-letsencrypt.sh
```

Script đọc `API_DOMAIN`, `FRONTEND_DOMAIN` và `LETSENCRYPT_EMAIL` từ `.env`, tạo chứng chỉ tạm, xin chứng chỉ Let's Encrypt thật rồi reload Nginx.

Gia hạn thủ công khi cần:

```bash
cd /opt/webtui-chat
sh deploy/scripts/renew-letsencrypt.sh
```

Nên thêm cron trên VPS để gia hạn định kỳ:

```cron
0 3 * * * cd /opt/webtui-chat && sh deploy/scripts/renew-letsencrypt.sh >> /var/log/webtui-certbot.log 2>&1
```

## GitHub Secrets

Trong GitHub Environment `production`, tạo secrets:

```text
DEPLOY_HOST=IP_VPS_TRONG_VPS_INFO
DEPLOY_USER=root
DEPLOY_PASSWORD=MAT_KHAU_TRONG_VPS_INFO
DEPLOY_SSH_KEY=
GHCR_USERNAME=GITHUB_USERNAME
GHCR_TOKEN=TOKEN_CO_QUYEN_READ_PACKAGES
```

Khuyến nghị sau giai đoạn đầu: tạo SSH key riêng cho deploy, đưa private key vào `DEPLOY_SSH_KEY`, rồi bỏ `DEPLOY_PASSWORD`.

Repository variables nên có:

```text
DEPLOY_PATH=/opt/webtui-chat
API_HEALTH_URL=https://chat.vpsttt.com/ready
```

## Luồng CI/CD

1. Push code lên `main`.
2. Workflow `Docker` build và push image:
   - `ghcr.io/<owner>/<repo>/api:<tag>`
   - `ghcr.io/<owner>/<repo>/worker:<tag>`
   - `ghcr.io/<owner>/<repo>/web:<tag>`
3. Mở workflow `Deploy`.
4. Chọn `environment=production`.
5. Để trống `image_tag` để dùng SHA commit hiện tại, hoặc nhập `latest`/SHA image đã tồn tại trên GHCR.
6. Workflow SSH vào VPS, đồng bộ thư mục `deploy`, login GHCR, chạy migration và `docker compose up -d`.
7. Workflow gọi health check `https://chat.vpsttt.com/ready`.

Lưu ý: deploy tự động sau workflow `Docker` sẽ dùng tag full SHA của commit vừa build. Nếu chạy deploy thủ công và gặp `manifest unknown`, image tag đang chọn chưa tồn tại; hãy chạy workflow `Docker` trước rồi deploy lại với tag SHA đó, hoặc chỉ dùng `latest` sau khi Docker workflow trên `main`/`master` đã hoàn tất.

## Kiểm tra sau deploy

```bash
curl -fsS https://chat.vpsttt.com/health
curl -fsS https://chat.vpsttt.com/ready
curl -fsS https://chat.vpsttt.com/version
curl -fsS https://chat.vpsttt.com/metrics
```

Kiểm tra container:

```bash
cd /opt/webtui-chat
docker compose -f deploy/docker/compose.prod.yml ps
docker compose -f deploy/docker/compose.prod.yml logs --tail=100 api
docker compose -f deploy/docker/compose.prod.yml logs --tail=100 worker
```

## Seed demo nội bộ

Sau khi API production chạy ổn, tạo user quản trị và workspace demo qua API để đi đúng luồng audit/RBAC.

Có thể dùng các block trong `backend/docs/local-run.md`, chỉ cần đổi base URL:

```powershell
$baseUrl = "https://chat.vpsttt.com"
```

Luồng seed tối thiểu:

1. Register user quản trị đầu tiên.
2. Tạo workspace `vpsttt`.
3. Tạo channel `thong-bao`, `ky-thuat`, `sale`.
4. Tạo bot/server alert nếu cần demo tích hợp.
5. Gửi một message mẫu để kiểm tra WebSocket, notification và search.

## Rollback

Đổi tag image trong `.env` hoặc chạy lại workflow `Deploy` với `image_tag` cũ.

Nếu migration đã thay đổi dữ liệu theo hướng destructive, cần restore backup hoặc có kế hoạch rollback riêng.
