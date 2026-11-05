-- ADR091 D5, D10.1: agent-model trust tiers and spend caps.
--
-- billing_trust is the durable per-workspace payment record the trust tier is
-- a pure function of (with pricing.yaml agentModel.trustTiers).
-- stripe_billing_events cannot serve: it stores no amounts and is purged after
-- 90 days. lifetime_paid_cents sums amount_paid over every paid, non-zero bex
-- invoice; paid_cycle_invoices counts those with billing_reason
-- subscription_cycle. A workspace with no row is T0 with no lock.
--
-- Two independent holds pin a workspace to T0:
--   * A failed invoice payment sets locked_until = 'infinity' and
--     failed_invoice_id. Only that invoice's payment (paid at or after the
--     failure) settles it, turning locked_until into paid_at + 30 days.
--   * charge.dispute.created sets dispute_id. Only charge.dispute.closed with
--     status "won" for that dispute clears it (won_dispute_id remembers the
--     win so a late-delivered creation cannot re-set it). A lost dispute keeps
--     the hold; ops lifts it by hand once the account is settled:
--       UPDATE billing_trust SET dispute_id = NULL WHERE workspace_id = '<ws>';
CREATE TABLE billing_trust (
    workspace_id        text        PRIMARY KEY REFERENCES tenants (id) ON DELETE CASCADE,
    lifetime_paid_cents bigint      NOT NULL DEFAULT 0 CHECK (lifetime_paid_cents >= 0),
    paid_cycle_invoices integer     NOT NULL DEFAULT 0 CHECK (paid_cycle_invoices >= 0),
    locked_until        timestamptz,
    failed_invoice_id   text,
    last_failure_at     timestamptz,
    dispute_id          text,
    won_dispute_id      text,
    updated_at          timestamptz NOT NULL DEFAULT now()
);

-- One row per counted invoice, so a redelivered invoice.paid (or
-- invoice.payment_succeeded for the same invoice) never counts twice.
CREATE TABLE billing_trust_invoices (
    invoice_id   text        PRIMARY KEY,
    workspace_id text        NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    amount_cents bigint      NOT NULL CHECK (amount_cents > 0),
    cycle        boolean     NOT NULL,
    paid_at      timestamptz NOT NULL
);
CREATE INDEX billing_trust_invoices_workspace_idx ON billing_trust_invoices (workspace_id, paid_at);

-- The admin's monthly agent-model spend cap override (ADR091 D5), with the
-- other workspace agent settings, in agent_model_cost_microusd's unit. NULL =
-- the plan default.
ALTER TABLE agent_environments
    ADD COLUMN agent_model_spend_cap_override_microusd bigint
        CHECK (agent_model_spend_cap_override_microusd IS NULL OR agent_model_spend_cap_override_microusd > 0);

-- The mint's month-to-date agent-model cost sum (store.AgentSpendInputs).
-- The predicate is the query's literal resource_kind.
CREATE INDEX usage_hourly_agent_session_idx ON usage_hourly (workspace_id, kind, window_start)
    WHERE resource_kind = 'agent_session';

-- The per-workspace active-session counts (live-sandbox cap, trust-tier
-- concurrency). The predicate is store.activeAgentSessionPhasesSQL.
CREATE INDEX agent_sessions_active_workspace_idx ON agent_sessions (workspace_id)
    WHERE phase IN ('creating', 'running', 'resuming', 'redispatching');
