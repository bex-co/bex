ALTER TABLE agent_sessions DROP COLUMN IF EXISTS applied_secrets;
DROP TABLE IF EXISTS agent_secret_scopes;
