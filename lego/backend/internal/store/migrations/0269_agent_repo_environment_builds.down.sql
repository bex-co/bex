ALTER TABLE agent_sessions DROP COLUMN IF EXISTS environment_build_id;
DROP TABLE IF EXISTS agent_repo_environment_builds;
ALTER TABLE agent_repo_environments DROP COLUMN IF EXISTS version;
