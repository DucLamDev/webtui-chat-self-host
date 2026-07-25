DROP TABLE IF EXISTS zone_deployment_requests;
DROP INDEX IF EXISTS zone_domains_one_primary_uidx;

ALTER TABLE zones
    DROP COLUMN IF EXISTS lifecycle_reason,
    DROP COLUMN IF EXISTS archived_at,
    DROP COLUMN IF EXISTS suspended_at;
