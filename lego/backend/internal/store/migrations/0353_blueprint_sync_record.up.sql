-- w3/m678 (docs/render-artifacts/blueprint-pages.md § A sync's page): every
-- Blueprint sync has its own page, as on Render, which names the commit the
-- sync read with that commit's message, lists the actions it planned and shows
-- the errors that failed it.
--
-- commit_message is the message of commit_id, read from the Git host when the
-- sync is recorded, as deploys.commit_message is (0026): '' when the host could
-- not say (no connection, a lookup failure, no commit at all). Provenance is
-- never faked and never fails a sync.
--
-- error_notices are a failed plan's located notices (Render's planning errors:
-- severity, message, path, line, column), the JSON the apps layer owns. NULL
-- for every other row, whose error is error_message alone.
--
-- plan_actions (0187) is now written for every sync that plans, not only for a
-- planned one; a row from before this migration keeps NULL, which the page
-- reads as "actions not recorded".
ALTER TABLE blueprint_syncs
  ADD COLUMN IF NOT EXISTS commit_message text NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS error_notices jsonb;
