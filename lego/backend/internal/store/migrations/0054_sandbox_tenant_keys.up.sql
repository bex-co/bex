-- Per-workspace OpenSandbox tenant keys (w3/m32 t006, docs/ADR042-sandbox-cluster-substrate.md D4).
-- Each workspace gets one opaque high-entropy key that bex-api sends to the
-- OpenSandbox server as the OPEN-SANDBOX-API-KEY header. The server's HTTP tenant
-- provider calls back to bex-api's GET /v1/sandbox-tenants to resolve the key to
-- the workspace's `<ws>-sandbox` namespace, then auto-scopes every lifecycle op to
-- it. A stolen key is therefore confined to exactly one tenant namespace even if
-- bex-api has a bug — OpenSandbox itself rejects cross-tenant ops (ADR042 D4).
CREATE TABLE IF NOT EXISTS sandbox_tenant_keys (
    api_key      text        PRIMARY KEY,
    workspace_id text        NOT NULL UNIQUE,
    created_at   timestamptz NOT NULL DEFAULT now()
);
