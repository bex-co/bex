-- w8/m25: separate one logical endpoint/event notification from every
-- scheduled network attempt. webhook_deliveries remains the durable parent so
-- its established (endpoint_id,event_id) uniqueness, payload ownership, retry
-- budget, and endpoint cascade stay intact. This child is both the send queue
-- and the immutable-after-completion evidence history.
ALTER TABLE webhook_deliveries
    ADD CONSTRAINT webhook_deliveries_id_endpoint_uniq UNIQUE (id, endpoint_id);

CREATE TABLE webhook_delivery_attempts (
    id                  text PRIMARY KEY,
    notification_id     text NOT NULL,
    endpoint_id         text NOT NULL REFERENCES webhook_endpoints(id) ON DELETE CASCADE,
    attempt_number      integer NOT NULL CHECK (attempt_number > 0),
    status              text NOT NULL DEFAULT 'pending'
                        CHECK (status IN ('pending', 'delivered', 'failed')),
    origin              text NOT NULL DEFAULT 'automatic'
                        CHECK (origin IN ('automatic', 'manual')),
    requested_by        text NOT NULL DEFAULT '' CHECK (octet_length(requested_by) <= 512),
    idempotency_key     text NOT NULL DEFAULT '' CHECK (octet_length(idempotency_key) <= 256),
    available_at        timestamptz NOT NULL,
    lease_until         timestamptz,
    sent_at             timestamptz,
    status_code         integer NOT NULL DEFAULT 0 CHECK (status_code BETWEEN 0 AND 999),
    transport_error     text NOT NULL DEFAULT '' CHECK (octet_length(transport_error) <= 2048),
    response_body       text NOT NULL DEFAULT '' CHECK (octet_length(response_body) <= 4096),
    -- A manual send temporarily owns the notification's one send slot. If it
    -- superseded an unsent automatic retry, this preserves that retry's exact
    -- due time so failure can restore it without consuming the automatic
    -- eight-attempt budget; success deliberately drops it.
    resume_automatic_at timestamptz,
    created_at          timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT webhook_delivery_attempts_notification_number_uniq
        UNIQUE (notification_id, attempt_number),
    CONSTRAINT webhook_delivery_attempts_notification_endpoint_fk
        FOREIGN KEY (notification_id, endpoint_id)
        REFERENCES webhook_deliveries(id, endpoint_id) ON DELETE CASCADE,
    CONSTRAINT webhook_delivery_attempts_id_check
        CHECK (id ~ '^whd-[0-9a-v]{20}$'),
    CONSTRAINT webhook_delivery_attempts_state_check CHECK (
        (status = 'pending' AND sent_at IS NULL AND status_code = 0
         AND transport_error = '' AND response_body = '') OR
        (status = 'delivered' AND sent_at IS NOT NULL
         AND status_code BETWEEN 200 AND 299 AND transport_error = '') OR
        -- A response-body read can fail after a 2xx status line, so failure is
        -- not equivalent to a non-2xx code; transport_error carries the cause.
        (status = 'failed' AND sent_at IS NOT NULL)
    ),
    CONSTRAINT webhook_delivery_attempts_manual_key_check CHECK (
        (origin = 'automatic' AND requested_by = '' AND idempotency_key = ''
         AND resume_automatic_at IS NULL) OR
        (origin = 'manual' AND requested_by <> '' AND idempotency_key <> '')
    )
);

