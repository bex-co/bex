-- w8/m37: persisted execution fencing for Blueprint sync admission (t002),
-- disconnect coordination (t003), and abandoned-run recovery (t004).
--
-- execution_generation on blueprints is the fencing counter: every admission,
-- disconnect, and explicit re-creation bumps it, so a worker admitted under an
-- older generation cannot complete, restamp ownership, or resurrect the row.
-- active_run_id names the one admitted-but-uncompleted run that owns the
-- apply; admission claims it atomically and completion/recovery/disconnect
-- clears it. blueprint_syncs.execution_generation records which admission a
-- run belongs to, so recovery never settles a run against a newer generation.
ALTER TABLE blueprints
    ADD COLUMN execution_generation BIGINT NOT NULL DEFAULT 1,
    ADD COLUMN active_run_id TEXT NULL;
ALTER TABLE blueprint_syncs
    ADD COLUMN execution_generation BIGINT NOT NULL DEFAULT 1;
-- Recovery sweep: stale running runs, oldest first (t004, bounded per tick).
CREATE INDEX blueprint_syncs_recovery_idx
    ON blueprint_syncs (state, started_at)
    WHERE state = 'running';
