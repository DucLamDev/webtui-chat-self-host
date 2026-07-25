-- Phase 2-4: enforce the zone boundary, track domain verification, and bind
-- authentication sessions to one zone/workspace context.

ALTER TABLE zones
ADD COLUMN registration_mode text NOT NULL DEFAULT 'invite_only'
    CHECK (registration_mode IN ('open', 'invite_only', 'closed'));

UPDATE zones
SET registration_mode = 'open'
WHERE kind = 'vpsttt_internal';

UPDATE workspaces
SET zone_id = vpsttt.id
FROM zones vpsttt
WHERE vpsttt.slug = 'vpsttt'
  AND vpsttt.deleted_at IS NULL
  AND workspaces.zone_id IS NULL;

ALTER TABLE workspaces
ALTER COLUMN zone_id SET NOT NULL;

ALTER TABLE workspaces
ADD CONSTRAINT workspaces_id_zone_unique UNIQUE (id, zone_id);

ALTER TABLE contact_requests
ADD COLUMN zone_id uuid REFERENCES zones (id) ON DELETE CASCADE;

UPDATE contact_requests
SET zone_id = vpsttt.id
FROM zones vpsttt
WHERE vpsttt.slug = 'vpsttt'
  AND vpsttt.deleted_at IS NULL
  AND contact_requests.zone_id IS NULL;

ALTER TABLE contact_requests
ALTER COLUMN zone_id SET NOT NULL;

DROP INDEX contact_requests_pair_uidx;
CREATE UNIQUE INDEX contact_requests_zone_pair_uidx
ON contact_requests (
    zone_id,
    LEAST(requester_id, receiver_id),
    GREATEST(requester_id, receiver_id)
)
WHERE deleted_at IS NULL AND status IN ('pending', 'accepted');

CREATE INDEX contact_requests_zone_requester_idx
ON contact_requests (zone_id, requester_id, status, updated_at DESC);
CREATE INDEX contact_requests_zone_receiver_idx
ON contact_requests (zone_id, receiver_id, status, updated_at DESC);

ALTER TABLE push_devices
ADD COLUMN zone_id uuid REFERENCES zones (id) ON DELETE CASCADE;

UPDATE push_devices device
SET zone_id = workspace.zone_id
FROM workspaces workspace
WHERE workspace.id = device.workspace_id
  AND device.zone_id IS NULL;

WITH first_membership AS (
    SELECT DISTINCT ON (device.id)
        device.id AS device_id,
        workspace.zone_id
    FROM push_devices device
    JOIN workspace_members member
      ON member.user_id = device.user_id
     AND member.status = 'active'
    JOIN workspaces workspace
      ON workspace.id = member.workspace_id
     AND workspace.status = 'active'
     AND workspace.deleted_at IS NULL
    WHERE device.zone_id IS NULL
    ORDER BY device.id, member.joined_at NULLS LAST, member.created_at, workspace.id
)
UPDATE push_devices device
SET zone_id = first_membership.zone_id
FROM first_membership
WHERE first_membership.device_id = device.id;

UPDATE push_devices device
SET zone_id = vpsttt.id
FROM zones vpsttt
WHERE vpsttt.slug = 'vpsttt'
  AND vpsttt.deleted_at IS NULL
  AND device.zone_id IS NULL;

ALTER TABLE push_devices
ALTER COLUMN zone_id SET NOT NULL;

ALTER TABLE push_devices
DROP CONSTRAINT push_devices_user_id_device_id_key;
ALTER TABLE push_devices
ADD CONSTRAINT push_devices_zone_user_device_unique
UNIQUE (zone_id, user_id, device_id);
ALTER TABLE push_devices
ADD CONSTRAINT push_devices_workspace_zone_fk
FOREIGN KEY (workspace_id, zone_id)
REFERENCES workspaces (id, zone_id)
ON DELETE CASCADE;

CREATE INDEX push_devices_zone_user_status_idx
ON push_devices (zone_id, user_id, status, updated_at DESC);

ALTER TABLE zone_domains
ADD COLUMN verification_expires_at timestamptz,
ADD COLUMN verification_attempts integer NOT NULL DEFAULT 0 CHECK (verification_attempts >= 0),
ADD COLUMN last_verification_error text;

UPDATE zone_domains
SET verification_expires_at = created_at + interval '24 hours'
WHERE status = 'pending'
  AND verification_expires_at IS NULL;

