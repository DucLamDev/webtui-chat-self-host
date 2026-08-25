DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conrelid = 'call_sessions'::regclass
          AND conname = 'call_sessions_check'
    ) AND NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conrelid = 'call_sessions'::regclass
          AND conname = 'call_sessions_distinct_participants_check'
    ) THEN
        ALTER TABLE call_sessions
            RENAME CONSTRAINT call_sessions_check TO call_sessions_distinct_participants_check;
    END IF;
END $$;
