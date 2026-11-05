DROP INDEX IF EXISTS agent_model_usage_events_session_idx;
ALTER TABLE agent_model_usage_events DROP COLUMN IF EXISTS started_at;
