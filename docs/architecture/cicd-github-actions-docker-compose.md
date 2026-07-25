# CI/CD với GitHub Actions và Docker Compose

Tài liệu này mô tả luồng CI/CD mục tiêu cho WebTui Chat khi triển khai lên VPS hoặc máy chủ tự quản bằng Docker Compose.

## Cơ sở thiết kế

- GitHub Actions lưu workflow trong `.github/workflows`.
- Mỗi workflow gồm trigger, job và step rõ ràng.
- Secret triển khai lưu trong GitHub Secrets hoặc Environment Secrets, không commit vào repo.
- Docker image được build và push lên GitHub Container Registry.
- Server production dùng Docker Compose để pull image, chạy migration, restart service và health check.
- Docker Compose dùng `healthcheck` và `depends_on.condition: service_healthy` để API/worker chỉ chạy sau khi PostgreSQL và Redis nội bộ sẵn sàng; RabbitMQ và MinIO production dùng dịch vụ managed qua URL cấu hình.

## Workflow đề xuất

```text
Pull request
-> ci.yml
   -> kiểm tra tài liệu
   -> test backend nếu có go.mod
   -> test frontend nếu có package.json

Push main
-> ci.yml
-> docker.yml
   -> build image api
   -> build image worker
   -> push GHCR

Manual deploy hoặc tag release
-> deploy.yml
   -> SSH vào server
   -> docker login ghcr.io
   -> sync deploy config
   -> docker compose pull
   -> migration
   -> docker compose up -d
   -> health check
```

## File workflow

- `.github/workflows/ci.yml`: kiểm tra nền tảng.
- `.github/workflows/docker.yml`: build và push Docker image.
- `.github/workflows/deploy.yml`: triển khai bằng Docker Compose.

## Secret và variable cần có

Repository secrets hoặc environment secrets:

- `DEPLOY_HOST`: IP hoặc domain server.
- `DEPLOY_USER`: user SSH.
- `DEPLOY_PASSWORD`: mật khẩu SSH tạm thời nếu chưa có key.
- `DEPLOY_SSH_KEY`: private key SSH, khuyến nghị dùng cho production lâu dài.
- `GHCR_USERNAME`: username dùng để login GHCR trên server.
- `GHCR_TOKEN`: token có quyền pull package nếu image private.

Repository variables hoặc environment variables:

- `DEPLOY_PATH`: thư mục triển khai trên server, ví dụ `/opt/webtui-chat`.
- `API_HEALTH_URL`: URL health check public, ví dụ `https://chat.vpsttt.com/ready`.

File `.env` production nằm trên server tại `${DEPLOY_PATH}/.env`; pipeline không ghi đè secret runtime này.

## Docker Compose

- `deploy/docker/compose.dev.yml`: chạy hạ tầng local. API/worker nằm trong profile `app` để không bắt buộc có code backend ngay từ đầu.
- `deploy/docker/compose.prod.yml`: chạy production bằng image đã publish, PostgreSQL/Redis nội bộ, RabbitMQ CloudAMQP và MinIO/S3 ngoài VPS.
- `deploy/scripts/deploy-compose.sh`: lệnh chuẩn để pull image, migrate và restart.
- `deploy/scripts/init-letsencrypt.sh`: khởi tạo chứng chỉ HTTPS cho `chat.vpsttt.com` và `chat.vpsttt.com`.
- `deploy/.env.example`: mẫu `.env` production không chứa secret thật.

Yêu cầu Docker Compose v2.24 trở lên vì file Compose dùng `env_file.required` để phân biệt file môi trường bắt buộc và không bắt buộc.

Chạy hạ tầng local:

```sh
docker compose -f deploy/docker/compose.dev.yml up -d postgres redis rabbitmq minio minio-init
```

Khi backend đã có Dockerfile thật:

```sh
docker compose -f deploy/docker/compose.dev.yml --profile app up -d
```

## Chính sách môi trường

- Pull request chỉ chạy CI.
- Push `main` build image `edge` hoặc tag theo SHA.
- Release tag `v*` build image version cố định.
- Deploy production nên chạy qua GitHub Environment `production` và bật required reviewers.

## Health check

Health check tối thiểu:

- PostgreSQL: `pg_isready`.
- Redis: `redis-cli ping`.
- RabbitMQ: `rabbitmq-diagnostics ping`.
- MinIO: `/minio/health/live`.
- API: `/health`.

Nếu health check thất bại, deploy phải dừng trước khi cleanup image cũ để còn đường rollback.

## Rollback

Rollback production bằng cách đặt lại `WEBTUI_API_IMAGE` và `WEBTUI_WORKER_IMAGE` trong `.env` server về tag cũ, sau đó chạy:

```sh
docker compose -f deploy/docker/compose.prod.yml pull api worker
docker compose -f deploy/docker/compose.prod.yml up -d api worker
```

Migration destructive cần kế hoạch riêng, không rollback tự động nếu đã thay đổi dữ liệu không thể đảo ngược.
