-- w8/m579: share bex-api rate-limit token buckets and log-subscription slot
-- leases across replicas through control-plane Postgres, so effective caller
-- budgets and concurrent SSE/WS/NDJSON caps match Render's documented numbers
-- regardless of replica count.
CREATE TABLE rate_limit_buckets (
    bucket_key text PRIMARY KEY,
    tokens     double precision NOT NULL,
    updated_at timestamptz NOT NULL,
    expires_at timestamptz NOT NULL
);

CREATE INDEX rate_limit_buckets_expires_at_idx ON rate_limit_buckets (expires_at);

CREATE TABLE log_subscription_slots (
    slot_id    text PRIMARY KEY,
    subject    text NOT NULL,
    workspace  text NOT NULL,
    expires_at timestamptz NOT NULL
);

CREATE INDEX log_subscription_slots_expires_at_idx ON log_subscription_slots (expires_at);
CREATE INDEX log_subscription_slots_subject_idx ON log_subscription_slots (subject) WHERE subject <> '';
CREATE INDEX log_subscription_slots_workspace_idx ON log_subscription_slots (workspace);
