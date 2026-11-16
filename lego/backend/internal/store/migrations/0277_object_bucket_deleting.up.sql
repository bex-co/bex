-- w4/678: a bucket delete checks the bucket is empty, then drops its row, and
-- an upload that lands between the two would sit under a prefix no row maps.
-- DeleteObjectBucketIfEmpty marks the row here before it lists the bucket, so
-- a streamed put refuses a bucket being deleted before its first byte. A mark
-- older than objectBucketDeletingTTL (two minutes) is a delete that died mid-way and no
-- longer refuses anything. NULL while no delete is running.
ALTER TABLE object_buckets ADD COLUMN deleting_since timestamptz;
