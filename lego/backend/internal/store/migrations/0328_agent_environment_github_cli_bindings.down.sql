ALTER TABLE agent_environments
    DROP COLUMN IF EXISTS github_cli_mandatory,
    DROP COLUMN IF EXISTS automations_github_cli_enabled;
