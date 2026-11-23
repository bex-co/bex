DROP INDEX IF EXISTS deploys_awaiting_build_idx;

DELETE FROM deploys WHERE awaiting_build;

DROP INDEX IF EXISTS deploys_one_active_per_app_idx;
CREATE UNIQUE INDEX deploys_one_active_per_app_idx ON deploys (app_id)
WHERE NOT overlap_pending AND status IN (
    'created',
    'queued',
    'build_in_progress',
    'pre_deploy_in_progress',
    'update_in_progress'
);

ALTER TABLE deploys
    DROP CONSTRAINT IF EXISTS deploys_awaiting_build_state_check,
    DROP COLUMN IF EXISTS awaiting_build,
    DROP COLUMN IF EXISTS build_deploy_scope;
