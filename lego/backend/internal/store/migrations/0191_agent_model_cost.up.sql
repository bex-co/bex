-- w10/m14 t004: cost-based agent model pricing (ADR091 D1–D3).
--
-- Each platform-route exchange now records its upstream cost (OpenRouter's
-- reported usage.cost, or a cost computed from the /api/v1/models price
-- table), where that cost came from, and the customer price
-- (cost × pricing.yaml agentModel.markup), all in micro-USD rounded half-up
-- per request. The price also lands in usage_hourly as the one billed
-- agent-model meter, agent_model_cost_microusd (tier = model,
-- resource_kind = agent_session); the four token meters stay for display.
ALTER TABLE agent_model_usage_events
    ADD COLUMN upstream_cost_microusd bigint NOT NULL DEFAULT 0 CHECK (upstream_cost_microusd >= 0),
    ADD COLUMN cost_source            text   NOT NULL DEFAULT 'unpriced'
        CHECK (cost_source IN ('reported', 'computed', 'unpriced')),
    ADD COLUMN price_microusd         bigint NOT NULL DEFAULT 0 CHECK (price_microusd >= 0);

ALTER TABLE usage_hourly DROP CONSTRAINT usage_hourly_kind_check;
ALTER TABLE usage_hourly ADD CONSTRAINT usage_hourly_kind_check
    CHECK (kind IN ('instance_seconds', 'egress_bytes', 'build_seconds', 'storage_gb_seconds', 'sandbox_compute_seconds', 'disk_gb_seconds',
                    'agent_model_input_tokens', 'agent_model_output_tokens', 'agent_model_cache_read_tokens', 'agent_model_cache_write_tokens',
                    'agent_model_cost_microusd'));

ALTER TABLE usage_monthly DROP CONSTRAINT usage_monthly_kind_check;
ALTER TABLE usage_monthly ADD CONSTRAINT usage_monthly_kind_check
    CHECK (kind IN ('instance_seconds', 'egress_bytes', 'build_seconds', 'storage_gb_seconds', 'sandbox_compute_seconds', 'disk_gb_seconds',
                    'agent_model_input_tokens', 'agent_model_output_tokens', 'agent_model_cache_read_tokens', 'agent_model_cache_write_tokens',
                    'agent_model_cost_microusd'));
