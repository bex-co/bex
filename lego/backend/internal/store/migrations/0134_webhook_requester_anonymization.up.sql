-- Account deletion may anonymize requester attribution without rewriting the
-- immutable delivery evidence or its replay reservation. Comparing the whole
-- row also keeps future columns outside this exception.
CREATE OR REPLACE FUNCTION bex_guard_webhook_attempt_update()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    IF NEW.requested_by IS DISTINCT FROM OLD.requested_by
       AND to_jsonb(NEW) - 'requested_by' = to_jsonb(OLD) - 'requested_by'
       AND EXISTS (
           SELECT 1 FROM account_deletions
           WHERE subject = OLD.requested_by AND deleted_marker = NEW.requested_by
       ) THEN
        RETURN NEW;
    END IF;
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

CREATE INDEX webhook_delivery_attempts_requested_by_idx
    ON webhook_delivery_attempts (requested_by);

-- Account offboarding must not scan every workspace's deployment history.
CREATE INDEX deploys_triggered_by_idx ON deploys (triggered_by);
