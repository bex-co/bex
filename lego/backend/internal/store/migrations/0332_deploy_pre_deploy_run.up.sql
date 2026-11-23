-- w32/m572: Render's deploy log narrates a pre-deploy step: "==> Starting
-- pre-deploy: <command>" when it starts, "==> Running '<command>'" when its
-- container starts on a native runtime, the command's output, then "==>
-- Pre-deploy complete!" or "==> Pre-deploy has failed" and "==> Exited with
-- status N". The control plane copies the operator's record of the release's
-- step (status.preDeploy) onto the release's own deploy row while it is open,
-- so its log keeps those lines after the App moves on to a newer release.
-- NULL is no record.
ALTER TABLE deploys
  ADD COLUMN IF NOT EXISTS pre_deploy_started_at timestamptz,
  ADD COLUMN IF NOT EXISTS pre_deploy_container_started_at timestamptz,
  ADD COLUMN IF NOT EXISTS pre_deploy_finished_at timestamptz,
  ADD COLUMN IF NOT EXISTS pre_deploy_command text,
  ADD COLUMN IF NOT EXISTS pre_deploy_native boolean,
  ADD COLUMN IF NOT EXISTS pre_deploy_exit_code integer,
  ADD COLUMN IF NOT EXISTS pre_deploy_oom_killed boolean;
