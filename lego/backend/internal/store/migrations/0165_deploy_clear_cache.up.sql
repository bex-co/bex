-- w4/413: a deploy opened with clearCache=clear records it on the row, so
-- the events feed can report trigger.clearCache long after the
-- release-scoped App annotation has moved on.
ALTER TABLE deploys ADD COLUMN clear_cache boolean NOT NULL DEFAULT false;
