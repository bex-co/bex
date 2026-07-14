-- 0023: protected-environment ACLs (w6/m19) — Render's protectedStatus /
-- networkIsolationEnabled / ipAllowList fields on an Environment
-- (docs/ADR032-environments.md's documented "Known divergence", closed here).
-- All three default to Render's own defaults: a newly-created environment is
-- unprotected, unisolated, with an open (empty) allowlist — byte-identical
-- behavior for every environment that predates this migration.
ALTER TABLE environments
    ADD COLUMN protected_status text NOT NULL DEFAULT 'unprotected'
        CHECK (protected_status IN ('protected', 'unprotected')),
    ADD COLUMN network_isolation_enabled boolean NOT NULL DEFAULT false,
    ADD COLUMN ip_allow_list text[] NOT NULL DEFAULT '{}';
