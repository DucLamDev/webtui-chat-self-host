CREATE TABLE zone_quotas (
    zone_id uuid PRIMARY KEY REFERENCES zones (id) ON DELETE CASCADE,
    max_workspaces integer NOT NULL DEFAULT 25 CHECK (max_workspaces > 0),
    max_members integer NOT NULL DEFAULT 1000 CHECK (max_members > 0),
    max_storage_bytes bigint NOT NULL DEFAULT 107374182400 CHECK (max_storage_bytes > 0),
    max_automation_installations integer NOT NULL DEFAULT 100
        CHECK (max_automation_installations > 0),
    max_webhooks integer NOT NULL DEFAULT 100 CHECK (max_webhooks > 0),
    enforcement_mode text NOT NULL DEFAULT 'hard'
        CHECK (enforcement_mode IN ('monitor', 'hard')),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TRIGGER trg_zone_quotas_updated_at
    BEFORE UPDATE ON zone_quotas
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

INSERT INTO zone_quotas (zone_id)
SELECT id
FROM zones
WHERE deleted_at IS NULL
ON CONFLICT (zone_id) DO NOTHING;

CREATE FUNCTION create_default_zone_quota() RETURNS trigger AS $$
BEGIN
    INSERT INTO zone_quotas (zone_id) VALUES (NEW.id)
    ON CONFLICT (zone_id) DO NOTHING;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_zones_default_quota
    AFTER INSERT ON zones
    FOR EACH ROW EXECUTE FUNCTION create_default_zone_quota();

CREATE TABLE zone_oidc_providers (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    zone_id uuid NOT NULL REFERENCES zones (id) ON DELETE CASCADE,
    name text NOT NULL,
    issuer_url text NOT NULL,
    client_id text NOT NULL,
    client_secret_ref text,
    scopes text[] NOT NULL DEFAULT ARRAY['openid', 'profile', 'email']::text[],
    claim_mapping jsonb NOT NULL DEFAULT
        '{"subject":"sub","email":"email","username":"preferred_username","display_name":"name","groups":"groups"}'::jsonb,
    status text NOT NULL DEFAULT 'configured'
        CHECK (status IN ('configured', 'disabled')),
    created_by uuid REFERENCES users (id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz
);

CREATE UNIQUE INDEX zone_oidc_providers_zone_name_uidx
    ON zone_oidc_providers (zone_id, lower(name))
    WHERE deleted_at IS NULL;

CREATE INDEX zone_oidc_providers_zone_status_idx
    ON zone_oidc_providers (zone_id, status)
    WHERE deleted_at IS NULL;

CREATE TRIGGER trg_zone_oidc_providers_updated_at
    BEFORE UPDATE ON zone_oidc_providers
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE FUNCTION enforce_zone_workspace_quota() RETURNS trigger AS $$
DECLARE
    mode text;
    maximum integer;
    current_count bigint;
BEGIN
    SELECT enforcement_mode, max_workspaces
    INTO mode, maximum
    FROM zone_quotas
    WHERE zone_id = NEW.zone_id
    FOR UPDATE;

    IF mode = 'hard' THEN
        SELECT count(*) INTO current_count
        FROM workspaces
        WHERE zone_id = NEW.zone_id AND deleted_at IS NULL;
        IF current_count >= maximum THEN
            RAISE EXCEPTION 'ZONE_QUOTA_EXCEEDED:workspaces' USING ERRCODE = 'P0001';
        END IF;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_workspaces_zone_quota
    BEFORE INSERT ON workspaces
    FOR EACH ROW EXECUTE FUNCTION enforce_zone_workspace_quota();

CREATE FUNCTION enforce_zone_member_quota() RETURNS trigger AS $$
DECLARE
    target_zone_id uuid;
    mode text;
    maximum integer;
    current_count bigint;
BEGIN
    IF NEW.status NOT IN ('active', 'invited') THEN
        RETURN NEW;
    END IF;
    IF TG_OP = 'UPDATE' AND OLD.status IN ('active', 'invited') THEN
        RETURN NEW;
    END IF;

    SELECT zone_id INTO target_zone_id FROM workspaces WHERE id = NEW.workspace_id;
    IF EXISTS (
        SELECT 1
        FROM workspace_members member
        JOIN workspaces workspace ON workspace.id = member.workspace_id
        WHERE workspace.zone_id = target_zone_id
          AND member.user_id = NEW.user_id
          AND member.status IN ('active', 'invited')
    ) THEN
        RETURN NEW;
    END IF;

    SELECT enforcement_mode, max_members
    INTO mode, maximum
    FROM zone_quotas
    WHERE zone_id = target_zone_id
    FOR UPDATE;
    IF mode = 'hard' THEN
        SELECT count(DISTINCT member.user_id) INTO current_count
        FROM workspace_members member
        JOIN workspaces workspace ON workspace.id = member.workspace_id
        WHERE workspace.zone_id = target_zone_id
          AND member.status IN ('active', 'invited');
        IF current_count >= maximum THEN
            RAISE EXCEPTION 'ZONE_QUOTA_EXCEEDED:members' USING ERRCODE = 'P0001';
        END IF;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_workspace_members_zone_quota
    BEFORE INSERT OR UPDATE OF status ON workspace_members
    FOR EACH ROW EXECUTE FUNCTION enforce_zone_member_quota();

CREATE FUNCTION enforce_zone_automation_quota() RETURNS trigger AS $$
DECLARE
    mode text;
    maximum integer;
    current_count bigint;
BEGIN
    SELECT enforcement_mode, max_automation_installations
    INTO mode, maximum
    FROM zone_quotas
    WHERE zone_id = NEW.zone_id
    FOR UPDATE;
    IF mode = 'hard' THEN
        SELECT count(*) INTO current_count
        FROM automation_installations
        WHERE zone_id = NEW.zone_id AND deleted_at IS NULL;
        IF current_count >= maximum THEN
            RAISE EXCEPTION 'ZONE_QUOTA_EXCEEDED:automation_installations' USING ERRCODE = 'P0001';
        END IF;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_automation_installations_zone_quota
    BEFORE INSERT ON automation_installations
    FOR EACH ROW EXECUTE FUNCTION enforce_zone_automation_quota();

CREATE FUNCTION enforce_zone_webhook_quota() RETURNS trigger AS $$
DECLARE
    target_zone_id uuid;
    mode text;
    maximum integer;
    current_count bigint;
BEGIN
    SELECT zone_id INTO target_zone_id FROM workspaces WHERE id = NEW.workspace_id;
    SELECT enforcement_mode, max_webhooks
    INTO mode, maximum
    FROM zone_quotas
    WHERE zone_id = target_zone_id
    FOR UPDATE;
    IF mode = 'hard' THEN
        SELECT count(*) INTO current_count
        FROM (
            SELECT incoming.id
            FROM incoming_webhooks incoming
            JOIN workspaces workspace ON workspace.id = incoming.workspace_id
            WHERE workspace.zone_id = target_zone_id
            UNION ALL
            SELECT outgoing.id
            FROM outgoing_webhooks outgoing
            JOIN workspaces workspace ON workspace.id = outgoing.workspace_id
            WHERE workspace.zone_id = target_zone_id
        ) webhook;
        IF current_count >= maximum THEN
            RAISE EXCEPTION 'ZONE_QUOTA_EXCEEDED:webhooks' USING ERRCODE = 'P0001';
        END IF;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_incoming_webhooks_zone_quota
    BEFORE INSERT ON incoming_webhooks
    FOR EACH ROW EXECUTE FUNCTION enforce_zone_webhook_quota();

CREATE TRIGGER trg_outgoing_webhooks_zone_quota
    BEFORE INSERT ON outgoing_webhooks
    FOR EACH ROW EXECUTE FUNCTION enforce_zone_webhook_quota();

CREATE FUNCTION zone_storage_usage(target_zone_id uuid) RETURNS bigint AS $$
    SELECT
        COALESCE((
            SELECT sum(file.byte_size)
            FROM files file
            JOIN workspaces workspace ON workspace.id = file.workspace_id
            WHERE workspace.zone_id = target_zone_id
              AND file.deleted_at IS NULL
              AND file.status <> 'deleted'
        ), 0)
        +
        COALESCE((
            SELECT sum(version.byte_size)
            FROM file_versions version
            JOIN files file ON file.id = version.file_id
            JOIN workspaces workspace ON workspace.id = file.workspace_id
            WHERE workspace.zone_id = target_zone_id
              AND file.deleted_at IS NULL
        ), 0);
$$ LANGUAGE sql STABLE;

CREATE FUNCTION enforce_zone_file_quota() RETURNS trigger AS $$
DECLARE
    target_zone_id uuid;
    mode text;
    maximum bigint;
    delta bigint;
BEGIN
    IF NEW.workspace_id IS NULL OR NEW.deleted_at IS NOT NULL OR NEW.status = 'deleted' THEN
        RETURN NEW;
    END IF;
    SELECT zone_id INTO target_zone_id FROM workspaces WHERE id = NEW.workspace_id;
    delta := NEW.byte_size;
    IF TG_OP = 'UPDATE' AND OLD.deleted_at IS NULL AND OLD.status <> 'deleted' THEN
        delta := NEW.byte_size - OLD.byte_size;
    END IF;
    IF delta <= 0 THEN
        RETURN NEW;
    END IF;
    SELECT enforcement_mode, max_storage_bytes
    INTO mode, maximum
    FROM zone_quotas
    WHERE zone_id = target_zone_id
    FOR UPDATE;
    IF mode = 'hard' AND zone_storage_usage(target_zone_id) + delta > maximum THEN
        RAISE EXCEPTION 'ZONE_QUOTA_EXCEEDED:storage_bytes' USING ERRCODE = 'P0001';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_files_zone_quota
    BEFORE INSERT OR UPDATE OF byte_size, status, deleted_at ON files
    FOR EACH ROW EXECUTE FUNCTION enforce_zone_file_quota();

CREATE FUNCTION enforce_zone_file_version_quota() RETURNS trigger AS $$
DECLARE
    target_zone_id uuid;
    mode text;
    maximum bigint;
BEGIN
    SELECT workspace.zone_id INTO target_zone_id
    FROM files file
    JOIN workspaces workspace ON workspace.id = file.workspace_id
    WHERE file.id = NEW.file_id;
    SELECT enforcement_mode, max_storage_bytes
    INTO mode, maximum
    FROM zone_quotas
    WHERE zone_id = target_zone_id
    FOR UPDATE;
    IF mode = 'hard' AND zone_storage_usage(target_zone_id) + NEW.byte_size > maximum THEN
        RAISE EXCEPTION 'ZONE_QUOTA_EXCEEDED:storage_bytes' USING ERRCODE = 'P0001';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_file_versions_zone_quota
    BEFORE INSERT ON file_versions
    FOR EACH ROW EXECUTE FUNCTION enforce_zone_file_version_quota();

COMMENT ON TABLE zone_oidc_providers IS
    'Per-zone OIDC configuration contract. Login remains unavailable until a provider verifier is deployed.';
