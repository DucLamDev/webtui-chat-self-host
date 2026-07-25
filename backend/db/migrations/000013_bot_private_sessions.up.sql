-- Private per-user sessions for channels that handle customer/account data.

UPDATE channels
SET settings = settings || '{"bot_session_mode":"private"}'::jsonb,
    updated_at = now()
WHERE slug::text IN ('ke-toan', 'gia-han', 'ticket')
  AND deleted_at IS NULL;

-- Source channels are entry points only. Their messages are handled in a
-- dedicated direct channel created for each workspace member.
DELETE FROM channel_members cm
USING channels c
WHERE c.id = cm.channel_id
  AND c.settings->>'bot_session_mode' = 'private';

CREATE UNIQUE INDEX channels_bot_private_session_uidx
ON channels (
    workspace_id,
    (settings->>'bot_source_channel_id'),
    (settings->>'bot_session_user_id')
)
WHERE settings @> '{"bot_session":true}'::jsonb
  AND deleted_at IS NULL;

INSERT INTO permissions (code, module, action, name, description)
VALUES (
    'order.payment_request',
    'order',
    'payment_request',
    'Yêu cầu thanh toán Order',
    'Cho phép tạo QR nạp ví hoặc QR thanh toán đơn hàng trong phiên bot riêng tư'
)
ON CONFLICT (code) DO NOTHING;

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
JOIN permissions p ON p.code = 'order.payment_request'
WHERE r.code IN ('workspace_owner', 'workspace_admin', 'workspace_member')
  AND r.deleted_at IS NULL
ON CONFLICT DO NOTHING;
