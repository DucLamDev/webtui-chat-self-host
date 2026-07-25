DELETE FROM bot_installations bi
USING bots b
WHERE bi.bot_id = b.id
  AND b.slug::text IN ('cskh-bot', 'thanh-toan-bot')
  AND bi.config @> '{"phase": "order_bot_phase1"}'::jsonb;

DELETE FROM bots
WHERE slug::text IN ('cskh-bot', 'thanh-toan-bot')
  AND settings @> '{"phase": "order_bot_phase1"}'::jsonb;

DELETE FROM role_permissions rp
USING permissions p
WHERE rp.permission_id = p.id
  AND p.code IN ('order.view', 'order.billing');

DELETE FROM permissions
WHERE code IN ('order.view', 'order.billing');
