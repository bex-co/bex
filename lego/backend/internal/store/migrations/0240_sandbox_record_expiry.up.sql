-- w32/017: a sandbox's lifetime deadline as bex computes it (createdAt +
-- timeoutSeconds, moved by suspend and resume; NULL for none), so the sweeper
-- ends a sandbox OpenSandbox deleted at that deadline at once, at that time,
-- instead of keeping a young record live under its ten-minute create guard.
ALTER TABLE sandbox_records ADD COLUMN expires_at timestamptz;
