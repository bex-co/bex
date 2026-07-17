-- w2/m42: commit author timestamp. 0026 added commit+commit_message for the
-- resolved commit SHA + message; this column adds the author-date captured
-- from the same GitHub commit object at deploy-open time. NULL = unknown
-- (image-backed, no connection, or resolution failed) — omitted, not faked.
ALTER TABLE deploys ADD COLUMN commit_author_at timestamptz NULL;
