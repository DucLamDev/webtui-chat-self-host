DROP TABLE IF EXISTS automation_installations;
DROP TABLE IF EXISTS automation_templates;
DROP TABLE IF EXISTS zone_deployments;
DROP TABLE IF EXISTS zone_domains;

ALTER TABLE workspaces
DROP COLUMN IF EXISTS zone_id;

DROP TABLE IF EXISTS zones;
