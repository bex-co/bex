DROP INDEX IF EXISTS agent_sessions_automation_idx;
ALTER TABLE agent_sessions DROP COLUMN IF EXISTS automation_id;
DROP TABLE IF EXISTS agent_automation_runs;
DROP TABLE IF EXISTS agent_automations;
