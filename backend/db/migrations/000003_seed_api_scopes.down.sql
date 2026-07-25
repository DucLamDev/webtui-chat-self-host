DELETE FROM api_scopes
WHERE code IN (
    'message.read',
    'message.write',
    'file.write',
    'bot.write',
    'webhook.write',
    'admin.read'
);
