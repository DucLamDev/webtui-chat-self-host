CREATE TABLE IF NOT EXISTS notification_preferences (
    user_id uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    workspace_id uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    mode text NOT NULL DEFAULT 'all' CHECK (mode IN ('all', 'mentions', 'muted')),
    preview boolean NOT NULL DEFAULT true,
    quiet_hours boolean NOT NULL DEFAULT false,
    quiet_start char(5) NOT NULL DEFAULT '22:00' CHECK (quiet_start ~ '^([01][0-9]|2[0-3]):[0-5][0-9]$'),
    quiet_end char(5) NOT NULL DEFAULT '07:00' CHECK (quiet_end ~ '^([01][0-9]|2[0-3]):[0-5][0-9]$'),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, workspace_id)
);

CREATE INDEX IF NOT EXISTS notification_preferences_workspace_idx
ON notification_preferences (workspace_id, updated_at DESC);

CREATE TRIGGER trg_notification_preferences_updated_at
BEFORE UPDATE ON notification_preferences
FOR EACH ROW EXECUTE FUNCTION set_updated_at();
