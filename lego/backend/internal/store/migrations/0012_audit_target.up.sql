-- w3/m7: give an audit event the resource it acted ON, so the per-service
-- events feed can be a VIEW rather than a second write path.
--
-- HAZARD WORTH KNOWING (it bit this migration while it was still numbered 0011,
-- before w2/m10 took that slot): two earlier collision fixes RENUMBERED migrations
-- downward (1a507b0 owner_ids 0009→0010; 3182f27 usage →0006), so a database
-- migrated during that window can record a schema_migrations version HIGHER than
-- the files that actually ran — the local mock cluster's bex-db sat at version 11
-- with only ten migrations applied. golang-migrate then SILENTLY skips any file
-- numbered at or below that version. It is a quiet failure here in particular,
-- because an audit Record error is swallowed by design (core.Base.emit: recording
-- must never fail the verb it records) — a skipped ALTER surfaces not as an error
-- but as a permanently EMPTY events feed. If `target` is missing on a database
-- whose recorded version says otherwise, that is why: force the version back to
-- the last migration whose DDL is actually present, then re-run.
--
-- audit_events.resource is the OpenFGA object a verb was authorized AGAINST —
-- almost always the caller's workspace ("workspace:tea-…"), which cannot say
-- WHICH service was suspended. target is that missing dimension: the resource
-- NAME the verb acted on, "service:<app>" (core.ServiceTarget), written at the
-- same single interception point every write verb already funnels through
-- (core.Base.AuthorizeTarget → emit). Empty for a workspace-wide verb
-- (workspace rename/delete, api-key mint), so existing rows need no backfill:
-- '' means "not about one named resource", which is exactly what they are.
--
-- Redaction is structural and survives this: a target is a name, never a value
-- — the same reason audit rows never carried env-var values.
--
-- Typed prefix ("service:") rather than a bare name so a later feed for another
-- resource kind ("postgres:", "keyvalue:") needs no migration, matching the
-- shape `resource` already uses.
ALTER TABLE audit_events ADD COLUMN target text NOT NULL DEFAULT '';

-- internal/events reads one service's feed newest-first: (target, at DESC, id
-- DESC) is the keyset it pages on.
--
-- NOT partial (`WHERE target <> ''`, the obvious space optimization): pgx runs
-- these as server-side PREPARED statements, and in a GENERIC plan the planner
-- cannot prove that `target = $2` implies `target <> ''` — $2 is not a Const — so
-- it drops a partial index and falls back to scanning by workspace. Measured on a
-- 2M-row table that is a 31,930-buffer (~250 MB), 150 ms plan instead of a 367-
-- buffer, 2 ms one. Postgres' plan_cache_mode=auto does notice and revert to the
-- custom plan, so the exposure is one slow execution per prepared statement per
-- pooled connection rather than steady state — but the predicate buys only disk
-- space, and it is not worth a cost-comparison race for that.
CREATE INDEX audit_events_target_at_idx ON audit_events (target, at DESC, id DESC);

-- The feed's OTHER source needs an index the deploy-history feature never did.
-- `deploys` is indexed on (app_id, created_at DESC) (0005) — which serves the
-- deploy_started projection, but NOT deploy_ended: that one orders and bounds on
-- finished_at, so without this it bitmap-scans every deploy the app has ever had
-- and filters in the heap. Measured on an app with 300 deploys, the default
-- one-hour feed query read 367 buffers, 304 of them (83%) this one branch finding
-- a single row; with the index below, 65 buffers total and 4 for the branch. It
-- also makes the branch cost O(deploys in the window) rather than O(deploys ever),
-- which is the property the audit side already had.
CREATE INDEX deploys_app_finished_idx ON deploys (app_id, finished_at DESC)
    WHERE finished_at IS NOT NULL;
