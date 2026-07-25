-- Chỉ gỡ dữ liệu do migration tạo và chưa bị bỏ marker hệ thống.
DELETE FROM bot_installations
WHERE config @> '{"system_default": true}'::jsonb;

DELETE FROM bots
WHERE settings @> '{"system_default": true}'::jsonb;

DELETE FROM channels
WHERE settings @> '{"system_default": true}'::jsonb;
