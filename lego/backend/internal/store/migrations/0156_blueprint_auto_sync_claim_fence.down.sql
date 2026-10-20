DROP INDEX IF EXISTS blueprint_auto_sync_intents_blueprint_pending_idx;
ALTER TABLE blueprint_syncs
    DROP COLUMN auto_source_path,
    DROP COLUMN auto_source_branch,
    DROP COLUMN auto_source_repo,
    DROP COLUMN auto_claim_generation,
    DROP COLUMN auto_intent_id;
ALTER TABLE blueprint_auto_sync_intents DROP COLUMN claim_generation;
