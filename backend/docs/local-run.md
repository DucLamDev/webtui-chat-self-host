# Chạy backend local

Tài liệu này dùng cho môi trường local khi chạy backend trực tiếp bằng Go.

## Database local

Thông tin hiện tại:

```text
database: vpstttdb_chat
user: postgres
password: 123456
host: localhost
port: 5432
```

`DATABASE_URL` mặc định trong code:

```text
postgres://postgres:123456@localhost:5432/vpstttdb_chat?sslmode=disable
```

Nếu máy bạn dùng user khác, đặt biến môi trường trước khi chạy:

```powershell
$env:DATABASE_URL="postgres://postgres:123456@localhost:5432/vpstttdb_chat?sslmode=disable"
```

## Chạy migration

Từ thư mục `backend/`:

```powershell
go run ./cmd/migrate up
```

Lệnh này tạo bảng `schema_migrations` và chạy các file `*.up.sql` trong `db/migrations`.

## Chạy API

```powershell
go run ./cmd/api
```

Test:

```powershell
Invoke-RestMethod http://localhost:8080/health
Invoke-RestMethod http://localhost:8080/ready
Invoke-RestMethod http://localhost:8080/version
Invoke-RestMethod http://localhost:8080/api/v1
```

`/ready` sẽ kiểm tra:

- `database`
- `storage`
- `websocket`
- `redis` nếu `REDIS_ENABLED=true`
- `rabbitmq` nếu `RABBITMQ_ENABLED=true`

## Test auth, user và RBAC

Sau khi chạy migration và bật API:

`device_name` là tùy chọn. Nếu không gửi, backend sẽ tự suy ra từ `User-Agent` và lưu cùng IP của request vào database.

```powershell
$email = "admin@example.com"
$username = "admin1"
$password = "12345678"

$registerBody = @{
  email = $email
  username = $username
  display_name = "Quản trị viên"
  password = $password
} | ConvertTo-Json

try {
  $auth = Invoke-RestMethod `
    -Method Post `
    -Uri http://localhost:8080/api/v1/auth/register `
    -ContentType "application/json; charset=utf-8" `
    -Body $registerBody
} catch {
  $errorBody = $_.ErrorDetails.Message | ConvertFrom-Json
  if ($errorBody.error.code -ne "USER_ALREADY_EXISTS") {
    throw
  }

  $loginBody = @{
    identifier = $email
    password = $password
  } | ConvertTo-Json

  try {
    $auth = Invoke-RestMethod `
      -Method Post `
      -Uri http://localhost:8080/api/v1/auth/login `
      -ContentType "application/json; charset=utf-8" `
      -Body $loginBody
  } catch {
    $loginBody = @{
      identifier = $username
      password = $password
    } | ConvertTo-Json

    $auth = Invoke-RestMethod `
      -Method Post `
      -Uri http://localhost:8080/api/v1/auth/login `
      -ContentType "application/json; charset=utf-8" `
      -Body $loginBody
  }
}

$accessToken = $auth.data.tokens.access_token
$refreshToken = $auth.data.tokens.refresh_token
$headers = @{ Authorization = "Bearer $accessToken" }

Invoke-RestMethod -Headers $headers http://localhost:8080/api/v1/auth/me
Invoke-RestMethod -Headers $headers http://localhost:8080/api/v1/users/me
Invoke-RestMethod -Headers $headers http://localhost:8080/api/v1/users?limit=20
Invoke-RestMethod -Headers $headers http://localhost:8080/api/v1/rbac/permissions

$refreshBody = @{ refresh_token = $refreshToken } | ConvertTo-Json
Invoke-RestMethod `
  -Method Post `
  -Uri http://localhost:8080/api/v1/auth/refresh `
  -ContentType "application/json; charset=utf-8" `
  -Body $refreshBody
```

## Test workspace, channel và direct message

Sau khi đã có `$headers` từ bước login/register:

