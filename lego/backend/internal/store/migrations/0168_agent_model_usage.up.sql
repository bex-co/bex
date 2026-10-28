-- w10/m14 t003: platform model route token meters (ADR090 § Platform
-- OpenRouter route, ADR023).
--
-- The gateway model proxy reports each platform-key exchange's Anthropic
-- usage (input / output / cache-read / cache-write tokens) once per request.
-- agent_model_usage_events is the idempotency ledger: a request id is
-- recorded at most once, and only the winning insert adds its tokens to
-- usage_hourly (resource_kind 'agent_session', service_id = the session id,
-- tier = the OpenRouter model id), so a gateway retry can never double-count.
-- usage_hourly stays the one meter/outbox table the estimate and the billing
-- emitter read.
CREATE TABLE agent_model_usage_events (
    request_id         text        PRIMARY KEY,
    workspace_id       text        NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    session_id         text        NOT NULL,
    model              text        NOT NULL,
    window_start       timestamptz NOT NULL,
    input_tokens       bigint      NOT NULL DEFAULT 0 CHECK (input_tokens >= 0),
    output_tokens      bigint      NOT NULL DEFAULT 0 CHECK (output_tokens >= 0),
    cache_read_tokens  bigint      NOT NULL DEFAULT 0 CHECK (cache_read_tokens >= 0),
    cache_write_tokens bigint      NOT NULL DEFAULT 0 CHECK (cache_write_tokens >= 0),
    recorded_at        timestamptz NOT NULL DEFAULT now()
);

-- CompactUsage purges ledger rows with the hourly rows they fed.
CREATE INDEX agent_model_usage_events_window_idx ON agent_model_usage_events (window_start);

ALTER TABLE usage_hourly DROP CONSTRAINT usage_hourly_kind_check;
ALTER TABLE usage_hourly ADD CONSTRAINT usage_hourly_kind_check
    CHECK (kind IN ('instance_seconds', 'egress_bytes', 'build_seconds', 'storage_gb_seconds', 'sandbox_compute_seconds', 'disk_gb_seconds',
                    'agent_model_input_tokens', 'agent_model_output_tokens', 'agent_model_cache_read_tokens', 'agent_model_cache_write_tokens'));

ALTER TABLE usage_monthly DROP CONSTRAINT usage_monthly_kind_check;
ALTER TABLE usage_monthly ADD CONSTRAINT usage_monthly_kind_check
    CHECK (kind IN ('instance_seconds', 'egress_bytes', 'build_seconds', 'storage_gb_seconds', 'sandbox_compute_seconds', 'disk_gb_seconds',
                    'agent_model_input_tokens', 'agent_model_output_tokens', 'agent_model_cache_read_tokens', 'agent_model_cache_write_tokens'));
