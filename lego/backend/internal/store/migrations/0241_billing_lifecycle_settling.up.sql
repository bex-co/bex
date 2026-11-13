-- w10/m14 (ADR091 D10.4): settle before delete resets the billing cycle
-- anchor with payment_behavior=default_incomplete, which leaves the
-- Subscription past_due until bex pays the new invoice moments later. The
-- settle stamps settling_until (about ten minutes ahead) before the reset, and
-- a subscription_past_due observation made before it is not a payment failure.
ALTER TABLE billing_lifecycles ADD COLUMN settling_until timestamptz;
