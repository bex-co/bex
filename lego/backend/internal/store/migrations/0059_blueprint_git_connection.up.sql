-- 0059: Git-connected Blueprints (w2/m62) — add path/auto_sync/last_sync_at to
-- blueprints; add blueprint_syncs for per-run history. Legacy 'active' rows read
-- as 'in_sync' at the service layer (no data migration needed).
ALTER TABLE blueprints
    ADD COLUMN path         text        NOT NULL DEFAULT 'bex.yml',
    ADD COLUMN auto_sync    boolean     NOT NULL DEFAULT true,
    ADD COLUMN last_sync_at timestamptz;

CREATE TABLE blueprint_syncs (
    id           text        PRIMARY KEY,
    blueprint_id text        NOT NULL REFERENCES blueprints(id) ON DELETE CASCADE,
    commit_id    text        NOT NULL DEFAULT '',
    state        text        NOT NULL DEFAULT 'created',
    started_at   timestamptz NOT NULL DEFAULT now(),
    completed_at timestamptz,
    created_at   timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX blueprint_syncs_blueprint_id_started ON blueprint_syncs (blueprint_id, started_at DESC);
