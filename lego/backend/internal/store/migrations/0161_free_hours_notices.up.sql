-- w8/m139: one row per (workspace, month, stage) claims a Free-instance-hours
-- email so concurrent bex-api replicas send it once. stage is 'warned' or
-- 'exhausted'; month is the first day of the UTC calendar month.
CREATE TABLE free_hours_notices (
    workspace_id text        NOT NULL,
    month        date        NOT NULL,
    stage        text        NOT NULL CHECK (stage IN ('warned', 'exhausted')),
    created_at   timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (workspace_id, month, stage)
);
