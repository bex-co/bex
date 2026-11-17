-- Reverses 0293_organizations: no org rows may carry the two org relations
-- once the CHECK is narrowed again, so they are cleared first.
UPDATE audit_events SET relation = NULL WHERE relation IN ('can_join', 'can_join_as_admin');
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
            'can_manage_billing'
        ));

DROP INDEX IF EXISTS tenants_organization_idx;
ALTER TABLE tenants DROP COLUMN privacy, DROP COLUMN organization_id;
DROP TABLE organization_invites;
DROP TABLE organization_members;
DROP TABLE organizations;
