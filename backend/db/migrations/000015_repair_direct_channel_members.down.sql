-- Forward-only data repair. Rolling this back would remove legitimate direct
-- channel memberships, so the down migration is intentionally a no-op.
SELECT 1;
