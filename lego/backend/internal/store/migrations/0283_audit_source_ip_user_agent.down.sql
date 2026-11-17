-- Reverse w9/m450: drop where each audit row came from.
ALTER TABLE audit_events
  DROP COLUMN IF EXISTS user_agent,
  DROP COLUMN IF EXISTS source_ip;
