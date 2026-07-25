DROP INDEX IF EXISTS users_device_name_idx;
DROP INDEX IF EXISTS users_last_ip_address_idx;
DROP INDEX IF EXISTS users_registration_ip_address_idx;

ALTER TABLE users
    DROP COLUMN IF EXISTS last_user_agent,
    DROP COLUMN IF EXISTS device_name,
    DROP COLUMN IF EXISTS last_ip_address,
    DROP COLUMN IF EXISTS registration_user_agent,
    DROP COLUMN IF EXISTS registration_device_name,
    DROP COLUMN IF EXISTS registration_ip_address;
