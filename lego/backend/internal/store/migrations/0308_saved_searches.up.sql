-- 0308: project observability views are saved searches (w3/m505, ADR032).
-- Render keeps a project's Application Logs, Request Logs and Service Metrics
-- views as saved searches (svs-…), created for the project with an empty
-- filter document; bex does the same, and a view is deleted with its project.

-- Match internal/id.Derive(id.SavedSearch, project_id, search_type) exactly:
-- the first 100 bits of sha256(project_id || 0x00 || search_type) in Go's
-- lowercase base32hex alphabet (the 0083 bex_service_event_id recipe). A
-- default view's id is a function of its project and type, so the backfill
-- below, the trigger and the Go side all name it the same.
CREATE FUNCTION bex_saved_search_id(project_id text, search_type text)
RETURNS text
LANGUAGE plpgsql
IMMUTABLE
STRICT
PARALLEL SAFE
AS $$
DECLARE
    digest_bits bit(256);
    alphabet constant text := '0123456789abcdefghijklmnopqrstuv';
    derived text := 'svs-';
    digit integer;
BEGIN
    digest_bits := ('x' || encode(sha256(
        convert_to(project_id, 'UTF8') || '\x00'::bytea || convert_to(search_type, 'UTF8')
    ), 'hex'))::bit(256);
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

CREATE TABLE saved_searches (
    id          text        PRIMARY KEY,
    project_id  text        NOT NULL REFERENCES projects (id) ON DELETE CASCADE,
    name        text        NOT NULL,
    type        text        NOT NULL,
    filters     jsonb       NOT NULL DEFAULT '{"filters": [], "version": 1}',
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT saved_searches_id_check CHECK (id ~ '^svs-[0-9a-v]{20}$'),
    CONSTRAINT saved_searches_name_check CHECK (name <> ''),
    -- WORKFLOW_RUNS is Render Workflows, a bex non-goal (ADR018).
    CONSTRAINT saved_searches_type_check CHECK (type IN ('APPLICATION_LOGS', 'REQUEST_LOGS', 'METRICS')),
    -- Render's duplicate_saved_search_name.
    CONSTRAINT saved_searches_project_name_key UNIQUE (project_id, name)
);

-- Render's three defaults, in the transaction that creates the project, so
-- every create path (REST, GraphQL, MCP, Blueprints) gets them.
CREATE FUNCTION bex_default_saved_searches()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    INSERT INTO saved_searches (id, project_id, name, type, created_at, updated_at)
    SELECT bex_saved_search_id(NEW.id, d.type), NEW.id, d.name, d.type, NEW.created_at, NEW.created_at
    FROM (VALUES
        ('Application Logs', 'APPLICATION_LOGS'),
        ('Request Logs', 'REQUEST_LOGS'),
        ('Service Metrics', 'METRICS')
    ) AS d (name, type);
    RETURN NEW;
END;
$$;

CREATE TRIGGER projects_default_saved_searches
    AFTER INSERT ON projects
    FOR EACH ROW EXECUTE FUNCTION bex_default_saved_searches();

-- Every existing project gets the same three.
INSERT INTO saved_searches (id, project_id, name, type, created_at, updated_at)
SELECT bex_saved_search_id(p.id, d.type), p.id, d.name, d.type, p.created_at, p.created_at
FROM projects p
CROSS JOIN (VALUES
    ('Application Logs', 'APPLICATION_LOGS'),
    ('Request Logs', 'REQUEST_LOGS'),
    ('Service Metrics', 'METRICS')
) AS d (name, type)
ON CONFLICT DO NOTHING;
