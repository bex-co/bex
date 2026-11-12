-- Reverse w4/m337's rename and custom-domain details. Renames and domains
-- themselves are unaffected; their events stop naming them.
ALTER TABLE audit_events
  DROP COLUMN IF EXISTS domain,
  DROP COLUMN IF EXISTS display_name_to,
  DROP COLUMN IF EXISTS display_name_from;
