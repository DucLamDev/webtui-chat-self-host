# Mục lục tài liệu

Tài liệu dự án WebTui Chat được viết bằng tiếng Việt có dấu. Khi thêm tài liệu mới, hãy ưu tiên mô tả quyết định kiến trúc, ranh giới trách nhiệm, luồng dữ liệu và quy tắc kiểm thử.

## Kiến trúc

- [Tổng quan kiến trúc](architecture/overview.md)
- [Portal, instance self-hosted và client dùng chung](architecture/self-hosted-portal-clients.md)
- [Cấu trúc thư mục nguồn](architecture/source-layout.md)
- [Clean Architecture cho backend](architecture/backend-clean-architecture.md)
- [Chuẩn module backend](architecture/module-template.md)
- [Realtime, queue và worker](architecture/realtime-queue.md)
- [Vận hành và triển khai](architecture/operations.md)
- [CI/CD với GitHub Actions và Docker Compose](architecture/cicd-github-actions-docker-compose.md)

## Database

- [Thiết kế database PostgreSQL](database/postgresql-design.md)
- [ERD PostgreSQL dạng Mermaid](../backend/db/schema/erd.mmd)
- [Migration schema nền](../backend/db/migrations/000001_initial_schema.up.sql)

## Kế hoạch triển khai

- [Cài đặt self-hosted cho customer](../deploy/self-hosted/README.md)
- [Triển khai portal trung tâm](../portal/deploy/README.md)
- [Kế hoạch hoàn thiện backend](planning/backend-roadmap.md)
- [Kế hoạch cải thiện Push, Realtime, Session và File Transfer](planning/cross-platform-reliability-improvement-plan.md)
- [Deploy production lên VPS](deploy/vps-production.md)
- [CI/CD từng bước với GitHub Actions và Docker Compose](deploy/cicd.md)

Kế hoạch và tài liệu release mobile/desktop đã chuyển sang
[`clients/docs/`](../clients/docs/).

## Backend contract

- [API convention](../backend/docs/api-convention.md)
- [Event convention](../backend/docs/event-convention.md)
- [Security baseline](../backend/docs/security-baseline.md)
- [Auth, user và RBAC](../backend/docs/auth-rbac.md)
- [Workspace, phòng ban, kênh và direct message](../backend/docs/workspace-channel.md)
- [Order Bot VPSTTT Phase 1](../backend/docs/order-bot-phase1.md)
- [Chạy backend local](../backend/docs/local-run.md)
- [OpenAPI nền](../backend/api/openapi/openapi.yaml)
