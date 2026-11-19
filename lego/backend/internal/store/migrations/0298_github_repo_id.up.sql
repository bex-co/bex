-- w10/m490 (docs/ADR026-github-integration.md § Repository identity): a push
-- matches the services and Blueprints built from its repository by GitHub's
-- numeric repository id, which survives a rename or a transfer, not only by
-- the URL bex stored. `tianpan-v4-web` missed a month of pushes after its
-- repository was renamed, because the push named the new URL and the row the
-- old one.
--
-- 0 means no id is known: a repository on another host, a public repository
-- the workspace's GitHub App installation does not grant, or a row from before
-- this migration that bex-api's backfill (apps.RepoIdentityBackfill) has not
-- resolved yet. Such a row keeps matching pushes by URL.
ALTER TABLE apps
  ADD COLUMN IF NOT EXISTS github_repo_id bigint NOT NULL DEFAULT 0;
ALTER TABLE blueprints
  ADD COLUMN IF NOT EXISTS github_repo_id bigint NOT NULL DEFAULT 0;

-- The audit row a followed rename records names the repository URL on each
-- side, one typed column per value as the other audit details. NULL for every
-- other row.
ALTER TABLE audit_events
  ADD COLUMN IF NOT EXISTS repo_from text,
  ADD COLUMN IF NOT EXISTS repo_to text;
