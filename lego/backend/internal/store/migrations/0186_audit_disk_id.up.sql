-- w2/064: Render's disk_snapshot_restored event carries the restored disk's
-- dsk- id. One typed column, like every other per-verb audit detail; NULL for
-- every other verb and for restores recorded before this column existed.
ALTER TABLE audit_events ADD COLUMN disk_id text;
