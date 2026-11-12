-- w4/563: an env-group link or unlink names its group in the service's events.
--
-- The link and unlink rows recorded only the service, so the feed could not
-- say which group arrived or left. One typed column per value, as the other
-- audit details (docs/ADR006-bex-api.md's audit shape): the group's evg- id
-- and its name at the time, which stays readable after the group is deleted.
-- NULL for every other verb and for rows recorded before this.
ALTER TABLE audit_events
  ADD COLUMN IF NOT EXISTS env_group_id text,
  ADD COLUMN IF NOT EXISTS env_group_name text;
