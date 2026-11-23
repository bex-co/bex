ALTER TABLE deploys
  DROP COLUMN IF EXISTS pre_deploy_oom_killed,
  DROP COLUMN IF EXISTS pre_deploy_exit_code,
  DROP COLUMN IF EXISTS pre_deploy_native,
  DROP COLUMN IF EXISTS pre_deploy_command,
  DROP COLUMN IF EXISTS pre_deploy_finished_at,
  DROP COLUMN IF EXISTS pre_deploy_container_started_at,
  DROP COLUMN IF EXISTS pre_deploy_started_at;
