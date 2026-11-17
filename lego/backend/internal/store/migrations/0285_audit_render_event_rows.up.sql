-- w9/m449: the workspace audit log names Render's event for every action
-- Render's audit log names.
--
-- env_var_keys  the variable names a CreateEnvVarsEvent, UpdateEnvVarsEvent or
--               DeleteEnvVarsEvent row created, changed or deleted. Names only:
--               a value never enters an audit row. NULL on every other row.
-- feed_only     the row is a service activity feed's record of a change the
--               workspace audit log reports under Render's own rows instead
--               (an env var save's per-kind rows, an auto-deploy trigger
--               change, a service linked to a build source). The audit log
--               skips it; the feed and webhooks read it as before.
ALTER TABLE audit_events
  ADD COLUMN IF NOT EXISTS env_var_keys text[],
  ADD COLUMN IF NOT EXISTS feed_only boolean NOT NULL DEFAULT false;
