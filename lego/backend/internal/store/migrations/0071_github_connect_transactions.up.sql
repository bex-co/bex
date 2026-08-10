-- Subject-bound, single-use GitHub connect transactions (w1/m67 F3, from the
-- 2026-08-10 codex-security scan).
--
-- THE GAP: the connect flow's signed state carried only {workspace, expiry}, and
-- the callback is deliberately anonymous (GitHub's redirect has no bex session of
-- its own to present). The two proofs it collected therefore belonged to two
-- unrelated principals — "an authorized bex workspace started a connect" and
-- "this browser administers the installation" — and both were portable. An
-- attacker could hand its own signed install URL to a victim GitHub org admin,
-- who would complete a perfectly genuine installation and bind THEIR repositories
-- to the ATTACKER's workspace. w1/m65's unique installation->workspace binding
-- then locks the rightful workspace out of its own installation, so the same lure
-- is also a denial vector.
--
-- THE FIX: a server-side transaction row, minted when an authorized caller starts
-- the flow, that records WHO started it. The callback must present the same bex
-- subject and atomically consume the row before any installation is persisted, so
-- the two proofs are finally tied to one principal and one attempt.
CREATE TABLE IF NOT EXISTS github_connect_transactions (
    -- The opaque, cryptographically random nonce carried through GitHub. It is
    -- the primary key, so consumption is a single atomic DELETE … RETURNING.
    nonce       text PRIMARY KEY,
    -- The workspace the initiator authorized (previously the state's whole payload).
    tenant_id   text NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    -- The bex identity that started the flow. The callback must match it.
    subject     text NOT NULL,
    expires_at  timestamptz NOT NULL,
    created_at  timestamptz NOT NULL DEFAULT now()
);

-- Expired rows are pruned opportunistically on each mint (the flow is
-- human-driven and rare, so a janitor would be overkill); the index keeps that
-- sweep and any expiry check cheap.
CREATE INDEX IF NOT EXISTS github_connect_transactions_expires_idx
    ON github_connect_transactions (expires_at);
