ALTER TABLE agent_sessions
    DROP COLUMN IF EXISTS pr_merge_queued,
    DROP COLUMN IF EXISTS pr_auto_merge;
