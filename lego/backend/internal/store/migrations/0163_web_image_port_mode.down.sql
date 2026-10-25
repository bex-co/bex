-- Refuse to narrow the port-mode check while web image-port rows exist.
DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM apps WHERE port_mode IN ('web-image-v1', 'web-image-configured-v1')) THEN
    RAISE EXCEPTION 'web image port modes in use';
  END IF;
END $$;
ALTER TABLE apps DROP CONSTRAINT IF EXISTS apps_port_mode_check;
ALTER TABLE apps
  ADD CONSTRAINT apps_port_mode_check
    CHECK (port_mode IN ('', 'configured-v1', 'image-v1', 'image-configured-v1'));
