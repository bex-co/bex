-- ADR112 D3: the Cloudflare-shaped object storage surface (Contract B) is
-- removed (w21/m500), and with it the one writer and reader of
-- object_buckets.location_hint, the location hint a Cloudflare-shaped bucket
-- create asked for. A bucket's region is exact; nothing else ever read the hint.
ALTER TABLE object_buckets
    DROP COLUMN location_hint;
