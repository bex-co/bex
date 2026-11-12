-- ADR047 D11: a `/bex` comment on a pull request no session works on starts
-- a session that takes the pull request over. The session still pushes only
-- its own bex-agent/<id> branch, started from the pull request's head; its PR
-- verb then fast-forwards the pull request's branch to that branch through
-- the GitHub API, which refuses anything that is not a fast-forward. So the
-- row records which pull request it took over (takeover_pr_number, 0 for
-- every other session) and that pull request's head branch, the one ref the
-- fast-forward may move (takeover_head_ref). Only the internal PR intake
-- sets them, at create, and they never change.
ALTER TABLE agent_sessions
    ADD COLUMN takeover_pr_number integer NOT NULL DEFAULT 0
        CHECK (takeover_pr_number >= 0),
    ADD COLUMN takeover_head_ref text NOT NULL DEFAULT ''
        CHECK (char_length(takeover_head_ref) <= 255),
    ADD CONSTRAINT agent_sessions_takeover_check
        CHECK ((takeover_pr_number = 0 AND takeover_head_ref = '')
            OR (takeover_pr_number > 0 AND takeover_head_ref <> '' AND repo <> ''));
