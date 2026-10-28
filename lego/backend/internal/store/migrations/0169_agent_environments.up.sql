-- w10/m17 t003: the workspace agent environment (ADR047 D5 amendment
-- 2026-10-27). One row per workspace that customized it; a workspace with no
-- row uses the defaults (network tier 'registries', no custom hosts, the
-- provider's default model), so existing workspaces need no backfill.
--
-- network_tier: 'registries' (the built-in package registries stay reachable
-- for the whole session), 'custom' (registries plus custom_hosts) or 'full'
-- (opt-in any-host access; stored, enforced as 'registries' until the cluster
-- admission policy can express it without opening the model providers).
-- custom_hosts are exact public DNS names, validated by bex-api with
-- sessionegress.ExtraDestinations; default_model '' means the provider default.
CREATE TABLE agent_environments (
    workspace_id  text        PRIMARY KEY REFERENCES tenants (id) ON DELETE CASCADE,
    network_tier  text        NOT NULL DEFAULT 'registries'
                              CHECK (network_tier IN ('registries', 'full', 'custom')),
    custom_hosts  text[]      NOT NULL DEFAULT '{}' CHECK (cardinality(custom_hosts) <= 32),
    default_model text        NOT NULL DEFAULT '' CHECK (length(default_model) <= 255),
    updated_at    timestamptz NOT NULL DEFAULT now()
);
