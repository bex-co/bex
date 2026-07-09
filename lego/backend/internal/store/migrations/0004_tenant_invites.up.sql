-- w4/m12: workspace members & roles. 0002_workspaces already ships
-- tenant_members (tenant_id, subject, role) — the accepted-membership substrate.
-- This migration adds the PENDING side: an invite is addressed to an email
-- (there is no OpenFGA subject / Kratos identity id yet — the recipient may not
-- have signed up), carries the role it will grant, and is redeemed on the
-- recipient's first authenticated login (internal/api/tenancy.go), which turns
-- it into a tenant_members row + the matching workspace:<id> OpenFGA tuple.
--
--   id          typed opaque id, inv-<xid> (internal/id).
--   email       recipient address, lower-cased by the caller; the redemption
--               key (matched against the Kratos identity's verified email).
--   role        the FGA relation the accepted member gets
--               (viewer|contributor|developer|admin|billing).
--   token       a random secret carried in the invite link (belt-and-braces —
--               acceptance is by email match, the token lets a direct-accept
--               link exist and keeps rows unique per resend).
--   invited_by  the subject who sent it (audit).
--   expires_at  invites lapse; an expired invite is neither listed as pending
--               nor redeemable.
--   accepted_at set when redeemed; a redeemed invite is terminal (kept for audit,
--               excluded from the pending list). NULL while outstanding.
--
-- One OUTSTANDING invite per (tenant, email): a partial unique index lets a
-- resend after acceptance/expiry make a fresh row without colliding.

CREATE TABLE tenant_invites (
    id          text PRIMARY KEY,
    tenant_id   text NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    email       text NOT NULL,
    role        text NOT NULL CHECK (role IN ('viewer', 'contributor', 'developer', 'admin', 'billing')),
    token       text NOT NULL,
    invited_by  text NOT NULL DEFAULT '',
    created_at  timestamptz NOT NULL DEFAULT now(),
    expires_at  timestamptz NOT NULL,
    accepted_at timestamptz
);

-- At most one outstanding invite per (tenant, email); resends after
-- acceptance/expiry are new rows (the index only covers accepted_at IS NULL).
CREATE UNIQUE INDEX tenant_invites_pending_idx
    ON tenant_invites (tenant_id, email) WHERE accepted_at IS NULL;

-- Redemption looks invites up by email across all tenants (a signup accepts
-- every pending invite addressed to it), so index the redemption key.
CREATE INDEX tenant_invites_email_idx ON tenant_invites (email) WHERE accepted_at IS NULL;
