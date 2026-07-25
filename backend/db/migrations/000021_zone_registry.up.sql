-- Phase 0-2 foundation for domain-first customer zones.
-- A zone is the isolation boundary above workspace. VPSTTT keeps its own
-- internal zone, while customer domains resolve to separate customer zones.

CREATE TABLE zones (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    slug citext NOT NULL,
    name text NOT NULL,
    kind text NOT NULL DEFAULT 'customer_saas'
        CHECK (kind IN ('vpsttt_internal', 'customer_saas', 'customer_dedicated')),
    status text NOT NULL DEFAULT 'provisioning'
        CHECK (status IN ('provisioning', 'active', 'suspended', 'archived')),
    primary_workspace_id uuid REFERENCES workspaces (id) ON DELETE SET NULL,
    created_by uuid REFERENCES users (id) ON DELETE SET NULL,
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz
);

CREATE UNIQUE INDEX zones_slug_active_uidx ON zones (slug) WHERE deleted_at IS NULL;
CREATE INDEX zones_status_idx ON zones (status) WHERE deleted_at IS NULL;
CREATE INDEX zones_primary_workspace_idx ON zones (primary_workspace_id) WHERE primary_workspace_id IS NOT NULL;
CREATE TRIGGER trg_zones_updated_at BEFORE UPDATE ON zones FOR EACH ROW EXECUTE FUNCTION set_updated_at();

ALTER TABLE workspaces
ADD COLUMN zone_id uuid REFERENCES zones (id) ON DELETE SET NULL;

CREATE INDEX workspaces_zone_id_idx ON workspaces (zone_id) WHERE zone_id IS NOT NULL;

CREATE TABLE zone_domains (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    zone_id uuid NOT NULL REFERENCES zones (id) ON DELETE CASCADE,
    domain citext NOT NULL,
    kind text NOT NULL DEFAULT 'primary'
        CHECK (kind IN ('primary', 'alias', 'api', 'web')),
    status text NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'verified', 'active', 'suspended')),
    verification_method text NOT NULL DEFAULT 'dns_txt'
        CHECK (verification_method IN ('dns_txt', 'http_well_known', 'manual')),
    verification_token text NOT NULL DEFAULT encode(gen_random_bytes(24), 'hex'),
    verified_at timestamptz,
    tls_status text NOT NULL DEFAULT 'pending'
        CHECK (tls_status IN ('pending', 'ready', 'failed', 'disabled')),
    last_checked_at timestamptz,
    created_by uuid REFERENCES users (id) ON DELETE SET NULL,
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz
);

CREATE UNIQUE INDEX zone_domains_domain_active_uidx
    ON zone_domains (domain)
    WHERE deleted_at IS NULL AND status <> 'suspended';
CREATE INDEX zone_domains_zone_idx ON zone_domains (zone_id, status) WHERE deleted_at IS NULL;
CREATE INDEX zone_domains_verification_token_idx ON zone_domains (verification_token) WHERE deleted_at IS NULL;
CREATE TRIGGER trg_zone_domains_updated_at BEFORE UPDATE ON zone_domains FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE zone_deployments (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    zone_id uuid NOT NULL REFERENCES zones (id) ON DELETE CASCADE,
    mode text NOT NULL DEFAULT 'shared'
        CHECK (mode IN ('shared', 'dedicated_compose', 'dedicated_k8s')),
    web_base_url text NOT NULL,
    api_base_url text NOT NULL,
    ws_base_url text NOT NULL,
    admin_base_url text,
    database_mode text NOT NULL DEFAULT 'shared_schema'
        CHECK (database_mode IN ('shared_schema', 'dedicated_schema', 'dedicated_database')),
    storage_bucket text,
    redis_prefix text,
    status text NOT NULL DEFAULT 'provisioning'
        CHECK (status IN ('provisioning', 'ready', 'failed', 'suspended')),
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz
);

CREATE INDEX zone_deployments_zone_status_idx ON zone_deployments (zone_id, status, created_at DESC) WHERE deleted_at IS NULL;
CREATE TRIGGER trg_zone_deployments_updated_at BEFORE UPDATE ON zone_deployments FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE automation_templates (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    key citext NOT NULL,
    name text NOT NULL,
    description text,
    zone_kind text NOT NULL DEFAULT 'customer_saas'
        CHECK (zone_kind IN ('vpsttt_internal', 'customer_saas', 'customer_dedicated', 'any')),
    template_type text NOT NULL DEFAULT 'bot'
        CHECK (template_type IN ('bot', 'workflow', 'connector')),
    config_schema jsonb NOT NULL DEFAULT '{}'::jsonb,
    default_config jsonb NOT NULL DEFAULT '{}'::jsonb,
    required_scopes jsonb NOT NULL DEFAULT '[]'::jsonb,
    status text NOT NULL DEFAULT 'active'
        CHECK (status IN ('active', 'disabled')),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz
);

