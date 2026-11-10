-- w6/m309 (docs/ADR030 §13): a workspace's monthly spend limit on pipeline
-- minutes, as Render's Workspace Settings → Build Pipeline sets it. NULL: no
-- limit, so overage is bought without bound (Render documents no default).
ALTER TABLE tenants ADD COLUMN pipeline_spend_limit_cents bigint
    CHECK (pipeline_spend_limit_cents IS NULL OR pipeline_spend_limit_cents >= 0);

-- A third build-minutes notice: the workspace reached its spend limit.
ALTER TABLE build_minutes_notices DROP CONSTRAINT build_minutes_notices_stage_check;
ALTER TABLE build_minutes_notices ADD CONSTRAINT build_minutes_notices_stage_check
    CHECK (stage IN ('warned', 'exhausted', 'limited'));
