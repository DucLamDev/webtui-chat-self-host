DROP INDEX IF EXISTS users_phone_number_search_idx;
DROP INDEX IF EXISTS users_phone_number_active_uidx;

ALTER TABLE users
DROP COLUMN IF EXISTS phone_number;
