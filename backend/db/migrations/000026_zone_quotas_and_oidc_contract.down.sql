DROP TRIGGER IF EXISTS trg_file_versions_zone_quota ON file_versions;
DROP FUNCTION IF EXISTS enforce_zone_file_version_quota();
DROP TRIGGER IF EXISTS trg_files_zone_quota ON files;
DROP FUNCTION IF EXISTS enforce_zone_file_quota();
DROP FUNCTION IF EXISTS zone_storage_usage(uuid);

DROP TRIGGER IF EXISTS trg_outgoing_webhooks_zone_quota ON outgoing_webhooks;
DROP TRIGGER IF EXISTS trg_incoming_webhooks_zone_quota ON incoming_webhooks;
DROP FUNCTION IF EXISTS enforce_zone_webhook_quota();

DROP TRIGGER IF EXISTS trg_automation_installations_zone_quota ON automation_installations;
DROP FUNCTION IF EXISTS enforce_zone_automation_quota();

DROP TRIGGER IF EXISTS trg_workspace_members_zone_quota ON workspace_members;
DROP FUNCTION IF EXISTS enforce_zone_member_quota();

DROP TRIGGER IF EXISTS trg_workspaces_zone_quota ON workspaces;
DROP FUNCTION IF EXISTS enforce_zone_workspace_quota();

DROP TRIGGER IF EXISTS trg_zone_oidc_providers_updated_at ON zone_oidc_providers;
DROP TABLE IF EXISTS zone_oidc_providers;

DROP TRIGGER IF EXISTS trg_zones_default_quota ON zones;
DROP FUNCTION IF EXISTS create_default_zone_quota();

DROP TRIGGER IF EXISTS trg_zone_quotas_updated_at ON zone_quotas;
DROP TABLE IF EXISTS zone_quotas;
