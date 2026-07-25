# Thiết kế database PostgreSQL

Tài liệu này mô tả schema PostgreSQL nền cho WebTui Chat. Thiết kế ưu tiên MVP có thể chạy được, nhưng vẫn giữ sẵn ranh giới cho admin panel, bot, webhook, cronjob, audit, backup và worker.

## Cơ sở thiết kế

- PostgreSQL là nguồn dữ liệu chính cho nghiệp vụ.
- Redis chỉ dùng cho cache, session ngắn hạn, rate limit hoặc presence tạm thời; dữ liệu bền vững vẫn cần về PostgreSQL.
- MinIO/S3/local storage chỉ lưu object file; metadata file nằm trong bảng `files`.
- RabbitMQ nhận event từ bảng `outbox_events`, tránh mất event khi transaction lưu dữ liệu thành công nhưng publish queue thất bại.
- Soft delete dùng `deleted_at` cho dữ liệu cần giữ lịch sử.
- Metadata linh hoạt dùng `jsonb`, nhưng các trường lọc/sắp xếp thường xuyên vẫn phải là cột riêng.

## Extension PostgreSQL

- `pgcrypto`: dùng `gen_random_uuid()` để tạo UUID.
- `citext`: dùng cho email, username, slug để so khớp không phân biệt hoa thường.

PostgreSQL hỗ trợ kiểu `uuid` gốc cho định danh phân tán, `jsonb` phù hợp metadata cần query và có thể dùng GIN index. Partial index được dùng cho các unique constraint có soft delete, ví dụ email chỉ unique khi `deleted_at IS NULL`.

## Nhóm bảng chính

### Auth và user

- `users`: tài khoản người dùng.
- `user_sessions`: refresh token/session theo thiết bị.
- `workspace_invites`: lời mời tham gia workspace.

### Workspace và phân quyền

- `workspaces`: tenant chính.
- `workspace_settings`: cấu hình workspace dạng key/value để admin chỉnh từng nhóm cấu hình.
- `workspace_members`: thành viên workspace và vai trò.
- `departments`: phòng ban trong workspace.
- `department_members`: thành viên phòng ban.
- `permissions`: quyền hệ thống như delete channel, manage bot, invite user, manage webhook, view audit.
- `roles`: role global hoặc role riêng của workspace.
- `role_permissions`: quyền được gán vào role.
- `workspace_member_roles`: role được gán cho thành viên workspace.
- `channel_member_roles`: role được gán riêng trong channel.

### Chat realtime

- `channels`: kênh chat public, private, direct hoặc group.
- `channel_members`: thành viên kênh, trạng thái mute/left/removed và mốc đọc cuối.
- `direct_conversations`: bảng quản lý DM và group DM theo participant key.
- `direct_conversation_members`: thành viên của DM/group DM.
- `messages`: tin nhắn, partition hash theo `workspace_id`, có `parent_id`, `thread_root_id` và `search_vector`.
- `message_reactions`: reaction.
- `message_mentions`: mention người dùng.
- `message_reads`: read receipt chi tiết, chỉ dùng khi cần vì phần lớn trạng thái đọc nằm ở `channel_members.last_read_message_id`.
- `message_attachments`: liên kết message với file.
- `search_documents`: chỉ mục tìm kiếm tổng hợp cho message, file, channel, workspace hoặc bot.

### File

- `files`: metadata file, provider lưu trữ, object key, MIME type, dung lượng, checksum và trạng thái xử lý.
- `file_versions`: lịch sử phiên bản file cho avatar, logo, document hoặc file có cập nhật.

### Notification

- `notifications`: thông báo cho user, dùng để đồng bộ trạng thái đọc và gửi realtime/push/email sau này.
- `notification_jobs`: hàng đợi gửi thông báo qua desktop, push, email, webhook hoặc SMS.

### Bot, API token và webhook

- `bots`: bot trong workspace.
- `bot_installations`: bot được cài vào workspace hoặc channel.
- `api_tokens`: token tích hợp, chỉ lưu hash.
- `api_scopes`: danh mục scope API.
- `api_token_scopes`: scope được cấp cho token.
- `incoming_webhooks`: webhook nhận dữ liệu từ hệ thống ngoài.
- `outgoing_webhooks`: webhook phát event ra ngoài.
- `webhook_deliveries`: lịch sử gửi webhook, retry và dead letter ở cấp ứng dụng.

### Worker, audit và backup

