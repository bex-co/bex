-- w2/m254 t013: the build source build a deploy shipped (Render's
-- deploy.buildId, a bld- id). NULL for every deploy not built by a build
-- source.
ALTER TABLE deploys ADD COLUMN build_id text;
