-- R25-10 (w21/m394 t013): an unverified custom-domain claim is reclaimed 48 hours
-- after it was made, but only the row's own created_at counted, so deleting the
-- claim and adding it again started the 48 hours over, and a squatter could hold
-- a hostname for ever without pointing DNS at it. This records when a pending
-- claim was first made, keyed by host, so the next pending claim inside a day
-- carries it forward. It is done in triggers so every way a claim is added or
-- removed (explicit add, Blueprint sync, service create, service delete through
-- the domains.app_id cascade) honours it. Verified claims are never reclaimed
-- and are not recorded.
CREATE TABLE domain_claim_history (
    host        text        PRIMARY KEY,
    since       timestamptz NOT NULL,
    recorded_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX domain_claim_history_recorded_at_idx ON domain_claim_history (recorded_at);

-- A pending claim deleted inside its 48 hours records its first-claim time. One
-- already past 48 hours is reclaimable, so it records nothing: the next claim is
-- a new one.
CREATE FUNCTION bex_record_domain_claim_history()
RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF OLD.claim_state = 'pending' AND OLD.created_at > now() - interval '48 hours' THEN
        DELETE FROM domain_claim_history WHERE recorded_at < now() - interval '48 hours';
        INSERT INTO domain_claim_history (host, since, recorded_at)
        VALUES (OLD.host, OLD.created_at, now())
        ON CONFLICT (host) DO UPDATE SET since = EXCLUDED.since, recorded_at = EXCLUDED.recorded_at;
    END IF;
    RETURN OLD;
END;
$$;

CREATE TRIGGER domains_record_claim_history
BEFORE DELETE ON domains
FOR EACH ROW EXECUTE FUNCTION bex_record_domain_claim_history();

-- A new pending claim made within 24 hours of a recorded delete, while the first
-- claim is still inside its 48 hours, takes that first claim's time as its own
-- created_at. A longer gap starts afresh, so a legitimate owner who comes back
-- later gets a full window.
CREATE FUNCTION bex_inherit_domain_claim_time()
RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE
    first_claim timestamptz;
BEGIN
    IF NEW.claim_state = 'pending' THEN
        SELECT since INTO first_claim FROM domain_claim_history
        WHERE host = NEW.host
          AND recorded_at > now() - interval '24 hours'
          AND since > now() - interval '48 hours';
        IF FOUND AND first_claim < NEW.created_at THEN
            NEW.created_at := first_claim;
        END IF;
    END IF;
    RETURN NEW;
END;
$$;

CREATE TRIGGER domains_inherit_claim_time
BEFORE INSERT ON domains
FOR EACH ROW EXECUTE FUNCTION bex_inherit_domain_claim_time();

-- Once a row exists for the host the record is spent. AFTER INSERT does not run
-- for a row ON CONFLICT skipped, so a refused claim does not consume it.
CREATE FUNCTION bex_clear_domain_claim_history()
RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    DELETE FROM domain_claim_history WHERE host = NEW.host;
    RETURN NULL;
END;
$$;

CREATE TRIGGER domains_clear_claim_history
AFTER INSERT ON domains
FOR EACH ROW EXECUTE FUNCTION bex_clear_domain_claim_history();
