-- A service can disappear before its last disk hour is metered. Keep the
-- closed disk and size periods, keyed by the immutable service id, so the
-- existing meter can still integrate that hour after any apps DELETE path.
ALTER TABLE service_disks DROP CONSTRAINT service_disks_app_id_fkey;

CREATE FUNCTION bex_require_live_disk_parent()
RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF NEW.deleted_at IS NULL THEN
        -- Workspace deletion locks tenant -> App; attachments must use the
        -- same order before the remaining tenant FK checks the inserted row.
        PERFORM 1 FROM tenants WHERE id = NEW.tenant_id FOR KEY SHARE;
        IF NOT FOUND THEN
            RAISE EXCEPTION 'live disk requires its owning workspace'
                USING ERRCODE = '23503', CONSTRAINT = 'service_disks_tenant_id_fkey';
        END IF;
        -- Replace the removed FK's existence/locking guarantee for live disks,
        -- and bind the disk to its parent's workspace. Hold the parent through
        -- commit so an attachment racing deletion cannot become an orphan.
        PERFORM 1 FROM apps
        WHERE id = NEW.app_id AND tenant_id = NEW.tenant_id
        FOR KEY SHARE;
        IF NOT FOUND THEN
            RAISE EXCEPTION 'live disk requires its owning service and workspace'
                USING ERRCODE = '23503', CONSTRAINT = 'service_disks_live_app';
        END IF;
    END IF;
    RETURN NEW;
END;
$$;

CREATE TRIGGER service_disks_require_live_parent
BEFORE INSERT OR UPDATE OF app_id, tenant_id, deleted_at ON service_disks
FOR EACH ROW EXECUTE FUNCTION bex_require_live_disk_parent();

CREATE FUNCTION bex_close_disks_on_app_delete()
RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE
    stopped_at timestamptz;
BEGIN
    -- A grow may already hold the disk lock. Take every disk lock before
    -- reading the clock: a transaction begun before that grow must not close
    -- its newly committed period at an earlier transaction timestamp.
    PERFORM 1 FROM service_disks
    WHERE app_id = OLD.id AND deleted_at IS NULL
    ORDER BY id FOR UPDATE;
    stopped_at := clock_timestamp();
    WITH retired AS (
        UPDATE service_disks
        SET deleted_at = stopped_at, updated_at = stopped_at
        WHERE app_id = OLD.id AND deleted_at IS NULL
        RETURNING id
    )
    UPDATE service_disk_sizes
    SET to_ts = stopped_at
    WHERE disk_id IN (SELECT id FROM retired)
      AND to_ts IS NULL;
    RETURN OLD;
END;
$$;

CREATE TRIGGER apps_close_disk_billing
BEFORE DELETE ON apps
FOR EACH ROW EXECUTE FUNCTION bex_close_disks_on_app_delete();
