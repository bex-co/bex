-- One automatic lease generation follows a sync through admission and every
-- later mutation boundary. The source snapshot is the resolved source, while
-- the accepted intent's original commit remains unchanged for provenance.
ALTER TABLE blueprint_auto_sync_intents
    ADD COLUMN claim_generation bigint NOT NULL DEFAULT 0;

ALTER TABLE blueprint_syncs
    ADD COLUMN auto_intent_id text REFERENCES blueprint_auto_sync_intents (id),
    ADD COLUMN auto_claim_generation bigint NOT NULL DEFAULT 0,
    ADD COLUMN auto_source_repo text NOT NULL DEFAULT '',
    ADD COLUMN auto_source_branch text NOT NULL DEFAULT '',
    ADD COLUMN auto_source_path text NOT NULL DEFAULT '';

CREATE INDEX blueprint_auto_sync_intents_blueprint_pending_idx
    ON blueprint_auto_sync_intents (blueprint_id, created_at, id)
    WHERE state IN ('pending', 'claimed');
