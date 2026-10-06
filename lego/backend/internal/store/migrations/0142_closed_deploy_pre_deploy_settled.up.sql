-- w5/m114 (w4/187): a closed deploy cannot keep pre_deploy_status 'running'.
-- SetDeployPreDeployStatus writes only open rows, so before w4/187 a deploy
-- that closed mid-step kept 'running' forever; every close now settles the
-- step (TransitionDeploy, cancelPendingDeploys). Settle the rows closed before
-- that the same way — the deploy's own outcome decides it: a release that went
-- live (deactivated only ever follows live) passed its pre-deploy, a
-- pre_deploy_failed close failed it, and any other close ended it — then have
-- Postgres refuse the state instead of trusting the app code alone.
-- finished_at is set by every close. Rows w4/187 settled as canceled beside a
-- pre_deploy_failed close are the contradiction m114 removes: they read failed.
--
-- The backfill must not fire product_deploy_event (0114), which re-inserts a
-- deploy's analytics events on any UPDATE and would resurrect ones retention
-- already purged — the reason 0114 itself skipped an UPDATE backfill. The
-- service-event index trigger (0083) watches finished_at only.
ALTER TABLE deploys DISABLE TRIGGER product_deploy_event;
UPDATE deploys
SET pre_deploy_status = CASE
        WHEN status IN ('live', 'deactivated') THEN 'succeeded'
        WHEN status = 'pre_deploy_failed' THEN 'failed'
        ELSE 'canceled'
    END
WHERE finished_at IS NOT NULL
  AND (pre_deploy_status = 'running'
       OR (status = 'pre_deploy_failed' AND pre_deploy_status = 'canceled'));
ALTER TABLE deploys ENABLE TRIGGER product_deploy_event;

ALTER TABLE deploys ADD CONSTRAINT deploys_closed_pre_deploy_settled
    CHECK (finished_at IS NULL OR pre_deploy_status <> 'running');
