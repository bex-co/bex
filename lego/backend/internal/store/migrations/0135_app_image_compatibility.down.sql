-- Refuse a downgrade that would silently strip running services' policy.
DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM apps WHERE container_policy = 'image-v1' OR port_mode IN ('image-v1', 'image-configured-v1')) THEN
    RAISE EXCEPTION 'cannot remove image compatibility while compatible services exist';
  END IF;
END $$;
ALTER TABLE apps DROP COLUMN port_mode, DROP COLUMN container_policy;
