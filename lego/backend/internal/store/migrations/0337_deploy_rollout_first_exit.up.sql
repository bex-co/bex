-- w32/m591: Render's deploy log places a failed deploy's single
-- "==> Exited with status N" and troubleshooting line right after the start
-- command's first exit rather than at the deploy's close after later restarts.
-- The control plane copies the operator's first non-OOM app-container exit
-- (status.releaseRollout.firstExit) onto the release's own open deploy row so
-- every log reader keeps that moment and status after the App moves on. NULL
-- is no recorded first exit (older rows keep their closing lines).
ALTER TABLE deploys
  ADD COLUMN IF NOT EXISTS rollout_first_exit_at timestamptz,
  ADD COLUMN IF NOT EXISTS rollout_first_exit_code integer;
