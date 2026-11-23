-- w10/047: GitHub auto-merge and the merge queue on a session's pull request,
-- as Devin Review's merge bar shows "the current auto-merge status, including
-- who enabled it". pr_auto_merge is the auto-merge armed on it as GitHub
-- reports it ({"method", "enabledBy", "enabledAt"}), NULL when none is;
-- pr_merge_queued is set while it is in its base branch's merge queue. Both
-- are kept by the merge verb's answer and the PR-state poller, and cleared
-- once the pull request is merged or closed, or the session records another.
ALTER TABLE agent_sessions
    ADD COLUMN pr_auto_merge   jsonb,
    ADD COLUMN pr_merge_queued boolean NOT NULL DEFAULT false;