-- A reservation's identity/request metadata never changes, and terminal
-- evidence is append-only. The worker may update only the lease and perform
-- the pending -> terminal transition. DELETE remains available for retention,
-- endpoint cascades, and superseding an unsent automatic retry with Resend.
CREATE FUNCTION bex_guard_webhook_attempt_update()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    IF OLD.status <> 'pending' THEN
        RAISE EXCEPTION 'terminal webhook attempt % is immutable', OLD.id
            USING ERRCODE = '23514';
    END IF;
    IF NEW.id IS DISTINCT FROM OLD.id
       OR NEW.notification_id IS DISTINCT FROM OLD.notification_id
       OR NEW.endpoint_id IS DISTINCT FROM OLD.endpoint_id
       OR NEW.attempt_number IS DISTINCT FROM OLD.attempt_number
       OR NEW.origin IS DISTINCT FROM OLD.origin
       OR NEW.requested_by IS DISTINCT FROM OLD.requested_by
       OR NEW.idempotency_key IS DISTINCT FROM OLD.idempotency_key
       OR NEW.available_at IS DISTINCT FROM OLD.available_at
       OR NEW.resume_automatic_at IS DISTINCT FROM OLD.resume_automatic_at
       OR NEW.created_at IS DISTINCT FROM OLD.created_at THEN
        RAISE EXCEPTION 'webhook attempt % reservation identity is immutable', OLD.id
            USING ERRCODE = '23514';
    END IF;
    RETURN NEW;
END;
$$;

CREATE TRIGGER webhook_delivery_attempts_immutable
BEFORE UPDATE ON webhook_delivery_attempts
FOR EACH ROW EXECUTE FUNCTION bex_guard_webhook_attempt_update();

-- Exactly one replay reservation/result for an endpoint-scoped caller key.
-- Retaining the key on the terminal attempt makes an arbitrarily late repeat
-- idempotent, not merely two concurrent requests.
CREATE UNIQUE INDEX webhook_delivery_attempts_manual_key_uniq
    ON webhook_delivery_attempts (endpoint_id, idempotency_key)
    WHERE origin = 'manual';

-- The queue is a small partial working set and claims oldest-due first.
CREATE INDEX webhook_delivery_attempts_due_idx
    ON webhook_delivery_attempts (available_at, id)
    WHERE status = 'pending';

-- History excludes unsent reservations and pages on immutable (sent_at,id).
CREATE INDEX webhook_delivery_attempts_endpoint_sent_idx
    ON webhook_delivery_attempts (endpoint_id, sent_at DESC, id DESC)
    WHERE sent_at IS NOT NULL;

-- Match internal/id.Derive(id.WebhookDelivery, key) for the one case where a
-- legacy mutable row has both retained evidence and a separately scheduled
-- retry. Rows never attempted can reuse their unexposed parent whd directly.
CREATE FUNCTION bex_webhook_attempt_id(event_key text)
RETURNS text
LANGUAGE plpgsql
IMMUTABLE
STRICT
PARALLEL SAFE
AS $$
DECLARE
    digest_bits bit(256);
    alphabet constant text := '0123456789abcdefghijklmnopqrstuv';
    derived text := 'whd-';
    digit integer;
BEGIN
    digest_bits := ('x' || encode(sha256(convert_to(event_key, 'UTF8')), 'hex'))::bit(256);
    FOR digit IN 0..19 LOOP
        derived := derived || substr(
            alphabet,
            substring(digest_bits FROM digit * 5 + 1 FOR 5)::integer + 1,
            1
        );
    END LOOP;
    RETURN derived;
END;
$$;

-- Rolling-version bridge. An old replica inserts only the mutable parent; make
-- that insert indistinguishable from the new Enqueue path by reserving its
-- initial child in the same transaction. New replicas use this same trigger,
-- so a losing parent ON CONFLICT performs no child work.
CREATE FUNCTION bex_webhook_attempt_from_legacy_insert()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    INSERT INTO webhook_delivery_attempts (
        id, notification_id, endpoint_id, attempt_number, status, origin,
        available_at, created_at
    ) VALUES (
        NEW.id, NEW.id, NEW.endpoint_id, 1, 'pending', 'automatic',
        NEW.next_attempt_at, NEW.created_at
    ) ON CONFLICT DO NOTHING;
    RETURN NEW;
END;
$$;

CREATE TRIGGER webhook_deliveries_attempt_insert_bridge
AFTER INSERT ON webhook_deliveries
FOR EACH ROW EXECUTE FUNCTION bex_webhook_attempt_from_legacy_insert();

