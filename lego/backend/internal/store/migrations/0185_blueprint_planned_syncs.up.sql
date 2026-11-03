-- w2/m256: Render's Blueprint onboarding (CLI v2.35 `ea blueprints create |
-- plan | approve`). Creating a Blueprint plans its first sync without applying
-- it; a planned sync is a blueprint_syncs row in the `created` state carrying
-- the Blueprint path it was planned at and the manual env vars approval must
-- supply. Approval applies exactly the planned commit and turns that row into
-- the admitted run. A non-null plan_path marks a planning attempt (planned or
-- failed) as opposed to an ordinary sync run.
ALTER TABLE blueprint_syncs
    ADD COLUMN plan_path text,
    ADD COLUMN required_manual_env_vars jsonb NOT NULL DEFAULT '[]';

-- How planning treats resources that already exist by name: Render's `adopt`
-- (the default) or `create_new`, which suffixes every managed resource name.
ALTER TABLE blueprints
    ADD COLUMN existing_resources text NOT NULL DEFAULT 'adopt'
        CHECK (existing_resources IN ('adopt', 'create_new')),
    ADD COLUMN resource_suffix text NOT NULL DEFAULT '';
