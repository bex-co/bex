-- w2/m254 t003: Render scopes a build source to a project, if any. A carrier's
-- own project_id stays NULL (so no project or environment listing surfaces it,
-- apps_build_source_shape_check); its scope lives here.
ALTER TABLE apps
    ADD COLUMN scope_project_id text REFERENCES projects (id) ON DELETE SET NULL,
    ADD CONSTRAINT apps_scope_project_check CHECK (scope_project_id IS NULL OR type = 'build_source');
