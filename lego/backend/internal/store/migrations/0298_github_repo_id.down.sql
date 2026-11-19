ALTER TABLE audit_events
  DROP COLUMN IF EXISTS repo_to,
  DROP COLUMN IF EXISTS repo_from;
ALTER TABLE blueprints DROP COLUMN IF EXISTS github_repo_id;
ALTER TABLE apps DROP COLUMN IF EXISTS github_repo_id;
