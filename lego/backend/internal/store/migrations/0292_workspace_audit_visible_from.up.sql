-- w9/098, ADR006 § Audit log: the instant a workspace's audit history starts.
-- Render: "Retention starts only when you upgrade to Pro or higher. Earlier
-- data is unavailable." bex keeps recording audit events on Hobby, so a plan
-- change from a plan without audit logs to one with them stamps the change's
-- moment here, and every audit read starts no earlier than it. A downgrade
-- keeps it, and a later upgrade moves it forward. NULL (every existing
-- workspace, and one created on Pro or higher) means no floor. Written only
-- by store.UpdateTenantPlan (core.AuditVisibleFromAfterPlanChange).
ALTER TABLE tenants ADD COLUMN audit_visible_from timestamptz;
