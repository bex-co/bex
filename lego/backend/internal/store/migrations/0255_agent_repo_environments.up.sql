-- w9/m387, ADR109: a workspace's per-repository environment for cloud agent
-- sessions — setup steps the driver runs from the repository root before the
-- agent, and checks the overlay asks the agent to run (Devin's blueprint,
-- phase 1). repo is owner/name, lowercased like a session's repository.
-- Shapes and limits are validated by bex-api (ADR109 D3).
CREATE TABLE agent_repo_environments (
    workspace_id text        NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    repo         text        NOT NULL CHECK (length(repo) BETWEEN 3 AND 255),
    setup        jsonb       NOT NULL DEFAULT '[]'::jsonb,
    checks       jsonb       NOT NULL DEFAULT '[]'::jsonb,
    updated_at   timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (workspace_id, repo)
);
