-- w21/104: a billing notice no longer records a Stripe mode. The mail's
-- "Stripe Live Mode" / "Stripe Test Mode" label is the platform's configured
-- mode, read when the notice is sent, so the value the store used to guess here
-- (the workspace's provider mapping, or "test" when it had none) is not stored
-- any more. Rows written before this keep theirs; new rows leave it NULL. The
-- column is dropped in a later migration, once no replica writes it.
ALTER TABLE billing_notifications ALTER COLUMN livemode DROP NOT NULL;
