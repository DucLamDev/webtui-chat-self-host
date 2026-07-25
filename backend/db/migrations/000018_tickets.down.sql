DELETE FROM role_permissions
WHERE permission_id IN (
    SELECT id FROM permissions WHERE code IN ('ticket.view', 'ticket.manage')
);

DELETE FROM permissions WHERE code IN ('ticket.view', 'ticket.manage');

DROP TRIGGER IF EXISTS trg_tickets_updated_at ON tickets;
DROP TABLE IF EXISTS tickets;
