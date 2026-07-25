DROP TRIGGER IF EXISTS trg_zones_primary_workspace_guard ON zones;
DROP FUNCTION IF EXISTS validate_zone_primary_workspace();

DROP INDEX IF EXISTS automation_installations_zone_name_active_uidx;
ALTER TABLE automation_installations
DROP CONSTRAINT IF EXISTS automation_installations_workspace_zone_fk;

DROP INDEX IF EXISTS audit_logs_zone_created_idx;
DROP TRIGGER IF EXISTS trg_audit_logs_zone ON audit_logs;
DROP FUNCTION IF EXISTS populate_audit_log_zone();
ALTER TABLE audit_logs
DROP COLUMN IF EXISTS zone_id;

DROP INDEX IF EXISTS user_sessions_workspace_user_idx;
DROP INDEX IF EXISTS user_sessions_zone_user_active_idx;
ALTER TABLE user_sessions
DROP COLUMN IF EXISTS domain,
DROP COLUMN IF EXISTS workspace_id,
DROP COLUMN IF EXISTS zone_id;

DROP INDEX IF EXISTS zone_domains_pending_verification_idx;
ALTER TABLE zone_domains
DROP COLUMN IF EXISTS last_verification_error,
DROP COLUMN IF EXISTS verification_attempts,
DROP COLUMN IF EXISTS verification_expires_at;

DROP INDEX IF EXISTS push_devices_zone_user_status_idx;
ALTER TABLE push_devices
DROP CONSTRAINT IF EXISTS push_devices_workspace_zone_fk;
ALTER TABLE push_devices
DROP CONSTRAINT IF EXISTS push_devices_zone_user_device_unique;
ALTER TABLE push_devices
ADD CONSTRAINT push_devices_user_id_device_id_key UNIQUE (user_id, device_id);
ALTER TABLE push_devices
DROP COLUMN IF EXISTS zone_id;

ALTER TABLE workspaces
DROP CONSTRAINT IF EXISTS workspaces_id_zone_unique;
ALTER TABLE workspaces
ALTER COLUMN zone_id DROP NOT NULL;

DROP INDEX IF EXISTS contact_requests_zone_receiver_idx;
DROP INDEX IF EXISTS contact_requests_zone_requester_idx;
DROP INDEX IF EXISTS contact_requests_zone_pair_uidx;
CREATE UNIQUE INDEX contact_requests_pair_uidx
ON contact_requests (LEAST(requester_id, receiver_id), GREATEST(requester_id, receiver_id))
WHERE deleted_at IS NULL AND status IN ('pending', 'accepted');
ALTER TABLE contact_requests
DROP COLUMN IF EXISTS zone_id;

ALTER TABLE zones
DROP COLUMN IF EXISTS registration_mode;
