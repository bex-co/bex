ALTER TABLE deploys
  DROP COLUMN IF EXISTS rollout_web_concurrency,
  DROP COLUMN IF EXISTS rollout_command,
  DROP COLUMN IF EXISTS app_started_at,
  DROP COLUMN IF EXISTS rollout_started_at;
