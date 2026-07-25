ALTER TABLE automation_templates
    ADD COLUMN runtime_kind text NOT NULL DEFAULT 'none'
        CHECK (runtime_kind IN ('none', 'outgoing_webhook'));

UPDATE automation_templates
SET runtime_kind = 'outgoing_webhook',
    config_schema = jsonb_set(
        config_schema,
        '{properties,event_types}',
        '{"type":"array","items":{"type":"string"}}'::jsonb,
        true
    ),
    default_config = default_config || '{"event_types":["MessageCreated"]}'::jsonb
WHERE key = 'customer-basic-webhook-bot';

ALTER TABLE automation_installations
    ADD COLUMN runtime_webhook_id uuid
        REFERENCES outgoing_webhooks (id) ON DELETE SET NULL;

CREATE UNIQUE INDEX automation_installations_runtime_webhook_uidx
    ON automation_installations (runtime_webhook_id)
    WHERE runtime_webhook_id IS NOT NULL;

-- Registry-only installations created before this migration need to be
-- recreated once so the owner receives a verifiable runtime signing secret.
UPDATE automation_installations installation
SET status = 'disabled'
FROM automation_templates template
WHERE template.id = installation.template_id
  AND template.runtime_kind = 'outgoing_webhook'
  AND installation.runtime_webhook_id IS NULL
  AND installation.deleted_at IS NULL;
