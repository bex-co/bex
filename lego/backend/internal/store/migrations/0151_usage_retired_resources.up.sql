-- Usage meters enumerate live resources, so a resource deleted before its hour
-- was rolled up went unmetered for that hour (w4/m173; ADR023). This keeps what
-- the meters need to bill it through its deletion hour.
--
-- No foreign key to tenants: the trigger below writes while a workspace
-- deletion cascades through its Apps, and must not fail that delete. The meter
-- drops rows whose workspace is gone.
CREATE TABLE usage_retired_resources (
    resource_kind text        NOT NULL,
    resource_id   text        NOT NULL,
    workspace_id  text        NOT NULL,
    -- Apps: the service name their pod identity derives from. Datastores: the
    -- CR name, with its namespace, display name and public-proxy flag.
    name          text        NOT NULL,
    namespace     text        NOT NULL DEFAULT '',
    display_name  text        NOT NULL DEFAULT '',
    public        boolean     NOT NULL DEFAULT false,
    tier          text        NOT NULL DEFAULT '',
    created_at    timestamptz NOT NULL,
    retired_at    timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (resource_kind, resource_id)
);

-- Every App deletion path — the delete verb, a workspace cascade — removes the
-- row, so the row's removal is what retires it. (RollbackAppCreation discards
-- the record again: a create that never completed owes nothing.)
CREATE FUNCTION bex_retire_app_usage()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    INSERT INTO usage_retired_resources (resource_kind, resource_id, workspace_id, name, tier, created_at)
    VALUES ('service', OLD.id, OLD.tenant_id, OLD.name, OLD.tier, OLD.created_at)
    ON CONFLICT (resource_kind, resource_id) DO UPDATE SET retired_at = EXCLUDED.retired_at;
    RETURN NULL;
END;
$$;

CREATE TRIGGER apps_retire_usage
AFTER DELETE ON apps
FOR EACH ROW EXECUTE FUNCTION bex_retire_app_usage();
