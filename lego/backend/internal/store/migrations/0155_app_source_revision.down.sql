DROP TRIGGER IF EXISTS apps_source_revision ON apps;
DROP FUNCTION IF EXISTS bump_app_source_revision();
ALTER TABLE apps DROP COLUMN source_revision;
