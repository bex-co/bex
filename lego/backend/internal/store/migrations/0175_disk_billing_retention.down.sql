-- Restoring the old cascade is possible only before historical disks outlive
-- their services. Never discard financial history to make a rollback fit.
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM service_disks d
        WHERE NOT EXISTS (SELECT 1 FROM apps a WHERE a.id = d.app_id)
    ) THEN
        RAISE EXCEPTION 'cannot restore service_disks_app_id_fkey: retained disk billing history has no live service'
            USING ERRCODE = '55000';
    END IF;
END;
$$;

ALTER TABLE service_disks ADD CONSTRAINT service_disks_app_id_fkey
    FOREIGN KEY (app_id) REFERENCES apps(id) ON DELETE CASCADE;
DROP TRIGGER apps_close_disk_billing ON apps;
DROP FUNCTION bex_close_disks_on_app_delete();
DROP TRIGGER service_disks_require_live_parent ON service_disks;
DROP FUNCTION bex_require_live_disk_parent();
