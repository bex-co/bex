-- w32/m608: Record each Git-backed deploy's own repo and branch at deploy
-- creation/promotion so switching a service's --repo, --branch, or --image
-- never rewrites historical build progress lines (`==> Build queued`,
-- `==> Building from <repo>@<commit|branch>`, `==> Deploying...`) or churns
-- their deterministic log IDs. Empty strings on pre-migration rows fall back
-- to the App's current source when `image` is also empty.
ALTER TABLE deploys
  ADD COLUMN IF NOT EXISTS repo text NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS branch text NOT NULL DEFAULT '';
