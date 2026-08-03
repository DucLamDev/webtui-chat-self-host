CREATE OR REPLACE FUNCTION purge_terminal_call_signals()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    IF NEW.status IN ('rejected', 'cancelled', 'ended', 'missed')
       AND OLD.status IS DISTINCT FROM NEW.status THEN
        DELETE FROM call_signals WHERE call_id = NEW.id;
    END IF;
    RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS trg_call_sessions_purge_signals ON call_sessions;
CREATE TRIGGER trg_call_sessions_purge_signals
AFTER UPDATE OF status ON call_sessions
FOR EACH ROW EXECUTE FUNCTION purge_terminal_call_signals();

DELETE FROM call_signals signal
USING call_sessions call
WHERE signal.call_id = call.id
  AND (call.status IN ('rejected', 'cancelled', 'ended', 'missed')
       OR signal.created_at < now() - interval '24 hours');

CREATE INDEX IF NOT EXISTS call_signals_created_at_idx
ON call_signals (created_at);
