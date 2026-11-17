-- Reverse 0280: rows written without a mode read as test, which is what the
-- store recorded for a workspace that had no provider mapping.
UPDATE billing_notifications SET livemode = false WHERE livemode IS NULL;
ALTER TABLE billing_notifications ALTER COLUMN livemode SET NOT NULL;
