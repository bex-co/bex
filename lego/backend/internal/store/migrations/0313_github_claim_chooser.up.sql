-- w1/m526 (ADR078 §3b, 2026-11-19): the single connect entry.
--
-- The dashboard used to offer two buttons, "Connect another account" (the
-- install URL) and "Claim installed account" (the OAuth claim, §3a), and left
-- the user to know which one GitHub would honour: through the install URL an
-- account that already has the App dead-ends on GitHub's Configure page with
-- the signed state dropped. Every connect entry now starts the OAuth claim in
-- chooser mode, and the callback always hands the proved set to the account
-- chooser (zero, one or several candidates) instead of binding a sole
-- candidate on its own, because the user may have meant to install on a new
-- account instead.
--
-- The mode rides the server-side transaction row like the start-time
-- installation selector does, never the signed state, which stays a nonce that
-- authorizes nothing (w1/m67 F3). Default false keeps every existing claimGit
-- client on the §3a behaviour.
ALTER TABLE github_connect_transactions
  ADD COLUMN IF NOT EXISTS chooser boolean NOT NULL DEFAULT false;
