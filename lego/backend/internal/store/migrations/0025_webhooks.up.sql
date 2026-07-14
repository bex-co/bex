-- w3/m11: outbound event webhooks — Render's /webhooks (bex → you) direction.
--
-- webhook_endpoints is a workspace's registered destinations: a URL, a
-- server-minted signing secret (returned once at creation, never by a read —
-- the api-key mint-once precedent), and the event types it subscribes to.
-- No plan-tier cap on endpoints per workspace: bex has no billing system
-- (w6 non-goal), a deliberate divergence from Render's Pro=1/Scale=100.
--
-- webhook_deliveries is the durable delivery queue AND the dashboard's
-- history view: one row per (event × subscribed endpoint), retried in place
-- (attempt_count/next_attempt_at) until delivered, exhausted, or its endpoint
-- is disabled. The payload column carries the exact serialized body the
-- signature was computed over, so a retry resends byte-identical content.
--
-- webhook_watermark is the dispatcher's single-row cursor into the composed
-- event feed (deploys + audit_events — the same two tables the w3/m7 events
-- feed reads; docs/ADR006-bex-api.md § Service events). Seeded at first start
-- so history from before the feature existed is never replayed.

CREATE TABLE webhook_endpoints (
    id              text PRIMARY KEY,
    tenant_id       text NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    name            text NOT NULL,
    url             text NOT NULL,
    secret          text NOT NULL,
    event_types     text[] NOT NULL,
    enabled         boolean NOT NULL DEFAULT true,
    -- Why the endpoint is disabled ('' while enabled): 'delivery failures' is
    -- the auto-disable path, anything else is a caller's manual toggle.
    disabled_reason text NOT NULL DEFAULT '',
    created_by      text NOT NULL DEFAULT '',
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX webhook_endpoints_tenant_idx ON webhook_endpoints (tenant_id);

CREATE TABLE webhook_deliveries (
    id                text PRIMARY KEY,
    endpoint_id       text NOT NULL REFERENCES webhook_endpoints (id) ON DELETE CASCADE,
    -- event_id is the derived evt-… id of the source transition — the same id
    -- the service-events feed shows for it, and the webhook-id header /
    -- payload data.id the receiver dedupes on.
    event_id          text NOT NULL,
    event_type        text NOT NULL,
    service_id        text NOT NULL DEFAULT '',
    payload           text NOT NULL,
    attempt_count     integer NOT NULL DEFAULT 0,
    -- Last attempt's HTTP status (0 = transport error or never attempted).
    last_status       integer NOT NULL DEFAULT 0,
    last_error        text NOT NULL DEFAULT '',
    next_attempt_at   timestamptz NOT NULL,
    last_attempted_at timestamptz,
    delivered_at      timestamptz,
    failed_at         timestamptz,
    created_at        timestamptz NOT NULL DEFAULT now()
);

-- The sender polls due open deliveries; open rows are a small working set, so
-- the partial index stays tiny however large the history grows. (This one may
-- be planned generically like 0012's audit index caveat, but the predicate
-- here matches columns the query filters on with constants, not parameters.)
CREATE INDEX webhook_deliveries_due_idx ON webhook_deliveries (next_attempt_at)
    WHERE delivered_at IS NULL AND failed_at IS NULL;
-- The dashboard/API reads one endpoint's history newest-first, keyset-paged.
CREATE INDEX webhook_deliveries_endpoint_idx ON webhook_deliveries (endpoint_id, created_at DESC, id DESC);

-- Single-row watermark (id is always true — the CHECK makes a second row
-- unrepresentable).
CREATE TABLE webhook_watermark (
    id  boolean PRIMARY KEY DEFAULT true CHECK (id),
    at  timestamptz NOT NULL,
    key text NOT NULL DEFAULT ''
);

-- The dispatcher's composed feed scans BOTH deploy transitions globally
-- (ascending, from the watermark) — 0005/0012 indexed them per-app only.
CREATE INDEX deploys_created_idx ON deploys (created_at);
CREATE INDEX deploys_finished_idx ON deploys (finished_at) WHERE finished_at IS NOT NULL;
