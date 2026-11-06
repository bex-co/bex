-- ADR099: workspace skills for cloud agents, SKILL.md documents in the Agent
-- Skills format. name/description/triggers are the frontmatter bex reads;
-- extra holds any other frontmatter keys verbatim (YAML), so a skill moves
-- between a repository and the workspace unchanged. The row goes with its
-- workspace.
CREATE TABLE agent_skills (
    id           text        PRIMARY KEY,
    workspace_id text        NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    name         text        NOT NULL CHECK (name ~ '^[a-z0-9]+(-[a-z0-9]+)*$' AND length(name) <= 64),
    description  text        NOT NULL CHECK (length(description) BETWEEN 1 AND 1024),
    triggers     text[]      NOT NULL DEFAULT '{user,model}'
                             CHECK (triggers = '{user}' OR triggers = '{user,model}'),
    enabled      boolean     NOT NULL DEFAULT true,
    body         text        NOT NULL CHECK (octet_length(body) BETWEEN 1 AND 65536),
    extra        text        NOT NULL DEFAULT '' CHECK (octet_length(extra) <= 16384),
    version      int         NOT NULL DEFAULT 1 CHECK (version > 0),
    updated_by   text        NOT NULL DEFAULT '',
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now(),
    UNIQUE (workspace_id, name)
);

-- The skills a turn invoked explicitly (ADR099 D4), in request order:
-- [{name, arguments}].
ALTER TABLE agent_session_turns
    ADD COLUMN skills jsonb NOT NULL DEFAULT '[]'::jsonb;
