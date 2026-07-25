# Kiến trúc backend

Backend dùng Go + Gin theo modular monolith và Clean Architecture.

## Entry point

- `cmd/api`: chạy REST API và WebSocket.
- `cmd/worker`: chạy queue consumer, cronjob và background job.

## Lớp nội bộ

- `internal/bootstrap`: khởi tạo ứng dụng, provider, router và worker.
- `internal/config`: đọc cấu hình từ biến môi trường.
- `internal/platform`: adapter kỹ thuật dùng chung.
- `internal/shared`: helper, response, middleware, validator, errors và DTO dùng chung.
- `internal/modules`: module nghiệp vụ.

## Quy tắc

- Không đặt nghiệp vụ trong `cmd`.
- Không để `domain` import framework hoặc driver kỹ thuật.
- Handler chỉ parse request, gọi application và map response.
- Worker gọi application giống delivery khác.
- Adapter kỹ thuật phải nằm trong `infrastructure` hoặc `platform`.

