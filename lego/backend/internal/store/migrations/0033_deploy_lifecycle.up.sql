-- w2/m38 migration 0033: persist Render's full evidence-backed deploy lifecycle. updated_at
-- advances only when a stored fact changes; existing rows are backfilled from
-- their last authoritative timestamp rather than migration wall-clock time.
ALTER TABLE deploys ADD COLUMN updated_at timestamptz;

UPDATE deploys
SET updated_at = COALESCE(finished_at, created_at);

ALTER TABLE deploys
    ALTER COLUMN updated_at SET NOT NULL,
    ALTER COLUMN updated_at SET DEFAULT now();

-- Older code could open overlapping update_in_progress rows. Preserve the
-- newest and settle every older one canceled so the new one-open-row invariant
-- starts true on upgrade as well as on fresh databases.
WITH ranked AS (
    SELECT id,
           row_number() OVER (PARTITION BY app_id ORDER BY created_at DESC, id DESC) AS position
    FROM deploys
    WHERE status = 'update_in_progress' AND finished_at IS NULL
)
UPDATE deploys AS d
SET status = 'canceled', updated_at = now(), finished_at = now()
FROM ranked
WHERE d.id = ranked.id AND ranked.position > 1;

CREATE UNIQUE INDEX deploys_one_open_per_app_idx ON deploys (app_id)
WHERE status IN (
    'created',
    'queued',
    'build_in_progress',
    'pre_deploy_in_progress',
    'update_in_progress'
);
CREATE INDEX deploys_app_updated_idx ON deploys (app_id, updated_at DESC);
