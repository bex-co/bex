ALTER TABLE deploys
  DROP COLUMN IF EXISTS rollout_first_exit_code,
  DROP COLUMN IF EXISTS rollout_first_exit_at;
