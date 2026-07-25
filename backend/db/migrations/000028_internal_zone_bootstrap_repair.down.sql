-- Preserve a workspace once it has members. An unused workspace created by the
-- repair can be removed safely when rolling the migration back.
DO $$
DECLARE
    bootstrap_workspace_id uuid;
    internal_zone_id uuid;
BEGIN
    SELECT zone.id, workspace.id
    INTO internal_zone_id, bootstrap_workspace_id
    FROM zones zone
    JOIN workspaces workspace
      ON workspace.id = zone.primary_workspace_id
     AND workspace.zone_id = zone.id
    WHERE zone.slug = 'vpsttt'
      AND zone.kind = 'vpsttt_internal'
      AND workspace.description = 'Bootstrap workspace for the VPSTTT internal zone'
      AND workspace.owner_id IS NULL
      AND NOT EXISTS (
          SELECT 1
          FROM workspace_members member
          WHERE member.workspace_id = workspace.id
      )
    LIMIT 1;

    IF bootstrap_workspace_id IS NULL THEN
        RETURN;
    END IF;

    UPDATE zones
    SET primary_workspace_id = NULL
    WHERE id = internal_zone_id
      AND primary_workspace_id = bootstrap_workspace_id;

    DELETE FROM workspaces
    WHERE id = bootstrap_workspace_id;
END;
$$;