```powershell
$workspaceBody = @{
  slug = "vpsttt"
  name = "Workspace VPSTTT"
  description = "Workspace demo nội bộ"
} | ConvertTo-Json

$workspace = Invoke-RestMethod `
  -Method Post `
  -Headers $headers `
  -Uri http://localhost:8080/api/v1/workspaces `
  -ContentType "application/json; charset=utf-8" `
  -Body $workspaceBody

$workspaceId = $workspace.data.id

$channelBody = @{
  slug = "thong-bao"
  name = "Thông báo"
  type = "public"
} | ConvertTo-Json

$channel = Invoke-RestMethod `
  -Method Post `
  -Headers $headers `
  -Uri "http://localhost:8080/api/v1/workspaces/$workspaceId/channels" `
  -ContentType "application/json; charset=utf-8" `
  -Body $channelBody

$channelId = $channel.data.id

Invoke-RestMethod -Headers $headers "http://localhost:8080/api/v1/workspaces/$workspaceId/channels"
Invoke-RestMethod -Headers $headers "http://localhost:8080/api/v1/workspaces/$workspaceId/members"
Invoke-RestMethod -Headers $headers "http://localhost:8080/api/v1/rbac/me?workspace_id=$workspaceId"
```

## Test presence

Sau khi đã có `$headers` và `$workspaceId`:

```powershell
$presenceBody = @{
  device_id = "desktop-local"
  socket_id = "socket-local-1"
  node_id = "api-local"
  status = "online"
  metadata = @{
    platform = "desktop"
  }
} | ConvertTo-Json

Invoke-RestMethod `
  -Method Put `
  -Headers $headers `
  -Uri "http://localhost:8080/api/v1/workspaces/$workspaceId/presence/heartbeat" `
  -ContentType "application/json; charset=utf-8" `
  -Body $presenceBody

Invoke-RestMethod -Headers $headers "http://localhost:8080/api/v1/workspaces/$workspaceId/presence?limit=20"
```

## Test notification và worker

Mở thêm một terminal ở thư mục `backend/`:

```powershell
go run ./cmd/worker
```

Gửi một message có mention user khác trong kênh. Worker sẽ đọc `outbox_events`, tạo `notifications` và xử lý `notification_jobs`.
Để xem notification, hãy dùng token của user được mention hoặc đăng nhập lại bằng user đó.

```powershell
$messageBody = @{
  body = "Nhắc user <@USER_ID_KHAC>"
  mentioned_user_ids = @("USER_ID_KHAC")
} | ConvertTo-Json

Invoke-RestMethod `
  -Method Post `
  -Headers $headers `
  -Uri "http://localhost:8080/api/v1/workspaces/$workspaceId/channels/$channelId/messages" `
  -ContentType "application/json; charset=utf-8" `
  -Body $messageBody

Invoke-RestMethod -Headers $headers "http://localhost:8080/api/v1/notifications?workspace_id=$workspaceId&limit=20"
Invoke-RestMethod `
  -Method Put `
  -Headers $headers `
  -Uri "http://localhost:8080/api/v1/notifications/read-all?workspace_id=$workspaceId"
```

## Test API token, bot và webhook

Sau khi đã có `$headers`, `$workspaceId`, `$channelId`:

```powershell
Invoke-RestMethod -Headers $headers "http://localhost:8080/api/v1/api-scopes"

$tokenBody = @{
  name = "Server alert token"
  scopes = @("message.write")
} | ConvertTo-Json

$apiTokenResult = Invoke-RestMethod `
  -Method Post `
  -Headers $headers `
  -Uri "http://localhost:8080/api/v1/workspaces/$workspaceId/api-tokens" `
  -ContentType "application/json; charset=utf-8" `
  -Body $tokenBody

$apiToken = $apiTokenResult.data.token
$apiHeaders = @{ Authorization = "Bearer $apiToken" }

$apiMessageBody = @{
  channel_id = $channelId
  body = "Server alert gửi qua API token"
  metadata = @{
    severity = "info"
  }
} | ConvertTo-Json

Invoke-RestMethod `
  -Method Post `
  -Headers $apiHeaders `
  -Uri "http://localhost:8080/api/v1/integrations/messages" `
  -ContentType "application/json; charset=utf-8" `
  -Body $apiMessageBody
```

Test bot:

```powershell
$botBody = @{
  slug = "server-alert"
  name = "Server Alert"
  description = "Bot gửi cảnh báo server"
} | ConvertTo-Json

$bot = Invoke-RestMethod `
  -Method Post `
  -Headers $headers `
  -Uri "http://localhost:8080/api/v1/workspaces/$workspaceId/bots" `
  -ContentType "application/json; charset=utf-8" `
  -Body $botBody

