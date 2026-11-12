-- w9/m344 (ADR047 D11): feedback on an agent session's pull request, queued
-- for the session's next turn. A review, a PR comment or a failed CI run is
-- one row, keyed by the id of the GitHub object it came from (the comment,
-- review or check suite id, all from the signed payload), so a redelivery is
-- a no-op. Rows wait while a turn is running; the feedback worker leases a
-- session's pending rows, steers the session once with them, and records the
-- turn they started.
CREATE TABLE agent_session_feedback (
    session_id     text        NOT NULL REFERENCES agent_sessions (id) ON DELETE CASCADE,
    source_key     text        NOT NULL CHECK (char_length(source_key) BETWEEN 1 AND 200),
    kind           text        NOT NULL CHECK (kind IN ('pr_comment', 'pr_review', 'ci_failure')),
    -- The bex subject the steer runs as: the linked member who wrote a comment
    -- or review, the session's creator for a CI failure.
    actor          text        NOT NULL CHECK (char_length(actor) BETWEEN 1 AND 200),
    -- The commenter's GitHub login, empty for a CI failure.
    author         text        NOT NULL DEFAULT '' CHECK (char_length(author) <= 100),
    url            text        NOT NULL DEFAULT '' CHECK (char_length(url) <= 2000),
    body           text        NOT NULL CHECK (char_length(body) <= 60000),
    -- The commit a CI failure ran on; a failure on a head the session has
    -- since moved past is dropped rather than delivered.
    head_sha       text        NOT NULL DEFAULT '' CHECK (char_length(head_sha) <= 64),
    created_at     timestamptz NOT NULL DEFAULT now(),
    claimed_until  timestamptz,
    delivered_turn integer,
    delivered_at   timestamptz,
    dropped_reason text,
    PRIMARY KEY (session_id, source_key)
);

-- The feedback worker's claim starts from the sessions with pending rows.
CREATE INDEX agent_session_feedback_pending_idx
    ON agent_session_feedback (session_id)
    WHERE delivered_at IS NULL AND dropped_reason IS NULL;

-- A comment or review finds its session by repository and PR number, a check
-- suite by head commit.
CREATE INDEX agent_sessions_pull_request_idx
    ON agent_sessions (repo, pr_number)
    WHERE pr_number > 0;
CREATE INDEX agent_sessions_pr_head_idx
    ON agent_sessions (head_sha)
    WHERE pr_number > 0 AND head_sha <> '';
