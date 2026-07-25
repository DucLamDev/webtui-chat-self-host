-- Repair installations where migration 000021 created the internal zone before
-- any workspace existed. A domain without a primary workspace can be
-- discovered, but no password, Google, or OIDC session can enter it.

DO $$
DECLARE
    internal_zone_id uuid;
    internal_workspace_id uuid;
    internal_workspace_slug text;
BEGIN
    SELECT id
    INTO internal_zone_id
    FROM zones
    WHERE slug = 'vpsttt'
      AND kind = 'vpsttt_internal'
      AND deleted_at IS NULL
    ORDER BY created_at
    LIMIT 1;

    IF internal_zone_id IS NULL THEN
        RETURN;
    END IF;

    SELECT id
    INTO internal_workspace_id
    FROM workspaces
    WHERE zone_id = internal_zone_id
      AND status = 'active'
      AND deleted_at IS NULL
    ORDER BY
        CASE WHEN slug = 'vpsttt' THEN 0 ELSE 1 END,
        created_at,
        id
    LIMIT 1;

    IF internal_workspace_id IS NULL THEN
        internal_workspace_slug := 'vpsttt';
        IF EXISTS (
            SELECT 1
            FROM workspaces
            WHERE slug = internal_workspace_slug
              AND deleted_at IS NULL
        ) THEN
            internal_workspace_slug :=
                'vpsttt-' || substring(replace(internal_zone_id::text, '-', '') FROM 1 FOR 8);
        END IF;

        INSERT INTO workspaces (
            slug, name, description, plan, status, zone_id
        )
        VALUES (
            internal_workspace_slug,
            'VPSTTT Internal',
            'Bootstrap workspace for the VPSTTT internal zone',
            'self_hosted',
            'active',
            internal_zone_id
        )
        RETURNING id INTO internal_workspace_id;
    END IF;

    UPDATE zones
    SET primary_workspace_id = internal_workspace_id
    WHERE id = internal_zone_id
      AND primary_workspace_id IS NULL;

    INSERT INTO channels (
        workspace_id, slug, name, description, type, settings
    )
    VALUES
        (
            internal_workspace_id,
            'general',
            'General',
            'Trao doi chung trong workspace',
            'public',
            '{"system_default":true,"template_key":"vpsttt_services"}'::jsonb
        ),
        (
            internal_workspace_id,
            'announcements',
            'Announcements',
            'Thong bao noi bo cua workspace',
            'public',
            '{"system_default":true,"template_key":"vpsttt_services"}'::jsonb
        )
    ON CONFLICT (workspace_id, slug)
        WHERE slug IS NOT NULL AND deleted_at IS NULL
        DO NOTHING;
END;
$$;