$botId = $bot.data.id

$installBody = @{
  channel_id = $channelId
} | ConvertTo-Json

Invoke-RestMethod `
  -Method Post `
  -Headers $headers `
  -Uri "http://localhost:8080/api/v1/workspaces/$workspaceId/bots/$botId/installations" `
  -ContentType "application/json; charset=utf-8" `
  -Body $installBody

$botMessageBody = @{
  channel_id = $channelId
  body = "Bot gửi cảnh báo thử nghiệm"
} | ConvertTo-Json

Invoke-RestMethod `
  -Method Post `
  -Headers $headers `
  -Uri "http://localhost:8080/api/v1/workspaces/$workspaceId/bots/$botId/messages" `
  -ContentType "application/json; charset=utf-8" `
  -Body $botMessageBody
```

Test incoming webhook:

```powershell
$incomingBody = @{
  name = "GitHub Actions"
  channel_id = $channelId
} | ConvertTo-Json

$incoming = Invoke-RestMethod `
  -Method Post `
  -Headers $headers `
  -Uri "http://localhost:8080/api/v1/workspaces/$workspaceId/incoming-webhooks" `
  -ContentType "application/json; charset=utf-8" `
  -Body $incomingBody

$incomingSecret = $incoming.data.secret
$incomingUrl = $incoming.data.url

$incomingMessageBody = @{
  secret = $incomingSecret
  body = "Deploy production thành công"
  metadata = @{
    source = "github-actions"
  }
} | ConvertTo-Json

Invoke-RestMethod `
  -Method Post `
  -Uri $incomingUrl `
  -ContentType "application/json; charset=utf-8" `
  -Body $incomingMessageBody
```

Test outgoing webhook cần một URL nhận HTTP POST. Có thể dùng webhook.site hoặc một endpoint nội bộ riêng khi dev:

```powershell
$outgoingBody = @{
  name = "Message event receiver"
  target_url = "https://example.com/webtui-webhook"
  event_types = @("MessageCreated")
} | ConvertTo-Json

$outgoing = Invoke-RestMethod `
  -Method Post `
  -Headers $headers `
  -Uri "http://localhost:8080/api/v1/workspaces/$workspaceId/outgoing-webhooks" `
  -ContentType "application/json; charset=utf-8" `
  -Body $outgoingBody

$outgoingId = $outgoing.data.id
Invoke-RestMethod -Headers $headers "http://localhost:8080/api/v1/workspaces/$workspaceId/outgoing-webhooks/$outgoingId/deliveries?limit=20"
```

## Test cronjob và cleanup worker

Mở thêm một terminal ở thư mục `backend/` để worker tự xử lý job đến hạn:

```powershell
go run ./cmd/worker
```

Sau khi đã có `$headers` và `$workspaceId`, tạo một cronjob cleanup session hết hạn:

```powershell
$cronBody = @{
  name = "Dọn session hết hạn local"
  description = "Cronjob cleanup session cũ khi dev"
  schedule = "@every 15m"
  runner = "builtin_cleanup"
  status = "active"
  payload = @{
    task = "cleanup_expired_sessions"
    older_than = "24h"
  }
} | ConvertTo-Json -Depth 5

$cron = Invoke-RestMethod `
  -Method Post `
  -Headers $headers `
  -Uri "http://localhost:8080/api/v1/workspaces/$workspaceId/cronjobs" `
  -ContentType "application/json; charset=utf-8" `
  -Body $cronBody

$cronJobId = $cron.data.id

Invoke-RestMethod -Headers $headers "http://localhost:8080/api/v1/workspaces/$workspaceId/cronjobs?limit=20"

Invoke-RestMethod `
  -Method Post `
  -Headers $headers `
  -Uri "http://localhost:8080/api/v1/workspaces/$workspaceId/cronjobs/$cronJobId/run"

Invoke-RestMethod -Headers $headers "http://localhost:8080/api/v1/workspaces/$workspaceId/cronjobs/$cronJobId/runs?limit=20"
```

Ví dụ HTTP runner:

```powershell
$httpCronBody = @{
  name = "Health ping local"
  schedule = "@every 1h"
  runner = "http"
  status = "paused"
  payload = @{
    method = "GET"
    url = "http://localhost:8080/health"
  }
} | ConvertTo-Json -Depth 5

