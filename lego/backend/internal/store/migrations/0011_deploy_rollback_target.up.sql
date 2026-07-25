-- Rollback (w2/m10) needs to know the exact image a deploy put into the
-- cluster, not just what was requested at trigger time (image is '' for a
-- build-from-git deploy until the build resolves it — see 0005_deploys). The
-- reconciler's write-back backfills resolved_image the moment a deploy
-- reaches live (store.CloseDeploy), so it is the only field trustworthy
-- enough for Rollback to restore blind: repointing spec.image at an
-- already-pushed tag Zot still holds bypasses the build pipeline entirely
-- (docs/ADR004-app-deployment.md). Only a deploy that itself went live ever gets
-- one, so an in-progress/failed/canceled deploy is correctly never a valid
-- rollback target.
--
-- rollback_of is provenance, not a foreign key: deploys are only ever
-- bulk-deleted via the app's ON DELETE CASCADE, never individually, so
-- referential integrity buys nothing here. It's the source deploy id a
-- "rollback"-triggered deploy restores, surfaced as a bex extra so an agent
-- can see why a deploy exists.
--
-- generation records the App CR's metadata.generation this deploy actually
-- ran under, captured the instant the row opens (after the spec patch that
-- triggered it, so it's the post-bump value) — Cancel's build-Job identity
-- (bld-<name>-gen-<generation>, lego/operator/internal/build.JobName) must
-- be derived from THIS, not a fresh re-fetch of the App's current
-- generation: an unrelated spec write racing after the deploy opened (a
-- scale, an env-var change, another trigger) would otherwise make Cancel
-- compute the wrong Job name and silently no-op past the real build.
ALTER TABLE deploys
    ADD COLUMN resolved_image text NOT NULL DEFAULT '',
    ADD COLUMN rollback_of text NOT NULL DEFAULT '',
    ADD COLUMN generation bigint NOT NULL DEFAULT 0;
