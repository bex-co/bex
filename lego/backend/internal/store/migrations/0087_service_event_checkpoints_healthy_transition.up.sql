-- w6/m41: the reconciler's stale-conclusion guard (rejectStaleUnhealthy)
-- orders an unhealthy edge against the Ready=True LastTransitionTime recorded
-- with the service's current healthy checkpoint. Both timestamps come from the
-- operator's own condition clock, so the comparison carries no cross-process
-- clock skew (which updated_at, stamped by the reconciler, would). NULL =
-- unknown (pre-migration rows, or a healthy checkpoint recorded from a
-- condition without a timestamp): the guard cannot order and must fail open
-- toward recording real outages, never toward silence.
ALTER TABLE service_event_checkpoints
    ADD COLUMN healthy_transition_at TIMESTAMPTZ;