-- After the ledger exists, only child reservations may be claimed. An old
-- replica's claim updates next_attempt_at and no evidence/state column; cancel
-- that update so its CTE returns no row and it cannot double-send beside a new
-- worker. A pre-migration claim is handled by the completion bridge below.
CREATE FUNCTION bex_block_legacy_webhook_claim()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    IF NEW.next_attempt_at IS DISTINCT FROM OLD.next_attempt_at
       AND NEW.attempt_count IS NOT DISTINCT FROM OLD.attempt_count
       AND NEW.last_status IS NOT DISTINCT FROM OLD.last_status
       AND NEW.last_error IS NOT DISTINCT FROM OLD.last_error
       AND NEW.response_body IS NOT DISTINCT FROM OLD.response_body
       AND NEW.sent_at IS NOT DISTINCT FROM OLD.sent_at
       AND NEW.last_attempted_at IS NOT DISTINCT FROM OLD.last_attempted_at
       AND NEW.delivered_at IS NOT DISTINCT FROM OLD.delivered_at
       AND NEW.failed_at IS NOT DISTINCT FROM OLD.failed_at THEN
        RETURN NULL;
    END IF;
    RETURN NEW;
END;
$$;

CREATE TRIGGER webhook_deliveries_block_legacy_claim
BEFORE UPDATE ON webhook_deliveries
FOR EACH ROW EXECUTE FUNCTION bex_block_legacy_webhook_claim();

-- An old replica can have claimed immediately before this migration installed
-- the blocker. When it later writes its aggregate outcome, translate that one
-- exchange into terminal child evidence and its next reservation. New Complete
-- terminalizes the child before updating the parent, so the existence check
-- makes this bridge a no-op on the new path. Exhaustion also disables the
-- endpoint in this transaction, closing the old worker's historical gap.
CREATE FUNCTION bex_webhook_attempt_from_legacy_completion()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
    completed_number integer;
    completed_at timestamptz;
BEGIN
    IF NEW.attempt_count > OLD.attempt_count
       AND NOT EXISTS (
           SELECT 1 FROM webhook_delivery_attempts
           WHERE notification_id = NEW.id
             AND origin = 'automatic'
             AND status <> 'pending'
             AND attempt_number = NEW.attempt_count
       ) THEN
        completed_at := COALESCE(
            NEW.last_attempted_at, NEW.delivered_at, NEW.failed_at,
            clock_timestamp()
        );
        UPDATE webhook_delivery_attempts
        SET status = CASE WHEN NEW.delivered_at IS NOT NULL THEN 'delivered' ELSE 'failed' END,
            sent_at = completed_at,
            status_code = NEW.last_status,
            transport_error = CASE
                WHEN NEW.delivered_at IS NOT NULL THEN ''
                ELSE left(NEW.last_error, 512)
            END,
            response_body = left(NEW.response_body, 1024),
            lease_until = NULL
        WHERE id = (
            SELECT id FROM webhook_delivery_attempts
            WHERE notification_id = NEW.id
              AND origin = 'automatic'
              AND status = 'pending'
            ORDER BY attempt_number
            LIMIT 1
        )
        RETURNING attempt_number INTO completed_number;

        IF completed_number IS NOT NULL
           AND NEW.delivered_at IS NULL
           AND NEW.failed_at IS NULL THEN
            INSERT INTO webhook_delivery_attempts (
                id, notification_id, endpoint_id, attempt_number, status,
                origin, available_at, created_at
            ) VALUES (
                bex_webhook_attempt_id(
                    'legacy:' || NEW.id || ':' || (completed_number + 1)::text
                ),
                NEW.id, NEW.endpoint_id, completed_number + 1, 'pending',
                'automatic', NEW.next_attempt_at, completed_at
            ) ON CONFLICT DO NOTHING;
        END IF;
    END IF;

    IF NEW.failed_at IS NOT NULL AND OLD.failed_at IS NULL THEN
        UPDATE webhook_endpoints
        SET enabled = false,
            disabled_reason = 'disabled automatically after repeated delivery failures',
            updated_at = clock_timestamp()
        WHERE id = NEW.endpoint_id;
    END IF;
    RETURN NEW;
