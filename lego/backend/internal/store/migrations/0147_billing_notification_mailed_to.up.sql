-- w5/m136: a billing notice records the owners it already reached, so a retry
-- after one owner's failed send mails only the owners still waiting for it.
ALTER TABLE billing_notifications
    ADD COLUMN IF NOT EXISTS mailed_to text[] NOT NULL DEFAULT '{}';

-- Notices already past the attempt cap (store.MaxBillingNotificationAttempts,
-- 24) are no longer claimed; due at infinity, the claim scan skips them too.
UPDATE billing_notifications SET next_attempt_at = 'infinity'
    WHERE delivered_at IS NULL AND attempt_count >= 24;
