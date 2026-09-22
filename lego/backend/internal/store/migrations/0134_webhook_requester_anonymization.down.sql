DROP INDEX deploys_triggered_by_idx;

DROP INDEX webhook_delivery_attempts_requested_by_idx;

CREATE OR REPLACE FUNCTION bex_guard_webhook_attempt_update()
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
