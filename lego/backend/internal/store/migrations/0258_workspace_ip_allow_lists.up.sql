-- w9/m388, ADR032 § Workspace layer: a workspace's inbound IP rules, Render's
-- Scale and Enterprise "entire workspace" rules. One row per workspace that
-- saved rules; no row means the workspace never set any, so no layer is
-- projected and reads show Render's seeded allow-all. A saved empty list is
-- deny-all, as for environments. Entries are Render's {cidrBlock,
-- description} objects, the shape environments.ip_allow_list holds (0053's
-- structured-row rule); bex-api validates the CIDRs and caps the count at
-- core.MaxAllowListEntries, mirrored here.
CREATE TABLE workspace_ip_allow_lists (
    tenant_id     text        PRIMARY KEY REFERENCES tenants (id) ON DELETE CASCADE,
    ip_allow_list jsonb       NOT NULL DEFAULT '[]'::jsonb,
    updated_at    timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT workspace_ip_allow_lists_structured CHECK (
        jsonb_typeof(ip_allow_list) = 'array'
        AND jsonb_array_length(ip_allow_list) <= 1000
        AND NOT jsonb_path_exists(
            ip_allow_list,
            '$[*] ? (@.type() != "object" || !exists(@.cidrBlock) || @.cidrBlock.type() != "string" || @.cidrBlock == "" || (exists(@.description) && @.description.type() != "string"))'
        )
    )
);
