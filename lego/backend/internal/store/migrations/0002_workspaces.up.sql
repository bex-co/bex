-- w6/m1: workspaces become a user-facing product object (create/rename/delete,
-- plan limits). Two changes to the source of truth:
--
-- (1) tenants.plan carries Render's post-2026-04-23 workspace plan lineup
--     (hobby | pro | scale | enterprise), NOT a compute/instance tier. The
--     column pre-dates this distinction and defaulted to the compute catalog's
--     'free', which leaked in via the internal tenant API. Remap any legacy
--     value to 'hobby' (the free tier's successor), flip the default, then
--     constrain to the four real plans. Compute/instance sizing stays on
--     apps.tier (lego/types/tiers) — a separate ladder, untouched here.
--
-- (2) tenant_members records who belongs to a workspace and with what role —
--     the membership substrate the lifecycle verbs write (owner row on create,
--     cascade-removed on delete) and that w4/m12 (invite/roles) will extend.
--     subject is the OpenFGA user (Kratos identity id or Hydra client id), the
--     same string granted `admin` on workspace:<tenant-id> in OpenFGA.

UPDATE tenants SET plan = 'hobby' WHERE plan NOT IN ('hobby', 'pro', 'scale', 'enterprise');
ALTER TABLE tenants ALTER COLUMN plan SET DEFAULT 'hobby';
ALTER TABLE tenants ADD CONSTRAINT tenants_plan_check CHECK (plan IN ('hobby', 'pro', 'scale', 'enterprise'));

CREATE TABLE tenant_members (
    tenant_id  text NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    subject    text NOT NULL,
    role       text NOT NULL DEFAULT 'admin',
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, subject)
);

-- The caller's workspaces are looked up by subject (the switcher / list verb),
-- so index the non-leading key.
CREATE INDEX tenant_members_subject_idx ON tenant_members (subject);
