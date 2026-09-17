-- "Opened checkout" and "bound a card" are different facts, and until now only
-- the second had a column. EnsureContract mints the Stripe Customer and
-- Subscription BEFORE the payment page renders, so a row with a live
-- subscription_id has always meant merely that someone reached checkout — while
-- reading exactly like a paying customer. Twelve cardless workspaces looked
-- bound that way on 2026-09-16.
--
-- payment_method_bound_at remains the sole authority on payment state; this
-- column exists so nothing has to infer that state from subscription_id.
-- Existing rows stay NULL: their checkout time was never recorded and inventing
-- one would be worse than admitting it is unknown.
ALTER TABLE billing_provider_mappings
    ADD COLUMN IF NOT EXISTS checkout_started_at timestamptz;

COMMENT ON COLUMN billing_provider_mappings.checkout_started_at IS
    'When a hosted Checkout session was first created for this workspace. Proves intent only — payment_method_bound_at is the sole proof that a card was bound. NULL means unknown (pre-0123 row), not "never".';
