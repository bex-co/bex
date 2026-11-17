-- w7/m464, docs/ADR115-organizations.md: Render's organizations. A Scale or
-- Enterprise workspace is the primary of an org that holds unlimited
-- workspaces under the primary's one plan ("An org can contain unlimited
-- workspaces at no additional cost", render.com/docs/organizations, read
-- 2026-11-17).
--
--   organizations         the org; its plan is its primary's tenants.plan,
--                         which every write keeps equal on its secondaries.
--                         Deleting the primary deletes the org.
--   organization_members  a person's one org role: owner | member | guest.
--   organization_invites  an Owner's invite to the org by email, as owner or
--                         member, redeemed by its emailed token (stored as
--                         sha256, as tenant_invites' is since w1/041).
--   tenants.organization_id  NULL for a standalone workspace.
--   tenants.privacy       invite_only (Render's default for new workspaces)
--                         or public (any org Member may join it).
CREATE TABLE organizations (
    id                   text PRIMARY KEY,
    name                 text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 255),
    primary_workspace_id text NOT NULL UNIQUE REFERENCES tenants (id) ON DELETE CASCADE,
    created_at           timestamptz NOT NULL DEFAULT now(),
    -- The org audit log starts here: "Retention does not begin until you
    -- upgrade" (render.com/docs/audit-logs).
    audit_visible_from   timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE organization_members (
    organization_id text NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    subject         text NOT NULL,
    role            text NOT NULL CHECK (role IN ('owner', 'member', 'guest')),
    created_at      timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (organization_id, subject)
);
CREATE INDEX organization_members_subject_idx ON organization_members (subject);

CREATE TABLE organization_invites (
    id              text PRIMARY KEY,
    organization_id text NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    email           text NOT NULL,
    role            text NOT NULL CHECK (role IN ('owner', 'member')),
    token_hash      text NOT NULL UNIQUE,
    invited_by      text NOT NULL DEFAULT '',
    created_at      timestamptz NOT NULL DEFAULT now(),
    expires_at      timestamptz NOT NULL,
    accepted_at     timestamptz
);
CREATE UNIQUE INDEX organization_invites_pending_idx
    ON organization_invites (organization_id, email) WHERE accepted_at IS NULL;
-- The sign-in redemption looks pending invites up by email alone.
CREATE INDEX organization_invites_email_idx ON organization_invites (email) WHERE accepted_at IS NULL;

ALTER TABLE tenants
    ADD COLUMN organization_id text REFERENCES organizations (id) ON DELETE SET NULL,
    ADD COLUMN privacy text NOT NULL DEFAULT 'invite_only' CHECK (privacy IN ('invite_only', 'public'));
CREATE INDEX tenants_organization_idx ON tenants (organization_id) WHERE organization_id IS NOT NULL;

-- ADR115 D9: every existing Scale and Enterprise workspace becomes the
-- primary of its own org, named after it. bex never recorded who upgraded a
-- workspace, and an admin already controls it, so its people with the admin
-- role become Owners and every other person a Member (Render: "all other
-- workspace members become org members"). API keys' machine rows are not
-- people and join no org. The org ids are minted here in the id package's
-- shape: org- plus 20 characters of xid's alphabet (0-9a-v).
CREATE TEMPORARY TABLE organization_backfill ON COMMIT DROP AS
SELECT t.id AS tenant_id,
       'org-' || (
           SELECT string_agg(substr('0123456789abcdefghijklmnopqrstuv', 1 + floor(random() * 32)::int, 1), '')
           FROM generate_series(1, 20) AS g(i)
           WHERE t.id IS NOT NULL
       ) AS org_id,
       COALESCE(NULLIF(t.display_name, ''), t.name) AS name
FROM tenants t
WHERE t.plan IN ('scale', 'enterprise');

INSERT INTO organizations (id, name, primary_workspace_id)
SELECT org_id, name, tenant_id FROM organization_backfill;

UPDATE tenants t SET organization_id = b.org_id
FROM organization_backfill b WHERE t.id = b.tenant_id;

INSERT INTO organization_members (organization_id, subject, role)
SELECT b.org_id, m.subject, CASE WHEN m.role = 'admin' THEN 'owner' ELSE 'member' END
FROM organization_backfill b
JOIN tenant_members m ON m.tenant_id = b.tenant_id AND m.kind = 'user';

-- Two new workspace relations (ADR115 D5): an Org Member joining a Public
-- workspace (can_join) and an Owner joining any org workspace as admin
-- (can_join_as_admin). The relation CHECK must list core.RelCanRelations()
-- exactly (TestAuditRelationCheckMatchesRelCan).
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
