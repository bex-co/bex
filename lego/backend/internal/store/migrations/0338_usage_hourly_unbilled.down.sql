-- Reverse 0338. Without the column a marked row would read as billed usage, so
-- the rows only ever shown go with it.
DELETE FROM usage_hourly WHERE unbilled;
ALTER TABLE usage_hourly DROP COLUMN IF EXISTS unbilled;