- `cron_jobs`: cấu hình job định kỳ, gồm `next_run_at`, `locked_at`, `locked_by` để worker nhiều node không chạy trùng.
- `cron_job_runs`: lịch sử chạy job, trạng thái, log và lỗi.
- `outbox_events`: event chờ publish sang RabbitMQ.
- `audit_logs`: log hành động quan trọng.
- `user_presence`: trạng thái online theo device/socket/node để WebSocket scale nhiều node.
- `backup_jobs`: cấu hình backup, gồm `backup_type`, `next_run_at`, `locked_at`, `locked_by` để worker chạy định kỳ an toàn.
- `backup_runs`: lịch sử backup, object key, byte size, checksum và lỗi nếu có.
- `system_settings`: cấu hình hệ thống dạng key/value.

## Luồng ghi tin nhắn

```text
messages
-> message_mentions / message_attachments nếu có
-> outbox_events: MessageCreated
-> worker publish RabbitMQ
-> WebSocket broadcast
-> notifications / webhook_deliveries / audit_logs nếu cần
```

`messages` dùng khóa chính composite `(workspace_id, id)` để hỗ trợ partition theo `workspace_id`. Các bảng con của message đều mang `workspace_id` và `message_id` để giữ foreign key rõ ràng.

## Index quan trọng

- `messages_channel_timeline_idx`: tải timeline theo channel.
- `messages_thread_root_idx`: tải reply trong thread không cần recursive query.
- `messages_search_vector_gin_idx`: tìm kiếm full text trên nội dung message.
- `channel_members_user_status_idx`: lấy danh sách kênh của user.
- `notifications_user_unread_idx`: lấy thông báo chưa đọc.
- `notification_jobs_pending_idx`: worker lấy job gửi notification.
- `user_presence_workspace_status_idx`: lấy presence theo workspace.
- `outbox_events_pending_idx`: worker lấy event pending.
- `webhook_deliveries_pending_idx`: worker retry webhook.
- Partial unique index cho `users.email`, `users.username`, `workspaces.slug`, `channels.slug`, `bots.slug`.

## Những cải thiện đã áp dụng

- Tách RBAC động qua `roles`, `permissions`, `role_permissions`, `workspace_member_roles`, `channel_member_roles`.
- Thêm `direct_conversations` để quản lý DM hiệu quả hơn thay vì chỉ dựa vào `channel.type`.
- Thêm `thread_root_id` trong `messages` để lấy toàn bộ thread bằng index.
- Thêm `notification_jobs` cho desktop, push, email, webhook và SMS.
- Thêm `user_presence` cho WebSocket scale nhiều node.
- Thêm full text search bằng `tsvector` và GIN index.
- Thêm `file_versions`.
- Thêm `before_data` và `after_data` trong `audit_logs`.
- Chuẩn hóa API token scope bằng `api_scopes` và `api_token_scopes`.
- Bổ sung quyền `api_token.manage` để tách quản lý API token khỏi quyền webhook.
- Bổ sung lock cronjob bằng `locked_at` và `locked_by`.
- Bổ sung lịch chạy và lock cho backup job bằng `next_run_at`, `locked_at`, `locked_by`.
- Partition `messages` theo hash `workspace_id`.
- Giữ `message_reads` là bảng chi tiết tùy chọn, còn mốc đọc chính nằm trong `channel_members`.
- Tách `workspace_settings` khỏi `workspaces.settings`.

## Quy tắc migration

- Không sửa migration đã chạy ở staging/production.
- Thay đổi schema phải thêm migration mới.
- Migration destructive cần có tài liệu rollback hoặc backup trước khi chạy.
- Không để seed production trong migration.
- Mọi bảng nghiệp vụ phải có khóa chính rõ ràng.
- Foreign key dùng `ON DELETE CASCADE` cho bảng con thuần liên kết, dùng `SET NULL` khi cần giữ lịch sử.

## Gợi ý mở rộng sau MVP

- Subpartition `messages` theo tháng nếu một workspace đơn lẻ có dữ liệu quá lớn.
- Partition `audit_logs` và `webhook_deliveries` theo thời gian khi dữ liệu lớn.
- Thêm read replica cho truy vấn báo cáo hoặc admin dashboard.
- Thêm search backend chuyên dụng như OpenSearch hoặc Meilisearch nếu full text search PostgreSQL không còn đủ.
- Thêm bảng `message_threads` nếu thread cần metadata riêng như trạng thái resolve, subscriber hoặc SLA.
