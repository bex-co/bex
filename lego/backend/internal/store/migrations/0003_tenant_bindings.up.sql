-- w1/m9: tenant onboarding grows on top of 0002_workspaces' tenant_members
-- (tenant_id, subject, role — subject already covers both Kratos identity ids
-- and Hydra client ids, so a minted API key's binding is just another
-- tenant_members row with role 'developer', not a separate table).
--
-- owner_identity_id (on tenants): set by the mint-on-first-login path
-- (internal/api/tenancy.go) for a tenant auto-created for a human identity on
-- its first authenticated call. NULL for every other tenant (platform-created
-- via the internal API, or a workspace made through the w6/m1 lifecycle
-- verbs). The partial unique index is the race-safety invariant — two
-- concurrent first logins for the same identity yield one tenant (INSERT ...
-- ON CONFLICT, not a check-then-insert), because at most one tenant row may
-- name a given identity as its auto-mint owner.

ALTER TABLE tenants ADD COLUMN owner_identity_id text;
CREATE UNIQUE INDEX tenants_owner_identity_idx
    ON tenants (owner_identity_id) WHERE owner_identity_id IS NOT NULL;
