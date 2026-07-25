-- Seed scope mặc định cho API token và tích hợp ngoài hệ thống.

WITH seed_scopes (code, name, description, module, action) AS (
    VALUES
        ('message.read', 'Đọc tin nhắn', 'Cho phép đọc dữ liệu tin nhắn qua API tích hợp', 'message', 'read'),
        ('message.write', 'Gửi tin nhắn', 'Cho phép gửi tin nhắn qua API token, webhook hoặc bot', 'message', 'write'),
        ('file.write', 'Gửi file', 'Cho phép upload hoặc gắn file qua API tích hợp', 'file', 'write'),
        ('bot.write', 'Gửi bot message', 'Cho phép bot gửi tin nhắn vào kênh', 'bot', 'write'),
        ('webhook.write', 'Gửi webhook', 'Cho phép incoming webhook ghi sự kiện vào workspace', 'webhook', 'write'),
        ('admin.read', 'Đọc dữ liệu quản trị', 'Cho phép đọc dữ liệu quản trị ở phạm vi an toàn', 'admin', 'read')
)
INSERT INTO api_scopes (code, name, description, module, action)
SELECT code, name, description, module, action
FROM seed_scopes
ON CONFLICT (code) DO UPDATE
SET name = EXCLUDED.name,
    description = EXCLUDED.description,
    module = EXCLUDED.module,
    action = EXCLUDED.action;
