-- w10/m519: the workspace's default agent sandbox template (ADR047 D1
-- amendment 2026-11-19). A session or an automation run that names no
-- agentConfig.template runs on it; '' means the platform default ('agent').
-- bex-api validates a write against the registered agent templates and falls
-- back to 'agent' when a stored name is no longer registered, so the column
-- only bounds the length (agentConfig.template's own bound).
ALTER TABLE agent_environments
    ADD COLUMN default_template text NOT NULL DEFAULT '' CHECK (length(default_template) <= 128);
