DROP INDEX IF EXISTS call_signals_created_at_idx;
DROP TRIGGER IF EXISTS trg_call_sessions_purge_signals ON call_sessions;
DROP FUNCTION IF EXISTS purge_terminal_call_signals();
