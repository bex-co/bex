-- w4/m10: audit log — one row per authorized WRITE-verb attempt (allowed or
-- denied), captured at the one interception point every verb funnels through
-- (core.Base.Authorize/AuthorizeOn, internal/core/audit.go). Read verbs are
-- out of scope by default (volume) — see docs/bex-api.md § Audit log.
--
-- workspace_id is the OpenFGA object's tenant component ("tea-…", or
-- "default" for the store-off/bootstrap path) — the workspace the event is
-- ABOUT, not necessarily the caller's own (a cross-tenant attempt on someone
-- else's workspace belongs on THAT workspace's trail). No FK to tenants: a
-- denied cross-tenant guess may name a workspace that never existed, and a
-- deleted tenant's own history must survive its row's cascade delete, not
-- vanish with it.
--
-- caller/caller_method may be empty: an authz-enabled deployment always has
-- an identity by the time a write verb runs (Base.checkAuthz 403s otherwise),
-- but the column stays permissive rather than constraining a hook that other
-- callers (e.g. a future service-to-service credential) might one day use.
CREATE TABLE audit_events (
    id            text PRIMARY KEY,
    workspace_id  text NOT NULL,
    caller        text NOT NULL DEFAULT '',
    caller_method text NOT NULL DEFAULT '',
    verb          text NOT NULL,
    resource      text NOT NULL,
    outcome       text NOT NULL CHECK (outcome IN ('allowed', 'denied')),
    at            timestamptz NOT NULL
);

-- The read surface (t003) always filters by workspace and orders newest
-- first; the retention sweep (below) scans by at alone.
CREATE INDEX audit_events_workspace_at_idx ON audit_events (workspace_id, at DESC, id DESC);
CREATE INDEX audit_events_at_idx ON audit_events (at);
