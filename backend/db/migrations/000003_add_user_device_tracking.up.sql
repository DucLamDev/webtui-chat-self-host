-- Lưu thông tin thiết bị và IP đăng ký/đăng nhập gần nhất của user.

ALTER TABLE users
    ADD COLUMN registration_ip_address inet,
    ADD COLUMN registration_device_name text,
    ADD COLUMN registration_user_agent text,
    ADD COLUMN last_ip_address inet,
    ADD COLUMN device_name text,
    ADD COLUMN last_user_agent text;

CREATE INDEX users_registration_ip_address_idx ON users (registration_ip_address) WHERE registration_ip_address IS NOT NULL;
CREATE INDEX users_last_ip_address_idx ON users (last_ip_address) WHERE last_ip_address IS NOT NULL;
CREATE INDEX users_device_name_idx ON users (device_name) WHERE device_name IS NOT NULL;
