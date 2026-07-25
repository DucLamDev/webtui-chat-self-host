ALTER TABLE users
ADD COLUMN IF NOT EXISTS phone_number text;

CREATE UNIQUE INDEX IF NOT EXISTS users_phone_number_active_uidx
ON users (phone_number)
WHERE deleted_at IS NULL
  AND phone_number IS NOT NULL
  AND phone_number <> '';

CREATE INDEX IF NOT EXISTS users_phone_number_search_idx
ON users (phone_number)
WHERE deleted_at IS NULL
  AND phone_number IS NOT NULL
  AND phone_number <> '';