CREATE UNIQUE INDEX automation_templates_key_active_uidx ON automation_templates (key) WHERE deleted_at IS NULL;
CREATE INDEX automation_templates_zone_kind_idx ON automation_templates (zone_kind, template_type, status) WHERE deleted_at IS NULL;
CREATE TRIGGER trg_automation_templates_updated_at BEFORE UPDATE ON automation_templates FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE automation_installations (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    zone_id uuid NOT NULL REFERENCES zones (id) ON DELETE CASCADE,
    workspace_id uuid REFERENCES workspaces (id) ON DELETE CASCADE,
    template_id uuid REFERENCES automation_templates (id) ON DELETE SET NULL,
    name text NOT NULL,
    status text NOT NULL DEFAULT 'disabled'
        CHECK (status IN ('enabled', 'disabled', 'failed')),
    config jsonb NOT NULL DEFAULT '{}'::jsonb,
    secret_ref text,
    installed_by uuid REFERENCES users (id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz
);

CREATE INDEX automation_installations_zone_idx ON automation_installations (zone_id, status) WHERE deleted_at IS NULL;
CREATE INDEX automation_installations_workspace_idx ON automation_installations (workspace_id, status) WHERE workspace_id IS NOT NULL AND deleted_at IS NULL;
CREATE TRIGGER trg_automation_installations_updated_at BEFORE UPDATE ON automation_installations FOR EACH ROW EXECUTE FUNCTION set_updated_at();

WITH default_workspace AS (
    SELECT id
    FROM workspaces
    WHERE deleted_at IS NULL
    ORDER BY
        CASE WHEN slug::text = 'vpsttt' THEN 0 ELSE 1 END,
        created_at ASC
    LIMIT 1
)
INSERT INTO zones (slug, name, kind, status, primary_workspace_id, metadata)
VALUES (
    'vpsttt',
    'VPSTTT Internal',
    'vpsttt_internal',
    'active',
    (SELECT id FROM default_workspace),
    '{"template_key":"vpsttt_services","capabilities":{"chat":true,"files":true,"calls":true,"bots":true,"automation":true,"webhooks":true,"federation":false,"sso":false}}'::jsonb
)
ON CONFLICT (slug) WHERE deleted_at IS NULL DO NOTHING;

UPDATE workspaces
SET zone_id = z.id
FROM zones z
WHERE z.slug = 'vpsttt'
  AND z.deleted_at IS NULL
  AND workspaces.id = z.primary_workspace_id
  AND workspaces.zone_id IS NULL;

INSERT INTO zone_domains (zone_id, domain, kind, status, verification_method, verification_token, verified_at, tls_status, metadata)
SELECT
    z.id,
    'chat.vpsttt.com',
    'primary',
    'active',
    'manual',
    'vpsttt-internal-manual',
    now(),
    'ready',
    '{"managed_by":"vpsttt_control_plane"}'::jsonb
FROM zones z
WHERE z.slug = 'vpsttt'
  AND z.deleted_at IS NULL
ON CONFLICT (domain) WHERE deleted_at IS NULL AND status <> 'suspended' DO NOTHING;

INSERT INTO zone_deployments (zone_id, mode, web_base_url, api_base_url, ws_base_url, admin_base_url, database_mode, storage_bucket, redis_prefix, status, metadata)
SELECT
    z.id,
    'shared',
    'https://chat.vpsttt.com',
    'https://chat.vpsttt.com',
    'wss://chat.vpsttt.com/ws',
    'https://chat.vpsttt.com/admin',
    'shared_schema',
    'webtui-chat',
    'zone:vpsttt',
    'ready',
    '{"deployment":"current-production"}'::jsonb
FROM zones z
WHERE z.slug = 'vpsttt'
  AND z.deleted_at IS NULL;

INSERT INTO automation_templates (key, name, description, zone_kind, template_type, config_schema, default_config, required_scopes)
VALUES
    (
        'customer-basic-webhook-bot',
        'Customer Webhook Bot',
        'Bot mau cho khach hang ket noi webhook/API rieng theo tung zone.',
        'any',
        'bot',
        '{"type":"object","required":["endpoint_url","channel_slug"],"properties":{"endpoint_url":{"type":"string","format":"uri"},"channel_slug":{"type":"string"},"signature_header":{"type":"string","default":"X-VPSTTT-Signature"}}}'::jsonb,
        '{"signature_header":"X-VPSTTT-Signature"}'::jsonb,
        '["message.send","webhook.manage"]'::jsonb
    ),
    (
        'vpsttt-services-automation',
        'VPSTTT Services Automation',
        'Template noi bo VPSTTT cho VPS, proxy, hosting, domain, gia han va ticket.',
        'vpsttt_internal',
        'workflow',
        '{"type":"object","required":["order_api_base_url"],"properties":{"order_api_base_url":{"type":"string","format":"uri"},"service_types":{"type":"array","items":{"type":"string"}}}}'::jsonb,
        '{"service_types":["vps","proxy","hosting","domain","s3","waf"]}'::jsonb,
        '["order.view","order.billing","bot.manage"]'::jsonb
    )
ON CONFLICT (key) WHERE deleted_at IS NULL DO NOTHING;
