-- evm- is the public alias of an env- durable identity, never a second row.
-- Validate existing rows too: an imported collision must stop the upgrade
-- rather than let the alias address a different workspace's environment.
ALTER TABLE environments ADD CONSTRAINT environments_storage_id_namespace
  CHECK (id NOT LIKE 'evm-%');
