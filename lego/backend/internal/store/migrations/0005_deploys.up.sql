-- w2/m5: deploy history — every rollout of an app recorded as a row, Render's
-- deploy poll-loop (list_deploys/get_deploy/POST .../deploys). One open
-- (status='update_in_progress') row at a time per app; the reconciler's
-- write-back hook closes it to 'live' or 'update_failed' from the projected
-- App CR's observed status (docs/ADR004-deployment.md health gating).
--
-- commit is carried for forward-compatibility with w1/m5 (build-from-git
-- commit tracking) — always '' until that lands; the deploys feature omits it
-- from its views rather than surfacing an always-empty field.
CREATE TABLE deploys (
    id text PRIMARY KEY,
    app_id text NOT NULL REFERENCES apps(id) ON DELETE CASCADE,
    trigger text NOT NULL,
    image text NOT NULL DEFAULT '',
    commit text NOT NULL DEFAULT '',
    status text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    started_at timestamptz,
    finished_at timestamptz
);

-- ListDeploys reads newest-first per app; OpenDeploy narrows to the latest
-- non-terminal row — both lean on this index.
CREATE INDEX deploys_app_created_idx ON deploys (app_id, created_at DESC);
