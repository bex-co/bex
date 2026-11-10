DELETE FROM build_minutes_notices WHERE stage = 'limited';
ALTER TABLE build_minutes_notices DROP CONSTRAINT build_minutes_notices_stage_check;
ALTER TABLE build_minutes_notices ADD CONSTRAINT build_minutes_notices_stage_check
    CHECK (stage IN ('warned', 'exhausted'));
ALTER TABLE tenants DROP COLUMN IF EXISTS pipeline_spend_limit_cents;
