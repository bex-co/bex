-- w4/m212: an endpoint's source-event eligibility is a set of intervals, one
-- per stretch it was enabled. Webhook fan-out admits an event for an endpoint
-- only when the event's source time falls inside one of them, so a disabled
-- stretch never becomes notifications after re-enable — independent of how far
-- the shared feed cursor had read — while enabled-period events not yet
-- dispatched when the endpoint was disabled stay eligible.
--
-- [enabled_at, disabled_at): enabled_at inclusive, disabled_at exclusive; an
-- open interval (disabled_at NULL) is the current enabled stretch.
CREATE TABLE webhook_endpoint_eligibility (
    endpoint_id text NOT NULL REFERENCES webhook_endpoints (id) ON DELETE CASCADE,
    enabled_at  timestamptz NOT NULL,
    disabled_at timestamptz,
    PRIMARY KEY (endpoint_id, enabled_at),
    CHECK (disabled_at IS NULL OR disabled_at >= enabled_at)
);

CREATE UNIQUE INDEX webhook_endpoint_eligibility_open_idx
    ON webhook_endpoint_eligibility (endpoint_id) WHERE disabled_at IS NULL;

CREATE INDEX webhook_endpoint_eligibility_disabled_at_idx
    ON webhook_endpoint_eligibility (disabled_at);

-- Every writer of webhook_endpoints.enabled — create, the manual toggle, the
-- full update and the automatic disable after repeated failures — goes through
-- this trigger, in the writer's own transaction. A save that leaves enabled
-- unchanged touches no interval, and a failed write rolls its interval change
-- back with it.
CREATE FUNCTION bex_track_webhook_endpoint_eligibility()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    IF TG_OP = 'INSERT' THEN
        IF NEW.enabled THEN
            INSERT INTO webhook_endpoint_eligibility (endpoint_id, enabled_at)
            VALUES (NEW.id, NEW.created_at);
        END IF;
    ELSIF NEW.enabled AND NOT OLD.enabled THEN
        INSERT INTO webhook_endpoint_eligibility (endpoint_id, enabled_at)
        VALUES (NEW.id, clock_timestamp());
    ELSIF OLD.enabled AND NOT NEW.enabled THEN
        UPDATE webhook_endpoint_eligibility
           SET disabled_at = greatest(clock_timestamp(), enabled_at)
         WHERE endpoint_id = NEW.id AND disabled_at IS NULL;
    END IF;
    RETURN NULL;
END;
$$;

CREATE TRIGGER webhook_endpoint_eligibility_track
    AFTER INSERT OR UPDATE OF enabled ON webhook_endpoints
    FOR EACH ROW EXECUTE FUNCTION bex_track_webhook_endpoint_eligibility();

-- Existing endpoints. An enabled one has been eligible since creation, which
-- is exactly the CreatedAt guard it had before. A disabled one's disable time
-- was never recorded; updated_at is no earlier than it (a disable writes
-- updated_at, later edits only move it forward), so [created_at, updated_at)
-- never drops an enabled-period event, at worst it admits part of a disabled
-- stretch that predates this migration.
INSERT INTO webhook_endpoint_eligibility (endpoint_id, enabled_at, disabled_at)
SELECT id, created_at, CASE WHEN enabled THEN NULL ELSE greatest(updated_at, created_at) END
  FROM webhook_endpoints;
