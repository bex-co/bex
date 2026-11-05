DROP INDEX IF EXISTS usage_hourly_workspace_pending_idx;
DROP INDEX IF EXISTS billing_agent_model_writeoffs_fingerprint_idx;
ALTER TABLE billing_agent_model_writeoffs DROP COLUMN IF EXISTS card_fingerprint;
ALTER TABLE billing_provider_mappings DROP COLUMN IF EXISTS payment_method_rejection;
