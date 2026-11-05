-- ADR091 D10.2–D10.3: agent-model threshold billing state, open failed
-- invoices, and agent-model bad debt.

-- The Stripe agent-model threshold last applied for the workspace. The
-- threshold sweep (billing.AgentModelThresholds) picks a row whose trust record
-- changed since (threshold_synced_version, the updated_at it applied, differs
-- from updated_at) or whose payment lock expired after the value was computed
-- (threshold_synced_at), skipping a row in failure backoff
-- (threshold_next_attempt_at). Both are versions, not wall-clock windows, so a
-- webhook racing a sweep leaves the row due rather than skipped. A hand edit of
-- billing_trust must set updated_at = now() for the sweep to see it.
ALTER TABLE billing_trust
    ADD COLUMN threshold_synced_version  timestamptz,
    ADD COLUMN threshold_synced_microusd bigint,
    ADD COLUMN threshold_synced_at       timestamptz,
    ADD COLUMN threshold_attempts        integer NOT NULL DEFAULT 0,
    ADD COLUMN threshold_next_attempt_at timestamptz;

-- Every bex invoice whose payment failed, for the mint's BILLING_BALANCE_DUE
-- refusal: open while failed_at is set and settled_at is not. invoice.paid and
-- invoice.voided settle it; a settlement that arrives first leaves a row with
-- only settled_at, so a late invoice.payment_failed cannot reopen it. An invoice
-- marked uncollectible stays open (it is still owed). The T0 lock
-- (billing_trust.failed_invoice_id) is separate and unchanged.
CREATE TABLE billing_trust_failed_invoices (
    workspace_id text        NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    invoice_id   text        NOT NULL,
    failed_at    timestamptz,
    settled_at   timestamptz,
    PRIMARY KEY (workspace_id, invoice_id)
);

-- One row per bex invoice marked uncollectible that carried agent-model spend.
-- agent_model_net_microusd is the invoice's agent-model lines after discounts;
-- amount_microusd is the written-off share of it (prorated by
-- amount_remaining / total). usage_month is the calendar month (UTC) the
-- agent-model usage was billed for, so the D10 prepaid trigger compares it with
-- the same month's revenue. workspace_id and customer_id are plain text, not
-- foreign keys: the debt record outlives the workspace (ADR086 amendment
-- 2026-11-05: kept 7 years, then purged by the billing worker); a future D10.6
-- card-fingerprint check may read it. A later payment of the invoice deletes
-- its row.
CREATE TABLE billing_agent_model_writeoffs (
    invoice_id               text        PRIMARY KEY,
    workspace_id             text        NOT NULL,
    customer_id              text        NOT NULL,
    usage_month              date        NOT NULL,
    agent_model_net_microusd bigint      NOT NULL CHECK (agent_model_net_microusd > 0),
    amount_microusd          bigint      NOT NULL CHECK (amount_microusd > 0 AND amount_microusd <= agent_model_net_microusd),
    written_off_at           timestamptz NOT NULL
);
-- Readers: the month's bad debt, and the mint's unpaid-write-off refusal.
CREATE INDEX billing_agent_model_writeoffs_month_idx ON billing_agent_model_writeoffs (usage_month);
CREATE INDEX billing_agent_model_writeoffs_workspace_idx ON billing_agent_model_writeoffs (workspace_id);

-- Agent-model revenue (agent_model_cost_microusd, the metered customer price)
-- per calendar month (UTC), across every workspace. usage_hourly cascades with
-- its workspace, so the threshold sweep snapshots the month here and only ever
-- raises it: a workspace deleted later in the month keeps its revenue counted.
CREATE TABLE billing_agent_model_revenue_months (
    month            date        PRIMARY KEY,
    revenue_microusd bigint      NOT NULL CHECK (revenue_microusd >= 0),
    refreshed_at     timestamptz NOT NULL
);

-- The snapshot's cross-workspace month sum. The predicate is the query's
-- literal resource_kind.
CREATE INDEX usage_hourly_agent_session_kind_idx ON usage_hourly (kind, window_start)
    WHERE resource_kind = 'agent_session';