Invoke-RestMethod `
  -Method Post `
  -Headers $headers `
  -Uri "http://localhost:8080/api/v1/workspaces/$workspaceId/cronjobs" `
  -ContentType "application/json; charset=utf-8" `
  -Body $httpCronBody
```

## Test admin, audit và backup

Sau khi đã có `$headers` và `$workspaceId`:

```powershell
Invoke-RestMethod -Headers $headers "http://localhost:8080/api/v1/workspaces/$workspaceId/admin/stats"
Invoke-RestMethod -Headers $headers "http://localhost:8080/api/v1/workspaces/$workspaceId/admin/health"

Invoke-RestMethod -Headers $headers "http://localhost:8080/api/v1/workspaces/$workspaceId/audit-logs?limit=20"

$backupBody = @{
  name = "Backup database local"
  target = "local"
  backup_type = "database"
  schedule = "@daily"
  status = "paused"
  config = @{}
} | ConvertTo-Json -Depth 5

$backupJob = Invoke-RestMethod `
  -Method Post `
  -Headers $headers `
  -Uri "http://localhost:8080/api/v1/workspaces/$workspaceId/backup-jobs" `
  -ContentType "application/json; charset=utf-8" `
  -Body $backupBody

$backupJobId = $backupJob.data.id

Invoke-RestMethod -Headers $headers "http://localhost:8080/api/v1/workspaces/$workspaceId/backup-jobs?limit=20"

Invoke-RestMethod `
  -Method Post `
  -Headers $headers `
  -Uri "http://localhost:8080/api/v1/workspaces/$workspaceId/backup-jobs/$backupJobId/run"

Invoke-RestMethod -Headers $headers "http://localhost:8080/api/v1/workspaces/$workspaceId/backup-jobs/$backupJobId/runs?limit=20"
```

Nếu lệnh chạy backup trả lỗi `pg_dump`, hãy cài PostgreSQL client và bảo đảm `BACKUP_PG_DUMP_PATH` trỏ đúng binary `pg_dump`.

## Test hardening và metrics

```powershell
Invoke-RestMethod http://localhost:8080/metrics

$corsHeaders = @{
  Origin = "http://localhost:3000"
  "Access-Control-Request-Method" = "GET"
}

Invoke-WebRequest `
  -Method Options `
  -Headers $corsHeaders `
  -Uri http://localhost:8080/api/v1

$adminCorsHeaders = @{
  Origin = "http://localhost:3001"
  "Access-Control-Request-Method" = "GET"
}

Invoke-WebRequest `
  -Method Options `
  -Headers $adminCorsHeaders `
  -Uri http://localhost:8080/api/v1
```

Nếu cần kiểm thử rate limit nhanh, giảm tạm:

```powershell
$env:RATE_LIMIT_PER_MINUTE="1"
$env:RATE_LIMIT_BURST="0"
go run ./cmd/api
```

Muốn test direct message, cần có thêm một user khác và thêm user đó vào workspace, sau đó gọi:

```powershell
$directBody = @{
  participant_ids = @("USER_ID_KHAC")
} | ConvertTo-Json

Invoke-RestMethod `
  -Method Post `
  -Headers $headers `
  -Uri "http://localhost:8080/api/v1/workspaces/$workspaceId/direct-conversations" `
  -ContentType "application/json; charset=utf-8" `
  -Body $directBody
```

Nếu chỉ muốn login, dùng riêng block này:

```powershell
$loginBody = @{
  identifier = "admin@example.com"
  password = "12345678"
} | ConvertTo-Json

$login = Invoke-RestMethod `
  -Method Post `
  -Uri http://localhost:8080/api/v1/auth/login `
  -ContentType "application/json; charset=utf-8" `
  -Body $loginBody
```

## Chạy test trong sandbox hoặc khi cache bị chặn

Nếu gặp lỗi quyền truy cập Go build cache, đặt `GOCACHE` vào workspace:

```powershell
$env:GOCACHE="C:\Users\duclam\Desktop\application-chat-be\backend\.gocache"
go test ./...
```

## Tạm tắt database khi chỉ muốn test API nền

```powershell
$env:DATABASE_ENABLED="false"
go run ./cmd/api
```
