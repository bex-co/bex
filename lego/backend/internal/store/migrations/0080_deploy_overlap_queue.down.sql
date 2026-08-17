DROP INDEX IF EXISTS deploys_one_queued_per_app_idx;
DROP INDEX IF EXISTS deploys_one_active_per_app_idx;

-- The previous schema permits only one open row. Preserve the executing row
-- when one exists; otherwise preserve the newest queued row, and settle every
-- other open row before restoring the old partial unique index.
WITH ranked AS (
    SELECT id,
           row_number() OVER (
               PARTITION BY app_id
               ORDER BY overlap_pending,
                        created_at DESC,
                        id DESC
           ) AS position
    FROM deploys
    WHERE status IN (
        'created',
        'queued',
        'build_in_progress',
        'pre_deploy_in_progress',
        'update_in_progress'
    )
)
UPDATE deploys AS d
SET status = 'canceled',
    overlap_pending = false,
    updated_at = GREATEST(d.updated_at + interval '1 microsecond', clock_timestamp()),
    finished_at = COALESCE(d.finished_at, clock_timestamp())
FROM ranked
WHERE d.id = ranked.id AND ranked.position > 1;

ALTER TABLE deploys
    DROP CONSTRAINT deploys_overlap_pending_status_check,
    DROP COLUMN overlap_pending;

CREATE UNIQUE INDEX deploys_one_open_per_app_idx ON deploys (app_id)
WHERE status IN (
    'created',
    'queued',
    'build_in_progress',
    'pre_deploy_in_progress',
    'update_in_progress'
);
