ALTER TABLE audit_events
    DROP COLUMN IF EXISTS plan_from,
    DROP COLUMN IF EXISTS plan_to,
    DROP COLUMN IF EXISTS instance_count_from,
    DROP COLUMN IF EXISTS instance_count_to,
    DROP COLUMN IF EXISTS autoscaling_min_from,
    DROP COLUMN IF EXISTS autoscaling_max_from,
    DROP COLUMN IF EXISTS autoscaling_min_to,
    DROP COLUMN IF EXISTS autoscaling_max_to,
    DROP COLUMN IF EXISTS auto_deploy_enabled;
