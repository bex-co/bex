-- ADR091 D10.2a: bex charges the card the moment a
-- workspace's unbilled agent-model spend reaches its trust tier's charge
-- threshold, synchronously inside the model-credential mint, instead of
-- through a Stripe item billing_threshold.
--
-- agent_model_threshold_charges is the charge ledger: one row per crossing,
-- numbered per workspace and calendar month (UTC). amount_microusd is the
-- unbilled spend the crossing charged (month-to-date minus the month's earlier
-- paid charges); amount_cents what the standalone Stripe invoice bills.
--   pending  the charge is being made, or Stripe could not decide it yet (the
--            sweep retries it; attempts counts the undecided tries)
--   paid     the card paid invoice_id; net_item_id is the negative pending
--            invoice item on the workspace's Subscription that keeps the next
--            cycle invoice from billing the same usage again (NULL until made)
--   failed   the card was declined or needs authentication; failure is the
--            Stripe decline code and hosted_invoice_url where the customer
--            pays it (paying it later turns the row paid)
CREATE TABLE agent_model_threshold_charges (
    workspace_id       text        NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    month              date        NOT NULL,
    seq                integer     NOT NULL CHECK (seq > 0),
    amount_microusd    bigint      NOT NULL CHECK (amount_microusd > 0),
    amount_cents       bigint      NOT NULL CHECK (amount_cents > 0),
    status             text        NOT NULL CHECK (status IN ('pending', 'paid', 'failed')),
    invoice_id         text,
    hosted_invoice_url text,
    failure            text,
    net_item_id        text,
    attempts           integer     NOT NULL DEFAULT 0,
    created_at         timestamptz NOT NULL DEFAULT now(),
    settled_at         timestamptz,
    PRIMARY KEY (workspace_id, month, seq)
);

-- At most one charge per workspace is in flight, across replicas: a second
-- crossing joins the pending one (store.BeginAgentModelCharge).
CREATE UNIQUE INDEX agent_model_threshold_charges_pending_idx
    ON agent_model_threshold_charges (workspace_id) WHERE status = 'pending';

-- Webhooks for a charge invoice (paid later through its hosted page, or a
-- failure Stripe decided after the mint gave up) find the row by invoice.
CREATE UNIQUE INDEX agent_model_threshold_charges_invoice_idx
    ON agent_model_threshold_charges (invoice_id) WHERE invoice_id IS NOT NULL;

-- The retry sweep reads pending charges and paid ones not yet netted.
CREATE INDEX agent_model_threshold_charges_retry_idx
    ON agent_model_threshold_charges (created_at, workspace_id, seq)
    WHERE status = 'pending' OR (status = 'paid' AND net_item_id IS NULL);

-- Stripe item thresholds are retired: the threshold sweep now wants none for
-- every workspace. Forgetting every applied value makes each workspace due, so
-- the sweep removes the billing_thresholds Stripe still holds.
UPDATE billing_trust SET threshold_synced_version = NULL;
