-- ADR046: the signed checkout.session.completed webhook stamps the local
-- enforcement snapshot only after Stripe's authoritative setup objects have
-- been verified and both Customer/Subscription defaults have been bound.
ALTER TABLE billing_provider_mappings
    ADD COLUMN payment_method_bound_at timestamptz;
