-- Core source-of-truth entities (docs/ADR003-control-plane.md §Schema sketch):
-- tenants own apps; apps own domains. The control plane projects apps rows
-- (+ their domains) into App CRs; the operator executes them.
--
-- IDs are Render-style typed opaque strings ("tea-", "srv-", "cdm-" + xid,
-- e.g. "srv-c185th5c2rvvnhbfiltg"), generated in Go (store.go newID) — typed,
-- k-sortable, non-guessable, and safe to expose in URLs/hostnames (same
-- reasoning as docs/ADR009-postgresql-management.md §4). No server-side default:
-- the application is the only writer.

CREATE TABLE tenants (
    id         text PRIMARY KEY,
    name       text NOT NULL UNIQUE,
    plan       text NOT NULL DEFAULT 'free',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE apps (
    id               text PRIMARY KEY,
    tenant_id        text NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    name             text NOT NULL,
    repo             text,
    image            text,
    branch           text NOT NULL DEFAULT 'main',
    port             integer NOT NULL DEFAULT 3000,
    replicas         integer NOT NULL DEFAULT 1,
    tier             text NOT NULL DEFAULT 'free',
    idle_ttl_seconds integer NOT NULL DEFAULT 0,
    suspended        boolean NOT NULL DEFAULT false,
    -- Observed state (phase, url) is NOT stored — it lives on the App CR's
    -- status, read from the cluster by bex-api at query time.
    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, name),
    CHECK (repo IS NOT NULL OR image IS NOT NULL)
);

CREATE INDEX apps_tenant_id_idx ON apps (tenant_id);

-- BYOD custom domains. is_primary marks the App's canonical host (spec.host);
-- the rest become spec.hosts. Verification + TLS state are the operator's /
-- cert-manager's concern (read from the cluster), not stored here.
CREATE TABLE domains (
    id          text PRIMARY KEY,
    app_id      text NOT NULL REFERENCES apps (id) ON DELETE CASCADE,
    host        text NOT NULL UNIQUE,
    is_primary  boolean NOT NULL DEFAULT false,
    created_at  timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX domains_app_id_idx ON domains (app_id);

-- At most one primary domain per app.
CREATE UNIQUE INDEX domains_one_primary_per_app_idx ON domains (app_id) WHERE is_primary;
