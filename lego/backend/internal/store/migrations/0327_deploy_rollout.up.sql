-- w32/m560: Render's deploy log narrates a rollout between a build's end and
-- its verdict: "==> Deploying..." when it starts, "==> Setting
-- WEB_CONCURRENCY=N by default, based on available CPUs in the instance" for
-- the instance's default, and on a native runtime "==> Running '<start
-- command>'" each time the command starts. The control plane copies the
-- operator's record of the release's rollout (status.releaseRollout) onto the
-- release's own deploy row while it is open, so its log keeps those lines
-- after the App moves on to a newer release. NULL is no recorded rollout.
ALTER TABLE deploys
  ADD COLUMN IF NOT EXISTS rollout_started_at timestamptz,
  ADD COLUMN IF NOT EXISTS app_started_at timestamptz[],
  ADD COLUMN IF NOT EXISTS rollout_command text,
  ADD COLUMN IF NOT EXISTS rollout_web_concurrency integer;
