-- Pre-deploy step outcome on the deploy record (w1/m33). App.spec.preDeployCommand
-- runs as a Job against the new revision's image before it serves traffic; the
-- operator records the step's Running/Succeeded/Failed on the App CR status, and
-- the reconciler projects it here so a deploy that failed its migration is
-- distinguishable from one that failed its health check — the overall `status`
-- is `update_failed` either way, but `pre_deploy_status` is 'failed' only for the
-- former. Empty '' means no pre-deploy step ran (the default, byte-identical for
-- existing rows). Values: '' | 'running' | 'succeeded' | 'failed'.
ALTER TABLE deploys
    ADD COLUMN pre_deploy_status text NOT NULL DEFAULT '';
