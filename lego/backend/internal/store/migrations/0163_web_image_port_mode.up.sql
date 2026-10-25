-- w4/m243: web services take image-derived ports under their own port modes.
ALTER TABLE apps DROP CONSTRAINT IF EXISTS apps_port_mode_check;
ALTER TABLE apps
  ADD CONSTRAINT apps_port_mode_check
    CHECK (port_mode IN ('', 'configured-v1', 'image-v1', 'image-configured-v1', 'web-image-v1', 'web-image-configured-v1'));
