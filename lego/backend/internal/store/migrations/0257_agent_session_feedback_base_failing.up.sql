-- w6/m399: a CI-failure steer says when its failing checks also fail on the
-- pull request's base branch, so the session does not spend a turn on a
-- failure its change did not cause. base_failing_ref is that base branch
-- when at least one of the suite's failing checks also failed on its latest
-- commit; empty otherwise, and for every comment or review.
ALTER TABLE agent_session_feedback ADD COLUMN base_failing_ref text NOT NULL DEFAULT '';
