-- Persist creation-time policy independently of later platform defaults.
-- Existing rows remain strict and keep their configured private port.
ALTER TABLE apps
  ADD COLUMN container_policy text NOT NULL DEFAULT ''
    CHECK (container_policy IN ('', 'strict-v1', 'image-v1')),
  ADD COLUMN port_mode text NOT NULL DEFAULT ''
    CHECK (port_mode IN ('', 'configured-v1', 'image-v1', 'image-configured-v1'));
