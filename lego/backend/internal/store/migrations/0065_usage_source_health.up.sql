-- w11/m8: durable evidence for honest current-month usage coverage.
--
-- Existing usage rows are deliberately not backfilled.  A total predating this
-- migration is consumption evidence, but cannot prove that every source was
-- present and healthy for the hour.

CREATE TABLE usage_source_streams (
    workspace_id     text        NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    resource_kind    text        NOT NULL,
    service_id       text        NOT NULL,
    kind             text        NOT NULL,
    source           text        NOT NULL CHECK (source IN ('instance', 'build', 'storage', 'http', 'websocket', 'direct', 'postgres', 'key_value')),
    expected_from    timestamptz NOT NULL,
    expected_through timestamptz,
    PRIMARY KEY (resource_kind, service_id, kind, source)
);

CREATE INDEX usage_source_streams_workspace_idx
    ON usage_source_streams (workspace_id);

CREATE TABLE usage_source_health (
    workspace_id  text        NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    resource_kind text        NOT NULL,
    service_id    text        NOT NULL,
    kind          text        NOT NULL,
    source        text        NOT NULL CHECK (source IN ('instance', 'build', 'storage', 'http', 'websocket', 'direct', 'postgres', 'key_value')),
    window_start  timestamptz NOT NULL,
    state         text        NOT NULL CHECK (state IN ('healthy', 'degraded', 'unavailable')),
    PRIMARY KEY (resource_kind, service_id, kind, source, window_start),
    FOREIGN KEY (resource_kind, service_id, kind, source)
        REFERENCES usage_source_streams (resource_kind, service_id, kind, source)
        ON DELETE CASCADE
);

CREATE INDEX usage_source_health_workspace_window_idx
    ON usage_source_health (workspace_id, window_start);
