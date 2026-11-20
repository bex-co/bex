-- Reverse w1/m526's chooser mode (ADR078 §3b). The column holds only in-flight
-- browser state with a 15-minute TTL; an outstanding chooser-mode attempt
-- completes as an ordinary §3a claim afterwards.
ALTER TABLE github_connect_transactions DROP COLUMN IF EXISTS chooser;
