-- Bổ sung bộ kênh và bot Phase 1 cho các workspace đã tồn tại.
-- Workspace tạo mới được provision trong cùng transaction ở workspace repository.

INSERT INTO channels (workspace_id, slug, name, description, type, created_by, settings)
SELECT
    w.id,
    definition.slug,
    definition.name,
    definition.description,
    definition.channel_type,
    w.owner_id,
    '{"system_default": true}'::jsonb
FROM workspaces w
CROSS JOIN (
    VALUES
        ('thong-bao', 'Thông báo', 'Thông báo chung của workspace', 'public'),
        ('ban-giam-doc', 'Ban giám đốc', 'Trao đổi dành cho quản lý cấp cao', 'private'),
        ('ky-thuat', 'Kỹ thuật', 'Kỹ thuật và vận hành hệ thống', 'public'),
        ('sale', 'Sale', 'Kinh doanh và chăm sóc khách hàng', 'public'),
        ('ke-toan', 'Kế toán', 'Hóa đơn và thanh toán', 'private'),
        ('ticket', 'Ticket', 'Tiếp nhận ticket khách hàng', 'public'),
        ('server-alert', 'Server Alert', 'Cảnh báo server và dịch vụ', 'public'),
        ('gia-han', 'Gia hạn', 'Theo dõi gia hạn dịch vụ', 'public'),
        ('ban-giao-ca', 'Bàn giao ca', 'Bàn giao ca trực vận hành', 'public')
) AS definition(slug, name, description, channel_type)
WHERE w.deleted_at IS NULL
ON CONFLICT (workspace_id, slug) WHERE slug IS NOT NULL AND deleted_at IS NULL
DO NOTHING;

-- Owner luôn là thành viên của toàn bộ kênh mặc định, kể cả kênh private.
INSERT INTO channel_members (channel_id, user_id, status)
SELECT c.id, c.created_by, 'active'
FROM channels c
WHERE c.created_by IS NOT NULL
  AND c.status = 'active'
  AND c.deleted_at IS NULL
  AND c.settings @> '{"system_default": true}'::jsonb
ON CONFLICT (channel_id, user_id) DO UPDATE SET status = 'active';

-- Mọi kênh public chỉ được trả về cho thành viên của chính kênh. Đồng bộ
-- membership cho các thành viên workspace hiện hữu để không làm gián đoạn UX.
INSERT INTO channel_members (channel_id, user_id, status)
SELECT c.id, wm.user_id, 'active'
FROM channels c
JOIN workspace_members wm
  ON wm.workspace_id = c.workspace_id
 AND wm.status = 'active'
WHERE c.type = 'public'
  AND c.status = 'active'
  AND c.deleted_at IS NULL
  AND c.settings @> '{"system_default": true}'::jsonb
ON CONFLICT (channel_id, user_id) DO UPDATE SET status = 'active';

INSERT INTO bots (workspace_id, slug, name, description, created_by, settings)
SELECT
    w.id,
    definition.slug,
    definition.name,
    definition.description,
    w.owner_id,
    '{"system_default": true}'::jsonb
FROM workspaces w
CROSS JOIN (
    VALUES
        ('ticket-bot', 'Ticket Bot', 'Báo ticket mới từ web bán hàng/billing'),
        ('server-alert-bot', 'Server Alert Bot', 'Báo server mất ping, port lỗi, dịch vụ down'),
        ('gia-han-bot', 'Gia Hạn Bot', 'Báo khách hoặc dịch vụ sắp hết hạn')
) AS definition(slug, name, description)
WHERE w.deleted_at IS NULL
ON CONFLICT (workspace_id, slug) WHERE deleted_at IS NULL
DO NOTHING;

INSERT INTO bot_installations (bot_id, workspace_id, channel_id, config)
SELECT b.id, b.workspace_id, c.id, '{"system_default": true}'::jsonb
FROM bots b
JOIN (
    VALUES
        ('ticket-bot', 'ticket'),
        ('server-alert-bot', 'server-alert'),
        ('gia-han-bot', 'gia-han')
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
DO NOTHING;