END;
$$;

CREATE TRIGGER webhook_deliveries_attempt_completion_bridge
AFTER UPDATE ON webhook_deliveries
FOR EACH ROW EXECUTE FUNCTION bex_webhook_attempt_from_legacy_completion();

-- An old replica's retention query knows only the parent's terminal columns.
-- A manual replay of an old terminal notification deliberately leaves those
-- aggregate columns intact until its exchange finishes, so the legacy sweep
-- would otherwise cascade-delete the pending reservation before it sends.
-- Direct parent deletes are therefore parked while any child is pending. The
-- new abandoned-reservation sweep opts in explicitly after checking the child
-- reservation's own age; endpoint FK cascades are nested trigger work and must
-- remain able to remove the whole endpoint history.
CREATE FUNCTION bex_guard_webhook_delivery_delete()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    IF pg_trigger_depth() = 1
       AND COALESCE(current_setting('bex.webhook_pending_delete', true), '') <> 'on'
       AND EXISTS (
           SELECT 1 FROM webhook_delivery_attempts
           WHERE notification_id = OLD.id AND status = 'pending'
       ) THEN
        RETURN NULL;
    END IF;
    RETURN OLD;
END;
$$;

CREATE TRIGGER webhook_deliveries_guard_pending_delete
BEFORE DELETE ON webhook_deliveries
FOR EACH ROW EXECUTE FUNCTION bex_guard_webhook_delivery_delete();

-- The old row retained only its latest exchange, so that is the one evidence
-- attempt that can be backfilled truthfully. Its public id remains the old
-- delivery id, preserving existing REST cursors and references.
INSERT INTO webhook_delivery_attempts (
    id, notification_id, endpoint_id, attempt_number, status, origin,
    available_at, sent_at, status_code, transport_error, response_body, created_at
)
SELECT d.id, d.id, d.endpoint_id, GREATEST(d.attempt_count, 1),
       CASE WHEN d.delivered_at IS NOT NULL THEN 'delivered' ELSE 'failed' END,
       'automatic', d.next_attempt_at,
       COALESCE(d.last_attempted_at, d.sent_at, d.delivered_at, d.failed_at, d.created_at),
       d.last_status, left(d.last_error, 512), left(d.response_body, 1024), d.created_at
FROM webhook_deliveries d
WHERE d.attempt_count > 0;

-- Every open notification has exactly one pending automatic reservation. A
-- never-attempted row reuses the parent id; a retry needs a second deterministic
-- whd because the parent id above preserves its latest evidence attempt.
INSERT INTO webhook_delivery_attempts (
    id, notification_id, endpoint_id, attempt_number, status, origin,
    available_at, created_at
)
SELECT CASE
           WHEN d.attempt_count = 0 THEN d.id
           ELSE bex_webhook_attempt_id('backfill:' || d.id || ':' || (d.attempt_count + 1)::text)
       END,
       d.id, d.endpoint_id, d.attempt_count + 1, 'pending', 'automatic',
       d.next_attempt_at, d.created_at
FROM webhook_deliveries d
WHERE d.delivered_at IS NULL AND d.failed_at IS NULL;

-- A row claimed by an old replica just before the migration has no child lease.
-- Conservatively block Resend/new claims for the full production two-minute
-- claim window. available_at independently gates genuine future retries; using
-- LEAST here would leave an already-due row's lease in the past and let a new
-- worker double-send beside an old replica that claimed just before migration.
UPDATE webhook_delivery_attempts a
SET lease_until = clock_timestamp() + interval '2 minutes'
WHERE a.status = 'pending'
  AND a.origin = 'automatic'
  AND a.lease_until IS NULL;
