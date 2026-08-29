-- w6/m134: project/environment reassignment events. A successful
-- projects.SetServices / environments.SetServices left durable workspace audit
-- rows but no service-scoped event, so a move was invisible in the service's
-- own history. The service layer now records one fixed-verb row
-- (projects.MoveService / environments.MoveService) per service whose
-- placement actually changed, carrying the before/after public prj-/env- ids
-- in typed columns — the same structural discipline as plan_from/plan_to.
-- NULL means "no placement on that side" (assign/unassign), never a value.
ALTER TABLE audit_events ADD COLUMN project_from text;
ALTER TABLE audit_events ADD COLUMN project_to text;
ALTER TABLE audit_events ADD COLUMN environment_from text;
ALTER TABLE audit_events ADD COLUMN environment_to text;
