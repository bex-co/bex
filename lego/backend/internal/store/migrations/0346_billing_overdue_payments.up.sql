-- w6/m626: a card bound while dunning holds a workspace (grace, enforcing,
-- enforced) pays its overdue invoices at once instead of at Stripe's next
-- scheduled retry. Each invoice gets one attempt per card: a declined card is
-- never charged again for the same invoice, whether the member binds it
-- again, a webhook is redelivered or a second replica sees the same binding.
-- card_key is the card's fingerprint (a re-entered card gets a new
-- PaymentMethod id but keeps its fingerprint), or the PaymentMethod id for a
-- method that has none. An attempt still pending after a crash, or one that
-- failed for a reason other than the card, may be claimed again; attempts
-- counts the claims so each one gets its own Stripe idempotency key.
CREATE TABLE billing_overdue_payments (
    invoice_id   text NOT NULL,
    card_key     text NOT NULL,
    workspace_id text NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    outcome      text NOT NULL CHECK (outcome IN ('pending', 'paid', 'declined', 'error')),
    reason       text NOT NULL DEFAULT '',
    attempts     integer NOT NULL DEFAULT 1 CHECK (attempts > 0),
    attempted_at timestamptz NOT NULL,
    PRIMARY KEY (invoice_id, card_key)
);

CREATE INDEX billing_overdue_payments_workspace_idx ON billing_overdue_payments (workspace_id);
