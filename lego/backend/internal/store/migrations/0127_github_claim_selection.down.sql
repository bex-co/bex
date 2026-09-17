-- Reverse w2/m162's claim selector (ADR078 §3a). Both objects hold only
-- in-flight browser state with a few minutes' TTL, so dropping them strands
-- nothing durable: an outstanding selection simply fails closed to the ordinary
-- ambiguous_installation message, whose remedy is to start the claim again.
DROP INDEX IF EXISTS github_claim_selections_expires_idx;
DROP TABLE IF EXISTS github_claim_selections;
ALTER TABLE github_connect_transactions DROP COLUMN IF EXISTS installation_id;
