-- w4/m596 t002: Render's trigger-build-source-build deploy.serviceScope
-- (autodeploy_only | none | custom) and pre-created service deploy rows waiting
-- for their build source build to succeed.
--
-- build_deploy_scope is recorded on the carrier build row ('' for legacy/git
-- webhook builds that fan out by the default rule, 'autodeploy_only', 'none',
-- or 'custom').
--
-- awaiting_build marks a linked service's deploy row created at trigger time
-- while its build_id is still building: generation = 0, status = 'created',
-- not competing for the active or overlap-pending unique slot until the build
-- succeeds and promotes it (or fails/cancels and closes it terminal).
ALTER TABLE deploys
    ADD COLUMN build_deploy_scope text NOT NULL DEFAULT '',
    ADD COLUMN awaiting_build boolean NOT NULL DEFAULT false,
    ADD CONSTRAINT deploys_awaiting_build_state_check
        CHECK (NOT awaiting_build OR (status = 'created' AND NOT overlap_pending AND NOT restart_deferred AND build_id IS NOT NULL AND generation = 0));

DROP INDEX deploys_one_active_per_app_idx;
CREATE UNIQUE INDEX deploys_one_active_per_app_idx ON deploys (app_id)
WHERE NOT overlap_pending AND NOT awaiting_build AND status IN (
    'created',
    'queued',
    'build_in_progress',
    'pre_deploy_in_progress',
    'update_in_progress'
);

CREATE INDEX deploys_awaiting_build_idx ON deploys (build_id)
WHERE awaiting_build AND status = 'created';
