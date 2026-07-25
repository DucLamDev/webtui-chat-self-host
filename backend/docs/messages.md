# Tin nhắn, thread, mention và reaction

Phase 5 bổ sung module `messages` theo Clean Architecture:

- `domain`: entity tin nhắn, reaction summary và lỗi nghiệp vụ.
- `application`: validate quyền, validate nội dung, chuẩn hóa mention và điều phối use case.
- `infrastructure/postgres`: lưu `messages`, `message_mentions`, `message_reactions`, `search_documents` và `outbox_events`.
- `delivery/http`: REST API cho timeline, gửi/sửa/xóa tin nhắn, thread, reaction và search.

## Endpoint chính

| Method | Path | Mục đích |
|---|---|---|
| `GET` | `/api/v1/workspaces/{workspace_id}/channels/{channel_id}/messages` | Lấy timeline theo `created_at DESC` |
| `POST` | `/api/v1/workspaces/{workspace_id}/channels/{channel_id}/messages` | Gửi tin nhắn text |
| `GET` | `/api/v1/workspaces/{workspace_id}/channels/{channel_id}/messages/{message_id}` | Xem chi tiết tin nhắn |
| `PATCH` | `/api/v1/workspaces/{workspace_id}/channels/{channel_id}/messages/{message_id}` | Sửa nội dung tin nhắn |
| `DELETE` | `/api/v1/workspaces/{workspace_id}/channels/{channel_id}/messages/{message_id}` | Xóa mềm tin nhắn |
| `GET` | `/api/v1/workspaces/{workspace_id}/channels/{channel_id}/messages/{message_id}/thread` | Lấy root message và reply theo `thread_root_id` |
| `POST` | `/api/v1/workspaces/{workspace_id}/channels/{channel_id}/messages/{message_id}/reactions` | Thêm reaction |
| `DELETE` | `/api/v1/workspaces/{workspace_id}/channels/{channel_id}/messages/{message_id}/reactions/{emoji}` | Xóa reaction của user hiện tại |
| `GET` | `/api/v1/workspaces/{workspace_id}/messages/search?q=...` | Tìm kiếm tin nhắn bằng PostgreSQL full text search |

## Gửi tin nhắn

```json
{
  "body": "Chào team <@11111111-1111-1111-1111-111111111111>",
  "parent_id": "",
  "kind": "text",
  "metadata": {
    "client_id": "web"
  },
  "mentioned_user_ids": []
}
```

`mentioned_user_ids` là tùy chọn. Backend cũng tự parse mention theo dạng `<@uuid>` trong `body`. Mention chỉ hợp lệ nếu user đang là member active/muted của channel.

## Thread

Khi gửi reply, truyền `parent_id`. Backend tự tính `thread_root_id`:

- Reply trực tiếp vào root: `thread_root_id = parent.id`.
- Reply vào reply khác: `thread_root_id = parent.thread_root_id`.

Cách này giúp lấy thread không cần recursive query.

## Search

Tin nhắn được tìm bằng `messages.search_vector` và đồng bộ thêm vào `search_documents` để sau này mở rộng search đa nguồn như file, channel, bot. Phase hiện tại dùng cấu hình PostgreSQL `simple`.

## Event realtime nền

Mỗi thao tác quan trọng ghi outbox event trong cùng transaction:

- `MessageCreated`
- `MessageUpdated`
- `MessageDeleted`
- `ReactionChanged`

Worker publish RabbitMQ/WebSocket sẽ được nối tiếp ở phase notification/outbox. API phase 5 đã chuẩn bị dữ liệu event bền vững trước.

Ngoài outbox, service cũng broadcast best-effort vào `platform/websocket.Manager` với room:

```text
workspace:{workspace_id}:channel:{channel_id}
```

Client join đúng room này để nhận `MessageCreated`, `MessageUpdated`, `MessageDeleted` và `ReactionChanged`.

## WebSocket endpoint

Endpoint realtime hiện tại:

```text
GET /api/v1/ws
Authorization: Bearer <access_token>
```

Sau khi kết nối, client gửi lệnh:

```json
{
  "type": "join",
  "room": "workspace:{workspace_id}:channel:{channel_id}"
}
```

Muốn rời room, gửi `type = "leave"` với cùng giá trị `room`.
