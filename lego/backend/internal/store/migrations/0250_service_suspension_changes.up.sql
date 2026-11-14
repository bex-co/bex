-- w21/065: every accepted Suspend and Resume, in order, until the event
-- projector has replayed it. A service's service_suspended/service_resumed
-- facts come from the projector comparing what it samples to the event
-- checkpoint, so two actions closer together than one pass collapsed into one
-- sample and lost events (six runs 5 s apart gave 4 of 12). The verbs append
-- here after their audit row; RecordObservedServiceState replays the rows in
-- seq order inside the checkpoint's own transaction and deletes them, so the
-- table holds only what has not been reported yet.
CREATE TABLE IF NOT EXISTS service_suspension_changes (
    seq BIGSERIAL PRIMARY KEY,
    app_id TEXT NOT NULL REFERENCES apps(id) ON DELETE CASCADE,
    suspended BOOLEAN NOT NULL,
    at TIMESTAMPTZ NOT NULL
);
CREATE INDEX IF NOT EXISTS service_suspension_changes_app_idx
    ON service_suspension_changes (app_id, seq);
