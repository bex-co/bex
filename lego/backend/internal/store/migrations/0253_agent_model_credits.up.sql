-- ADR091 D10.2a (w6/m362 t017): billing credits pay agent-model usage before
-- the card does. agent_model_credit_microusd is the workspace's available
-- metered credit balance (Stripe Billing Credit Grants scoped to metered
-- prices, less grants that expire before the subscription's current period
-- ends), as the threshold charger last read it at agent_model_credit_read_at.
-- The charge window's unbilled spend is reduced by it only while the read is
-- inside the window: an invoice that bills the meter consumes credits, so
-- moving agent_model_billed_until clears both columns and the next crossing
-- reads the balance again. NULL: not read in this window. From this migration
-- on the window opens at agent_model_billed_until even when that is in the
-- previous month (one window per invoice, so credits are counted once).
ALTER TABLE billing_provider_mappings
  ADD COLUMN agent_model_credit_microusd bigint,
  ADD COLUMN agent_model_credit_read_at timestamptz;
