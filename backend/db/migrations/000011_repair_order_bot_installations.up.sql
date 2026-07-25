-- Repair order/automation bot targets for workspaces created before/after the
-- default workspace seed. This keeps auto bots from failing silently when a
-- channel exists but its bot target has not been installed yet.

INSERT INTO bots (workspace_id, slug, name, description, created_by, settings)
SELECT
    w.id,
    definition.slug,
    definition.name,
    definition.description,
    w.owner_id,
    '{"system_default": true, "phase": "order_bot_phase1", "repair": "order_bot_targets"}'::jsonb
FROM workspaces w
CROSS JOIN (
    VALUES
        ('ticket-bot', 'Ticket Bot', 'Báo ticket mới từ web bán hàng/billing và tự phân loại yêu cầu hỗ trợ'),
        ('server-alert-bot', 'Server Alert Bot', 'Báo server mất ping, port lỗi, dịch vụ down và gợi ý checklist xử lý'),
        ('gia-han-bot', 'Gia Hạn Bot', 'Báo khách hoặc dịch vụ sắp hết hạn từ hệ thống order VPSTTT'),
        ('cskh-bot', 'CSKH Bot', 'Tra cứu ví, dịch vụ và hỗ trợ khách hàng từ hệ thống order VPSTTT'),
        ('thanh-toan-bot', 'Thanh Toán Bot', 'Tạo QR nạp ví và hỗ trợ kiểm tra thanh toán')
) AS definition(slug, name, description)
WHERE w.deleted_at IS NULL
ON CONFLICT (workspace_id, slug) WHERE deleted_at IS NULL
DO UPDATE SET
    name = EXCLUDED.name,
    description = EXCLUDED.description,
    status = 'active',
    settings = bots.settings || EXCLUDED.settings;

INSERT INTO bot_installations (bot_id, workspace_id, channel_id, status, config)
SELECT
    b.id,
    b.workspace_id,
    c.id,
    'active',
    '{"system_default": true, "phase": "order_bot_phase1", "repair": "order_bot_targets"}'::jsonb
FROM bots b
JOIN (
    VALUES
        ('ticket-bot', 'ticket'),
        ('server-alert-bot', 'server-alert'),
        ('gia-han-bot', 'gia-han'),
        ('cskh-bot', 'ticket'),
        ('thanh-toan-bot', 'ke-toan')
) AS mapping(bot_slug, channel_slug)
  ON mapping.bot_slug = b.slug::text
JOIN channels c
  ON c.workspace_id = b.workspace_id
 AND c.slug::text = mapping.channel_slug
 AND c.status = 'active'
 AND c.deleted_at IS NULL
WHERE b.status = 'active'
  AND b.deleted_at IS NULL
ON CONFLICT (bot_id, workspace_id, channel_id) WHERE channel_id IS NOT NULL
DO UPDATE SET
    status = 'active',
    config = bot_installations.config || EXCLUDED.config;
