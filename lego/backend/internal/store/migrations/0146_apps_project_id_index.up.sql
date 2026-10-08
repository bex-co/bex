-- w5/155: a project's services are read by apps.project_id, which no index
-- covered (0014 added the column without one), so every project list and
-- project view scanned every workspace's apps.
CREATE INDEX IF NOT EXISTS apps_project_id_idx
    ON apps (project_id);
