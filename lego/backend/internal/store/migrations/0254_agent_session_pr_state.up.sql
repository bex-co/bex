-- w9/m386: the state of a session's pull request, as Devin's v3 session read
-- carries pull_requests[].pr_state ('open', 'merged' or 'closed'; a draft is
-- open). '' means not read yet. pr_state_at is when the state last changed.
-- pr_next_check_at is when bex next reads it from GitHub; NULL once there is
-- nothing left to learn (merged, or closed for a week), so finished pull
-- requests leave the reconciler's index instead of being rescanned forever.
ALTER TABLE agent_sessions
    ADD COLUMN pr_state         text NOT NULL DEFAULT ''
                                CHECK (pr_state IN ('', 'open', 'merged', 'closed')),
    ADD COLUMN pr_state_at      timestamptz,
    ADD COLUMN pr_next_check_at timestamptz;

UPDATE agent_sessions SET pr_next_check_at = now() WHERE pr_number > 0;

CREATE INDEX agent_sessions_pr_due_idx
    ON agent_sessions (pr_next_check_at)
    WHERE pr_next_check_at IS NOT NULL;

-- The list's prState filter, newest first like every list read.
CREATE INDEX agent_sessions_pr_state_list_idx
    ON agent_sessions (workspace_id, pr_state, created_at DESC, id DESC)
    WHERE pr_number > 0;
