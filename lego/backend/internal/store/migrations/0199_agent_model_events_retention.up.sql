-- w10/m14 t013: agent_model_usage_events outlive the hourly rows they fed
-- (ADR091 D6, D10.7). Compaction no longer purges them after ~48h; they are
-- kept 13 months (an annual invoice period plus a dispute window) so the
-- session detail can show per-turn cost and a disputed charge traces to its
-- requests.
--
-- started_at is when the gateway sent the exchange upstream (the report's
-- StartedAt, clamped to the record time when absent or skewed). A streamed
-- exchange is reported after it ends, possibly after the next turn began, so
-- the per-turn read attributes a request by its start. NULL on rows recorded
-- before this column; the read falls back to recorded_at.
ALTER TABLE agent_model_usage_events ADD COLUMN started_at timestamptz;

CREATE INDEX agent_model_usage_events_session_idx
    ON agent_model_usage_events (session_id, recorded_at);
