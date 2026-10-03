-- Opening Checkout used to mint a Stripe Customer + cardless Subscription
-- before the hosted page rendered, and nothing ever reclaimed them when the
-- user walked away. The abandoned-checkout reclaimer revisits each
-- unbound mapping at most once a day; this column is both its claim and its
-- "already looked" marker, so a mapping it keeps (open session, real invoice,
-- attached card) is not re-read from Stripe on every pass.
ALTER TABLE billing_provider_mappings
    ADD COLUMN IF NOT EXISTS reclaim_checked_at timestamptz;

COMMENT ON COLUMN billing_provider_mappings.reclaim_checked_at IS
    'Last time the abandoned-checkout reclaimer examined this unbound mapping. NULL means never examined.';
