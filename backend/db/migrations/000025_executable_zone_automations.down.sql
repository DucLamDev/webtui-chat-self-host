DROP INDEX IF EXISTS automation_installations_runtime_webhook_uidx;

ALTER TABLE automation_installations
    DROP COLUMN IF EXISTS runtime_webhook_id;

UPDATE automation_templates
SET config_schema = config_schema #- '{properties,event_types}',
    default_config = default_config - 'event_types'
WHERE key = 'customer-basic-webhook-bot';

ALTER TABLE automation_templates
    DROP COLUMN IF EXISTS runtime_kind;
