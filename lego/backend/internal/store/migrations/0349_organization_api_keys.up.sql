-- w1/m657: an organization's audit-log API keys, as on Render ("Org Owners
-- can create an audit log API key"; "Audit log API keys have read-only access
-- to audit logs for your organization and all of its workspaces",
-- render.com/docs/api-keys). The key itself is a bex API key, a Hydra client
-- like every other (its secret, label, creator and last use live there); this
-- row is what makes it the org's and nothing else.
--
--   client_id        the Hydra client id, the key's id on the wire.
--   organization_id  the org it reads. No foreign key: the row outlives the
--                    org as a tombstone. A key with a row is held to the
--                    audit-log reads whatever its state, so deleting the row
--                    would leave a bare API key behind it; authority needs
--                    the row live (revoked_at NULL) and the org present.
--   key_type         Render's APIToken.type for these keys.
--   revoked_at       set by a revoke or by the org's end; the row stays.
CREATE TABLE organization_api_keys (
    client_id       text PRIMARY KEY,
    organization_id text NOT NULL,
    label           text NOT NULL,
    key_type        text NOT NULL DEFAULT 'AUDIT_LOG' CHECK (key_type IN ('AUDIT_LOG')),
    created_by      text NOT NULL DEFAULT '',
    created_at      timestamptz NOT NULL DEFAULT now(),
    revoked_at      timestamptz
);

CREATE INDEX organization_api_keys_live_idx
    ON organization_api_keys (organization_id) WHERE revoked_at IS NULL;
