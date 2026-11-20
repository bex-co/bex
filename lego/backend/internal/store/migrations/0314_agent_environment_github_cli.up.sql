-- w10/m524: whether the workspace's agent sessions get a `gh` (ADR047 D2,
-- ADR090 D4 amendments 2026-11-19), as a Devin security profile can remove
-- the GitHub CLI token. Stored as the opt-out, so every existing row and a
-- workspace with no row keep `gh` on (the API reads it as githubCliEnabled).
-- While it is true, bex-api refuses the session's github-api mints and the
-- driver configures no `gh` at the next dispatch; Git and the bex tools are
-- unaffected.
ALTER TABLE agent_environments
    ADD COLUMN github_cli_disabled boolean NOT NULL DEFAULT false;
