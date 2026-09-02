DROP INDEX IF EXISTS git_webhook_replays_scope_idx;
DROP TABLE IF EXISTS git_webhook_replay_epoch_leases;
DROP TABLE IF EXISTS git_webhook_replay_retired_epochs;

ALTER TABLE git_webhook_replays
    DROP COLUMN scope,
    DROP COLUMN key_class,
    DROP COLUMN epoch;
