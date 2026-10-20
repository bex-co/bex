-- Render's deploy image.sha (w8/m122): the digest a deploy's image reference
-- resolved to. The reconciler stamps it on the deploy's live close from the
-- App's status.imageDigest for that release, beside resolved_image. '' is
-- unknown — a deploy that never went live, one whose digest the operator
-- could not observe, or one closed before this column existed — and the
-- surfaces omit it rather than fake it.
ALTER TABLE deploys
    ADD COLUMN image_sha text NOT NULL DEFAULT '';
