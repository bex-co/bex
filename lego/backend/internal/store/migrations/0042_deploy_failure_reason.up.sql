-- w9/011: carry an actionable failure reason on failed deploys. Stamped from
-- the App CR's Ready-condition message when the reconciler closes a deploy
-- build_failed / pre_deploy_failed / update_failed, so "update_failed" is no
-- longer an opaque terminal state whose cause lives only in pod state no
-- tenant surface shows (crash loop, image-pull failure, port-bind denial).
ALTER TABLE deploys
    ADD COLUMN failure_reason TEXT NOT NULL DEFAULT '';
