-- w21/m260 (ADR094 D8): a per-session ceiling on the platform-route model
-- spend, in micro-USD of the ADR091 customer price. NULL = no session ceiling
-- (only the workspace's monthly cap applies). The gateway model mint compares
-- the session's recorded agent_model_usage_events price with it before each
-- model request.
ALTER TABLE agent_sessions
    ADD COLUMN max_spend_microusd bigint CHECK (max_spend_microusd > 0);
