-- w4/m337: a rename names both names, and a custom-domain event its hostname.
--
-- display_name_changed and custom_domain_added/_removed/_verified rows
-- recorded only the service, so the feed could not say what the service was
-- called before or which domain arrived, left or was verified. One typed
-- column per value, as the other audit details: the label on each side of a
-- rename (the immutable name where none was set) and the domain's hostname.
-- Both are non-secret labels every reader of the service already sees. NULL
-- for every other verb and for rows recorded before this.
ALTER TABLE audit_events
  ADD COLUMN IF NOT EXISTS display_name_from text,
  ADD COLUMN IF NOT EXISTS display_name_to text,
  ADD COLUMN IF NOT EXISTS domain text;
