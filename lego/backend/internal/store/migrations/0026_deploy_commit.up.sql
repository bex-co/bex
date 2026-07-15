-- w9/001: per-deploy commit metadata. 0005 reserved `commit` (the resolved
-- SHA, always '' until now); the message needs its own column. Both are
-- written once at deploy-open time by the workspace's GitHub App connection
-- resolving the triggering ref — '' when the app is image-backed, the
-- workspace has no connection, or resolution failed (omitted, not faked).
ALTER TABLE deploys ADD COLUMN commit_message text NOT NULL DEFAULT '';
