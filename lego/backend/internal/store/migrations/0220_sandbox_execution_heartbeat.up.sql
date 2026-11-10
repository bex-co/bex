-- w8/m151: an open execution's stream refreshes seen_at while it runs, so a
-- record whose stream ended without a stop (a bex-api restart mid-exec) is
-- closed at its last heartbeat instead of reading "running" forever.
ALTER TABLE sandbox_executions ADD COLUMN seen_at timestamptz;
