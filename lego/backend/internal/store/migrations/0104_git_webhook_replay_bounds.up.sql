-- codex-security U9MUKo finding 4: bind replay claims to both the tenant scope
-- and the signing-secret epoch. The handler caps each scope across every live
-- epoch; replica leases let maintenance remove an old epoch only after no live
-- API process still accepts that secret. Rows predating this migration remain
-- in the finite legacy partition because their original scope/epoch cannot be
-- reconstructed safely.
ALTER TABLE git_webhook_replays
    ADD COLUMN scope text NOT NULL DEFAULT 'legacy',
    ADD COLUMN key_class text NOT NULL DEFAULT 'legacy',
    ADD COLUMN epoch text NOT NULL DEFAULT 'legacy';

CREATE TABLE git_webhook_replay_epoch_leases (
    instance_id text NOT NULL,
    key_class text NOT NULL CHECK (key_class IN ('manual', 'github')),
    epoch text NOT NULL,
    heartbeat_at timestamptz NOT NULL,
    PRIMARY KEY (instance_id, key_class)
);

CREATE INDEX git_webhook_replay_epoch_leases_expiry_idx
    ON git_webhook_replay_epoch_leases (heartbeat_at);

-- A crashed replica whose lease expired must not be able to reconnect later
-- and resurrect an epoch after its claims were purged. Retired epochs are
-- operator-driven (one per secret rotation), not tenant-driven ledger rows.
CREATE TABLE git_webhook_replay_retired_epochs (
    key_class text NOT NULL CHECK (key_class IN ('manual', 'github')),
    epoch text NOT NULL,
    retired_at timestamptz NOT NULL,
    PRIMARY KEY (key_class, epoch)
);

CREATE INDEX git_webhook_replays_scope_idx
    ON git_webhook_replays (scope)
    WHERE epoch <> 'legacy';