CREATE INDEX zone_domains_pending_verification_idx
    ON zone_domains (verification_expires_at, created_at)
    WHERE deleted_at IS NULL AND status = 'pending';

ALTER TABLE user_sessions
ADD COLUMN zone_id uuid REFERENCES zones (id) ON DELETE CASCADE,
ADD COLUMN workspace_id uuid REFERENCES workspaces (id) ON DELETE CASCADE,
ADD COLUMN domain citext;

WITH session_workspace AS (
    SELECT DISTINCT ON (us.id)
        us.id AS session_id,
        w.zone_id,
        w.id AS workspace_id
    FROM user_sessions us
    JOIN workspace_members wm
      ON wm.user_id = us.user_id
     AND wm.status = 'active'
    JOIN workspaces w
      ON w.id = wm.workspace_id
     AND w.status = 'active'
     AND w.deleted_at IS NULL
    ORDER BY us.id, wm.joined_at NULLS LAST, wm.created_at, w.id
)
UPDATE user_sessions us
SET zone_id = session_workspace.zone_id,
    workspace_id = session_workspace.workspace_id
FROM session_workspace
WHERE session_workspace.session_id = us.id;

UPDATE user_sessions session
SET domain = (
    SELECT domain.domain
    FROM zone_domains domain
    WHERE domain.zone_id = session.zone_id
      AND domain.deleted_at IS NULL
      AND domain.status IN ('verified', 'active')
    ORDER BY
        CASE domain.kind WHEN 'primary' THEN 0 ELSE 1 END,
        CASE domain.status WHEN 'active' THEN 0 ELSE 1 END,
        domain.created_at
    LIMIT 1
)
WHERE session.domain IS NULL
  AND session.zone_id IS NOT NULL;

UPDATE user_sessions
SET revoked_at = COALESCE(revoked_at, now())
WHERE zone_id IS NULL OR workspace_id IS NULL OR domain IS NULL;

CREATE INDEX user_sessions_zone_user_active_idx
    ON user_sessions (zone_id, user_id, expires_at)
    WHERE revoked_at IS NULL;
CREATE INDEX user_sessions_workspace_user_idx
    ON user_sessions (workspace_id, user_id, created_at DESC);

ALTER TABLE audit_logs
ADD COLUMN zone_id uuid REFERENCES zones (id) ON DELETE SET NULL;

UPDATE audit_logs audit
SET zone_id = workspace.zone_id
FROM workspaces workspace
WHERE workspace.id = audit.workspace_id
  AND audit.zone_id IS NULL;

CREATE OR REPLACE FUNCTION populate_audit_log_zone()
RETURNS trigger AS $$
BEGIN
    IF NEW.zone_id IS NULL AND NEW.workspace_id IS NOT NULL THEN
        SELECT workspace.zone_id
        INTO NEW.zone_id
        FROM workspaces workspace
        WHERE workspace.id = NEW.workspace_id;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_audit_logs_zone
BEFORE INSERT OR UPDATE OF workspace_id ON audit_logs
FOR EACH ROW
EXECUTE FUNCTION populate_audit_log_zone();

CREATE INDEX audit_logs_zone_created_idx
    ON audit_logs (zone_id, created_at DESC)
    WHERE zone_id IS NOT NULL;

ALTER TABLE automation_installations
ADD CONSTRAINT automation_installations_workspace_zone_fk
FOREIGN KEY (workspace_id, zone_id)
REFERENCES workspaces (id, zone_id)
ON DELETE CASCADE;

CREATE UNIQUE INDEX automation_installations_zone_name_active_uidx
    ON automation_installations (zone_id, lower(name))
    WHERE deleted_at IS NULL;

CREATE OR REPLACE FUNCTION validate_zone_primary_workspace()
RETURNS trigger AS $$
BEGIN
    IF NEW.primary_workspace_id IS NOT NULL AND NOT EXISTS (
        SELECT 1
        FROM workspaces workspace
        WHERE workspace.id = NEW.primary_workspace_id
          AND workspace.zone_id = NEW.id
          AND workspace.deleted_at IS NULL
    ) THEN
        RAISE EXCEPTION 'primary workspace must belong to the same zone'
            USING ERRCODE = '23514';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE CONSTRAINT TRIGGER trg_zones_primary_workspace_guard
AFTER INSERT OR UPDATE OF primary_workspace_id ON zones
DEFERRABLE INITIALLY DEFERRED
FOR EACH ROW
EXECUTE FUNCTION validate_zone_primary_workspace();
