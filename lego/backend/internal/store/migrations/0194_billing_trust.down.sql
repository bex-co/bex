-- Reverse 0193.
DROP INDEX agent_sessions_active_workspace_idx;
DROP INDEX usage_hourly_agent_session_idx;
ALTER TABLE agent_environments DROP COLUMN agent_model_spend_cap_override_microusd;
DROP TABLE billing_trust_invoices;
DROP TABLE billing_trust;
