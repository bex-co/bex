-- Render marks from/to required on plan_changed, instance_count_changed, and
-- autoscaling_config_changed. Each new column is typed (int/text/bool), nullable
-- for every other verb and for legacy rows without a recorded value. Same
-- structural guard as maintenance_mode_to: no free-form details object, so no
-- secret can reach the feed.
ALTER TABLE audit_events
    ADD COLUMN plan_from text,
    ADD COLUMN plan_to text,
    ADD COLUMN instance_count_from integer,
    ADD COLUMN instance_count_to integer,
    ADD COLUMN autoscaling_min_from integer,
    ADD COLUMN autoscaling_max_from integer,
    ADD COLUMN autoscaling_min_to integer,
    ADD COLUMN autoscaling_max_to integer,
    ADD COLUMN auto_deploy_enabled boolean;
