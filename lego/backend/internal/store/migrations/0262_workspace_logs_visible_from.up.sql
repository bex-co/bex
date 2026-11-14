-- w9/m389, ADR010 § Durability & retention: the log floor a workspace's last
-- plan change kept. Every log read is floored at the workspace plan's window
-- (core.LogRetention), and Render keeps logs that
-- aged out hidden "even if you upgrade your plan to extend the period". So an
-- upgrade stores the old plan's floor at that moment here, and reads take the
-- later of it and the new window's floor until the new window overtakes it.
-- NULL (every existing workspace) means the plan's window alone applies.
-- Written only by store.UpdateTenantPlan (core.LogsVisibleFromAfterPlanChange).
ALTER TABLE tenants ADD COLUMN logs_visible_from timestamptz;
