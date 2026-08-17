-- w2/018: an overlapping trigger replaces only the previous waiting deploy.
-- The release already executing remains open until the operator observes it
-- live/failed; this lets deploy history record the generation that actually
-- served traffic before the latest pending generation takes over.
DROP INDEX deploys_one_open_per_app_idx;

ALTER TABLE deploys
    ADD COLUMN overlap_pending boolean NOT NULL DEFAULT false,
    ADD CONSTRAINT deploys_overlap_pending_status_check
        CHECK (NOT overlap_pending OR status = 'queued');

CREATE UNIQUE INDEX deploys_one_active_per_app_idx ON deploys (app_id)
WHERE NOT overlap_pending AND status IN (
    'created',
    'queued',
    'build_in_progress',
    'pre_deploy_in_progress',
    'update_in_progress'
);

CREATE UNIQUE INDEX deploys_one_queued_per_app_idx ON deploys (app_id)
WHERE overlap_pending AND status = 'queued';
