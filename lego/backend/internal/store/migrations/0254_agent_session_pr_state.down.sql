DROP INDEX IF EXISTS agent_sessions_pr_state_list_idx;
DROP INDEX IF EXISTS agent_sessions_pr_due_idx;
ALTER TABLE agent_sessions
    DROP COLUMN IF EXISTS pr_next_check_at,
    DROP COLUMN IF EXISTS pr_state_at,
    DROP COLUMN IF EXISTS pr_state;
