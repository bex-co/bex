-- w9/m620: can_view_billing, Render's "View billing details" (Admin,
-- Developer and Billing), gates the workspace's usage, cost, invoices and
-- billing readiness. The relation CHECK must list core.RelCanRelations()
-- exactly (TestAuditRelationCheckMatchesRelCan); recordAudit swallows sink
-- errors, so a relation missing here would drop a refused read's row.
ALTER TABLE audit_events DROP CONSTRAINT audit_events_relation_check;
ALTER TABLE audit_events
    ADD CONSTRAINT audit_events_relation_check
        CHECK (relation IS NULL OR relation IN (
            'can_view',
            'can_view_logs',
            'can_operate',
            'can_create',
            'can_view_sensitive',
            'can_manage_keys',
            'can_manage_ssh_keys',
            'can_manage',
            'can_manage_billing',
            'can_join',
            'can_join_as_admin',
            'can_view_billing'
        ));
