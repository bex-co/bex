-- w5/m97: which surface created the resource (dashboard / CLI / MCP / direct
-- API / Blueprint).
--
-- ActorType already records human-vs-machine credential, which is a different
-- question: an agent holding a human's OAuth grant still reads as human, and a
-- person using the CLI is indistinguishable from one using the dashboard. This
-- column is the axis ADR008's agent-native thesis is actually measured on.
--
-- 'unknown' is the default and an honest bucket, not a gap: legacy backfill
-- predates collection, and any effect recorded outside a classified request
-- lands here rather than being guessed at.
-- Added in three steps on purpose. A CHECK declared inline with ADD COLUMN is
-- validated by a full scan under ACCESS EXCLUSIVE; NOT VALID still rejects every
-- new row, and VALIDATE then takes only SHARE UPDATE EXCLUSIVE. The table is
-- small today (one row per created resource, since 0114), so this costs nothing
-- now and stays safe when it is not small.
ALTER TABLE product_activity_events
    ADD COLUMN surface text NOT NULL DEFAULT 'unknown';

ALTER TABLE product_activity_events
    ADD CONSTRAINT product_activity_events_surface_check
    CHECK (surface IN ('dashboard', 'cli', 'mcp', 'api', 'blueprint', 'unknown')) NOT VALID;

ALTER TABLE product_activity_events
    VALIDATE CONSTRAINT product_activity_events_surface_check;

-- When surface attribution began, following the same watermark pattern 0114 and
-- 0115 use for their own collections. Every row above predates it and defaulted
-- to 'unknown', so a range reaching back before this instant cannot be scored:
-- the agent-share panel reads null there rather than dividing by a denominator
-- stuffed with creations that were never classifiable.
ALTER TABLE product_analytics_collection
    ADD COLUMN surface_started_at timestamptz NOT NULL DEFAULT clock_timestamp();
