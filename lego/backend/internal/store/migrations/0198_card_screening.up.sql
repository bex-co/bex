-- ADR091 D10.4–D10.6: settle before delete, and card checks at payment-method binding.

-- Why the last payment method offered for the workspace was refused at
-- binding, for the payment gate and the dashboard: 'prepaid_card' (D10.5) or
-- 'unpaid_balance' (D10.6, the card's fingerprint matches an unpaid
-- agent-model write-off). Cleared (NULL) when a method binds, so a later
-- successful binding never shows a stale refusal. The refused method is
-- detached from the Stripe Customer, and payment_method_bound_at is never set
-- by it.
ALTER TABLE billing_provider_mappings
    ADD COLUMN payment_method_rejection text CHECK (payment_method_rejection IN ('prepaid_card', 'unpaid_balance'));

-- The Stripe fingerprint of the card whose charge of the written-off invoice
-- failed (else the Customer's default card) when the write-off was recorded;
-- NULL when there was none, or it was not a card.
-- Binding a card with the same fingerprint, in any workspace, is refused while
-- the row exists (a paid invoice deletes it). A fingerprint identifies a card
-- number to this Stripe account only; it is never logged.
ALTER TABLE billing_agent_model_writeoffs
    ADD COLUMN card_fingerprint text;
CREATE INDEX billing_agent_model_writeoffs_fingerprint_idx ON billing_agent_model_writeoffs (card_fingerprint)
    WHERE card_fingerprint IS NOT NULL;

-- ADR091 D10.4 settle before delete reads one workspace's pending usage. The
-- outbox's usage_hourly_billing_pending_idx is keyed on window_start alone,
-- so that read would walk every workspace's pending backlog (the whole seal
-- window) to find one workspace's rows.
CREATE INDEX usage_hourly_workspace_pending_idx ON usage_hourly (workspace_id, window_start)
    WHERE billing_export_state = 'pending';
