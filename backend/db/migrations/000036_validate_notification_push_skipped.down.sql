-- PostgreSQL cannot mark a validated constraint NOT VALID again. Validation
-- is safe to retain while the schema migration version is rolled back.
SELECT 1;
