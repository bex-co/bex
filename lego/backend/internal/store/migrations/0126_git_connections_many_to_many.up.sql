-- w2/m162 (ADR078 §2, 2026-09-16): relax git_connections from N:1 to N:N — a
-- workspace holds many installations (already true since 0091), and an
-- installation may now serve many workspaces.
--
-- BEFORE: PRIMARY KEY (installation_id) bound each installation to at most one
-- workspace. GitHub permits exactly one installation of an App per account, so a
-- personal GitHub account could never back both a personal and a team workspace:
-- the install URL dead-ends (GitHub strips the signed state for an already-
-- installed account), the claim flow filtered the installation out as "bound
-- elsewhere", and a direct github.com install is a no-op. Three dead ends.
--
-- AFTER: PRIMARY KEY (workspace_id, installation_id). Each binding still carries
-- the full three-proof sequence (ADR026 §4, w1/m67 F3) — a signed nonce-only
-- state naming a single-use server-side transaction, the initiating subject
-- matched at callback, a fresh can_manage, and an OAuth proof that the human
-- administers the exact installation — so two workspaces holding one installation
-- means its administrator asserted it twice, deliberately. What w1/m65 F2 guards
-- is the UNPROVED attach, which stays rejected.
--
-- The new key is a strict RELAXATION of the old one: every existing row already
-- satisfies it, so this is a constraint swap with no dedup, no backfill, and no
-- row rewrite. It is also forward-compatible with the pre-revision binary (which
-- simply never inserts a second row for an installation), so it may land ahead of
-- the deploy. Reverting is NOT symmetric — see the down migration.
ALTER TABLE git_connections DROP CONSTRAINT git_connections_pkey;
ALTER TABLE git_connections ADD PRIMARY KEY (workspace_id, installation_id);

-- The composite key's leading column already indexes workspace_id, so 0091's
-- standalone index is redundant; drop it rather than carry a duplicate.
DROP INDEX IF EXISTS git_connections_workspace_idx;

-- Back the push webhook's REVERSE lookup (GitConnectionsByInstallation): resolve
-- a delivery's installation to every workspace that proved a binding for it
-- (ADR078 §4a). Non-unique — that non-uniqueness is the whole point of N:N.
CREATE INDEX IF NOT EXISTS git_connections_installation_idx
  ON git_connections (installation_id);
