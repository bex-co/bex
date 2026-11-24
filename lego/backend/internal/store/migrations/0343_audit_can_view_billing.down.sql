-- Reverses 0343_audit_can_view_billing: no row may carry can_view_billing
-- once the CHECK is narrowed again, so those relations are cleared first.
UPDATE audit_events SET relation = NULL WHERE relation = 'can_view_billing';
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
            'can_join_as_admin'
        ));
