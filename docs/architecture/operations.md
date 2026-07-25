# Vận hành và triển khai

Mục tiêu vận hành là có thể build, test, deploy, rollback và quan sát hệ thống một cách lặp lại được.

## Môi trường

- `dev`: chạy local bằng Docker Compose hoặc service cục bộ.
- `staging`: giống production nhưng dữ liệu không phải dữ liệu thật.
- `production`: bật backup, monitoring, alert và cấu hình bảo mật đầy đủ.

## CI/CD

Pipeline GitHub Actions mục tiêu:

```text
Developer push
-> lint
-> test
-> security scan
-> Docker build
-> push image
-> deploy
-> migration
-> restart service
-> health check
-> notification
```

Workflow nên tách theo trách nhiệm:

- `backend.yml`: lint, test và build backend.
- `frontend.yml`: lint, typecheck, test và build frontend.
- `docker.yml`: build và push image.
- `deploy.yml`: triển khai staging/production.
- `release.yml`: tạo release tag và changelog nếu cần.

Thiết kế chi tiết nằm ở [CI/CD với GitHub Actions và Docker Compose](cicd-github-actions-docker-compose.md).

## Docker và hạ tầng

`deploy/` chứa cấu hình cho:

- API server.
- Worker.
- Nginx.
- PostgreSQL.
- Redis.
- RabbitMQ.
- MinIO.
- Prometheus.
- Grafana.
- Loki.
- AlertManager.

File Compose hiện tại:

- `deploy/docker/compose.dev.yml`: chạy hạ tầng local và có profile `app` cho API/worker.
- `deploy/docker/compose.prod.yml`: chạy production bằng image đã publish, dùng PostgreSQL/Redis nội bộ VPS, RabbitMQ CloudAMQP và MinIO/S3 ngoài VPS.
- `deploy/scripts/deploy-compose.sh`: script pull image, chạy migration và restart service.
- `deploy/scripts/init-letsencrypt.sh`: script khởi tạo HTTPS cho `chat.vpsttt.com` và `chat.vpsttt.com`.
- `deploy/.env.example`: mẫu biến môi trường production, không chứa secret thật.

## Database

- Migration phải chạy tự động trong pipeline deploy hoặc bằng job riêng có kiểm soát.
- Không sửa migration đã chạy trên production.
- Dữ liệu seed chỉ dùng cho dev/staging, không chạy tự động trên production.
- Query đọc nặng nên được tối ưu trước khi đưa sang read replica.

## Backup và restore

- Backup PostgreSQL theo lịch.
- Backup file storage theo lịch.
- Lưu backup ở ít nhất một nơi ngoài server chính.
- Kiểm thử restore định kỳ, vì backup chưa được kiểm thử chưa thể xem là an toàn.

## Monitoring

- Prometheus thu metric.
- Grafana hiển thị dashboard.
- Loki lưu log tập trung.
- AlertManager gửi cảnh báo.

Metric tối thiểu:

- Latency và error rate của API.
- Số connection WebSocket.
- Số message gửi/phút.
- Queue depth và consumer lag.
- Tỷ lệ retry/dead letter.
- CPU, RAM, disk và network.
- Tình trạng PostgreSQL, Redis, RabbitMQ và MinIO.

## Bảo mật

- Không commit secret.
- Bật TLS ở Nginx.
- Bật rate limit cho auth, upload, webhook và API nhạy cảm.
- Log request cần tránh token, password và secret.
- Webhook cần ký request hoặc dùng token riêng.
- Admin API cần phân quyền rõ ràng và audit log.
