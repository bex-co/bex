-- Multi-replica correctness for outbound webhooks (w1/m58, docs/ADR003-control-plane.md).
-- bex-api runs two replicas since w1/m52; the delivery worker's dispatch fan-out
-- and its per-endpoint failure-notice suppression were replica-local, so two
-- replicas double-delivered every event and double-emailed every failure notice.
-- This migration makes both durable/shared:
--
--   t002 — dedup delivery rows on (endpoint_id, event_id) so a concurrent
--          dispatch (or a re-read after a crash) can ON CONFLICT DO NOTHING
--          instead of inserting a second row with a fresh id. The send side
--          claims rows with FOR UPDATE SKIP LOCKED (no schema needed — it
--          leases via the existing next_attempt_at).
--   t003 — a notified_at marker next to the endpoint's disable state so the
--          "webhook failing/disabled" notice is suppressed across replicas and
--          restarts, not just within one process's memory.

-- t002: collapse any pre-existing duplicate (endpoint_id, event_id) delivery
-- rows the pre-claim double-dispatch bug may have created, keeping the row that
-- is furthest along (delivered > failed > most-attempted > earliest), so the
-- unique index below can be built and no already-delivered event is retried.
DELETE FROM webhook_deliveries
WHERE id IN (
    SELECT id FROM (
        SELECT id,
               row_number() OVER (
                   PARTITION BY endpoint_id, event_id
                   ORDER BY (delivered_at IS NOT NULL) DESC,
                            (failed_at IS NOT NULL) DESC,
                            attempt_count DESC,
                            created_at,
                            id
               ) AS rn
        FROM webhook_deliveries
    ) ranked
    WHERE rn > 1
);

CREATE UNIQUE INDEX IF NOT EXISTS webhook_deliveries_endpoint_event_uniq
    ON webhook_deliveries (endpoint_id, event_id);

-- t003: one timestamp per endpoint recording when the last failure notice was
-- sent. NULL = never notified (or re-enabled since). The worker compare-and-sets
-- it so exactly one replica emails per suppression window.
ALTER TABLE webhook_endpoints ADD COLUMN IF NOT EXISTS notified_at timestamptz;
