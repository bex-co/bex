-- Retained display names for metered resources (w2/m96 t001).
--
-- Billing lines outlive the resources they bill for. Until now a charge row's
-- name was resolved only from the live App row or the live Database/KeyValue
-- CR, so the moment a resource was deleted its charges collapsed to a bare
-- `srv-…` id — and a sandbox, which has no name at all, was always a bare UUID.
--
-- This table is the durable side: (tenant, kind, id) -> the last display name
-- bex knew. It is written while the resource is alive, is NEVER removed when
-- the resource is deleted, and is keyed by **id** so a delete-then-recreate
-- under the same display name stays two separate rows (two separate charges).
--
-- Purge is the tenants cascade: the workspace teardown's DeleteTenant drops the
-- tenants row and this table goes with it, so no purger needs to know about it.
CREATE TABLE IF NOT EXISTS resource_display_names (
    tenant_id     text        NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    resource_kind text        NOT NULL,
    resource_id   text        NOT NULL,
    display_name  text        NOT NULL,
    updated_at    timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, resource_kind, resource_id)
);

-- Backfill what is still knowable. Resources already deleted before this
-- migration are genuinely lost — their names exist nowhere — and stay bare ids
-- (the backfill decision recorded in .pm/w2/m96/README.md).

-- Services: the row's effective display name is display_name when it has been
-- renamed, else the create-time name.
INSERT INTO resource_display_names (tenant_id, resource_kind, resource_id, display_name)
SELECT tenant_id, 'service', id, COALESCE(NULLIF(display_name, ''), name)
FROM apps
WHERE COALESCE(NULLIF(display_name, ''), name) <> ''
ON CONFLICT (tenant_id, resource_kind, resource_id) DO NOTHING;

-- Sandboxes: an agent session outlives the sandbox it ran in (sessions are
-- archived, never dropped), so its repo/branch labels both the session's
-- current sandbox and every earlier one it was rehydrated from — which is how
-- a metered sandbox UUID that no live `sandboxes` query lists still gets a name.
INSERT INTO resource_display_names (tenant_id, resource_kind, resource_id, display_name)
SELECT DISTINCT ON (workspace_id, sandbox) workspace_id, 'sandbox', sandbox, label
FROM (
    SELECT s.workspace_id,
           s.sandbox_id AS sandbox,
           CASE WHEN s.branch <> '' THEN s.repo || ' (' || s.branch || ')' ELSE s.repo END AS label,
           s.updated_at
    FROM agent_sessions s
    WHERE s.sandbox_id <> '' AND s.repo <> ''
    UNION ALL
    SELECT s.workspace_id,
           d.previous_sandbox_id AS sandbox,
           CASE WHEN s.branch <> '' THEN s.repo || ' (' || s.branch || ')' ELSE s.repo END AS label,
           s.updated_at
    FROM agent_session_dispatches d
    JOIN agent_sessions s ON s.id = d.session_id
    WHERE d.previous_sandbox_id <> '' AND s.repo <> ''
) AS labelled
ORDER BY workspace_id, sandbox, updated_at DESC
ON CONFLICT (tenant_id, resource_kind, resource_id) DO NOTHING;
