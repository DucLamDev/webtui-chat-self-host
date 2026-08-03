DROP TRIGGER IF EXISTS trg_push_relay_jobs_updated_at ON push_relay_jobs;
DROP TABLE IF EXISTS push_relay_jobs;

DROP TABLE IF EXISTS notification_web_push_deliveries;

DROP TRIGGER IF EXISTS trg_web_push_subscriptions_updated_at ON web_push_subscriptions;
DROP TABLE IF EXISTS web_push_subscriptions;
