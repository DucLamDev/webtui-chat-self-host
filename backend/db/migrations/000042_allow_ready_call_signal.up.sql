ALTER TABLE call_signals
    DROP CONSTRAINT IF EXISTS call_signals_signal_type_check;

ALTER TABLE call_signals
    ADD CONSTRAINT call_signals_signal_type_check
    CHECK (signal_type IN (
        'offer',
        'answer',
        'ice_candidate',
        'ready',
        'ringing',
        'accepted',
        'rejected',
        'cancelled',
        'ended',
        'missed'
    ));
