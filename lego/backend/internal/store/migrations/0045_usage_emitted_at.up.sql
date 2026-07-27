-- Billing export outbox (docs/ADR040-billing-metronome.md §4). A nullable
-- emitted_at lets the external-billing emitter track which sealed usage_hourly rows
-- have shipped, exactly-once, without a separate table: it selects
-- emitted_at IS NULL rows past the seal horizon, ingests them, then stamps
-- emitted_at. Nullable ⇒ every existing row reads as "not yet emitted"; the
-- epoch floor (currently BEX_STRIPE_EPOCH) — not this column — bounds how far back the
-- emitter ever ships. The primary key and every existing read path are
-- unchanged; only the emitter selects this column.
ALTER TABLE usage_hourly ADD COLUMN emitted_at timestamptz;

-- The emitter's outbox scan runs hourly:
--   WHERE emitted_at IS NULL AND window_start >= $floor AND window_start < $seal
--   ORDER BY window_start LIMIT $n
-- In steady state almost every row is already emitted, so a partial index over
-- just the un-emitted rows keeps that scan proportional to the outbox depth (not
-- the whole hot window) and serves both the window_start range and the ORDER BY.
CREATE INDEX usage_hourly_unemitted_idx ON usage_hourly (window_start) WHERE emitted_at IS NULL;
