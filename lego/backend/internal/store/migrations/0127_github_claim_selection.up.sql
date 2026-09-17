-- w2/m162 (ADR078 §3a, 2026-09-16): the claim selector.
--
-- Dropping the "not already bound to any workspace" candidate filter (N:N, §2)
-- ENLARGES the claim callback's candidate set, and set size is exactly what
-- ambiguous_installation trips on. Before, an admin who administers several
-- installations usually saw them filtered down to one because the others were
-- bound; after, every installation they administer is a candidate. Ambiguity
-- becomes the common case precisely for the multi-workspace users N:N exists to
-- serve, so shipping it without a selector would trade one dead end for another.

-- (1) The start-time shortcut: a claim may name the installation up front, which
-- NARROWS the server-proved candidate set. It can never add to it — an id the
-- OAuth user does not administer is still not a candidate. NULL = unspecified.
ALTER TABLE github_connect_transactions
  ADD COLUMN IF NOT EXISTS installation_id bigint;

-- (2) The deferred selector. The dashboard cannot know any installation id before
-- the OAuth round trip — the candidate set is only discoverable with the user
-- token minted from the single-use code — so on ambiguity the callback persists
-- the set it has ALREADY fully proved (state, nonce, initiator, fresh can_manage,
-- and VerifyInstallationAdmin per installation) and hands the browser a selection
-- id instead of a dead-end error code.
--
-- This row is a memo of a completed proof, never a substitute for one: it is
-- subject-bound, workspace-bound, single-use (consumed by DELETE .. RETURNING),
-- short-lived, and CLOSED — selecting an installation outside `candidates` is
-- refused, so the client chooses among proved options and can never introduce a
-- new one. The OAuth code is deliberately NOT stored: it is single-use and spent.
-- workspace_id CASCADEs (matching git_connections' own tenant FK, migration
-- 0094 / codex-security geyRc8 F12): a pending choice is workspace-owned
-- metadata, and a row that outlives its workspace is a selection nobody could
-- legitimately complete. Its subject is likewise deleted on account deletion
-- (ADR086, CleanupAccountSubject) — never anonymized, for the same reason.
-- Expiry alone is not a retention story: the sweep below is piggybacked on
-- writes, so without these two the last rows would sit with their subject until
-- somebody happened to start another claim. Raised by w2/037.
CREATE TABLE IF NOT EXISTS github_claim_selections (
    id           text PRIMARY KEY,
    workspace_id text        NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    subject      text        NOT NULL,
    -- [{"installationId": 123, "accountLogin": "acme"}, ...] — the proved set.
    candidates   jsonb       NOT NULL,
    created_at   timestamptz NOT NULL DEFAULT now(),
    expires_at   timestamptz NOT NULL
);

-- Backs the piggybacked expiry sweep (the flow is human-driven and rare, so a
-- DELETE on write keeps the table at "selections offered in the last few
-- minutes" without a janitor).
CREATE INDEX IF NOT EXISTS github_claim_selections_expires_idx
  ON github_claim_selections (expires_at);
