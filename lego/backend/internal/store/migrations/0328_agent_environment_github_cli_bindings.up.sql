-- w10/m561: Devin's narrower gh bindings over the workspace's GitHub CLI
-- setting (ADR047 D2, ADR090 D4 amendments 2026-11-22).
--
-- automations_github_cli_enabled is the automations default: NULL inherits
-- the workspace's setting (github_cli_disabled), true or false binds the
-- sessions an automation starts. github_cli_mandatory makes the workspace's
-- gh off a floor no automation, automations default or session turns back
-- on; it has no effect while the workspace's gh is on. Every existing row and
-- a workspace with no row read as inherit and not mandatory.
ALTER TABLE agent_environments
    ADD COLUMN automations_github_cli_enabled boolean,
    ADD COLUMN github_cli_mandatory boolean NOT NULL DEFAULT false;
