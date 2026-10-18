# w8 · m114 — Move environment-group metadata from OpenBao to the control-plane store

**Worker:** worker8 **Goal:** OpenBao holds only environment-group secret values (the env and files maps). Every non-secret fact — identity, owning workspace, name uniqueness, environment membership, service links, timestamps, the revision lock and in-flight operation state — lives in Postgres under constraints and transactions. All legacy-tenant, locator, name-claim, tombstone-fence and lazy-migration code is deleted, and so is the one-shot migrator once production has cut over. **Status:** in progress — t001–t006, t008 and t010–t011 done; t007 needs protected snapshot comparison (snapshot, apply, prune and live checks recorded in [t007.md](t007.md)); t009 final-image owned-fixture/UI acceptance pending; t012 closeout pending

**Principle (user decision 2026-10-18):** the clean end state outranks compatibility. Bex-specific client behavior may break, and no dual-read or dual-write path ships in serving code. Two things are not negotiable: Render's wire contract, and customer data — every live group's values, name, workspace, environment, links and timestamps survive the cutover.

## Tasks (in order)

| id   | title                                                                     | est | depends_on                 |
| ---- | ------------------------------------------------------------------------- | --- | -------------------------- |
| t001 | Design the end state and the cutover; rewrite ADR013's env-group sections | 45m | — — **DONE** |
| t002 | Schema and store layer for groups, links and operation state              | 60m | w8/m114/t001 — **DONE**               |
| t003 | Serve metadata, names and links from the store                            | 90m | w8/m114/t002 — **DONE**               |
| t004 | Move the revision lock and operation records to the store                 | 60m | w8/m114/t003 — **DONE**               |
| t005 | Rewire consumers: environments, apps, Blueprints, purge                   | 60m | w8/m114/t003 — **DONE**               |
| t006 | One-shot cutover migrator (dry-run / apply / prune), rehearsed on dev-8   | 75m | w8/m114/t004, w8/m114/t005 — **DONE** |
| t007 | Production cutover in a user-authorized window                            | 30m | w8/m114/t006               |
| t008 | Delete the compatibility code and the migrator — **DONE** | 60m | w8/m114/t007               |
| t009 | Render parity: env-group surfaces and deliberate breaks                   | 30m | w8/m114/t008               |
| t010 | Simplify the changed code — **DONE** | 20m | w8/m114/t009               |
| t011 | Test coverage: constraints, zero-OpenBao reads, save recovery — **DONE** | 50m | w8/m114/t009               |
| t012 | Close out the milestone                                                   | 10m | w8/m114/t011               |

Twelve tasks, 590m.

## Definition of done

- **OpenBao end state (production, after prune):** every env-group path is a live group's `env` or `files` map under its workspace tenant, plus transient `op/<opID>/{old,new}-{env,files}` value maps only while a save is in flight. Zero `meta`, `revision`, `op` record, name-claim, locator or tombstone paths remain, and nothing remains under the legacy tenant's `env-groups/`. Postgres env-group row counts per workspace equal the dry-run's live-group counts.
- **Reads:** listing env groups, reading one group's view and computing environment membership make zero OpenBao calls (test-asserted with a counting `core.SecretKV`); only reveals and saves read value maps. Live: `GET /v1/env-groups?ownerId=tea-d98210cbbpdc73dcrkvg` < 1s; the pinned CLI's every-project `GET /v1/environments` < 2s; `BEX_WORKSPACE=tea-d98210cbbpdc73dcrkvg bex services -o json` exits 0 in < 10s, twice.
- **Integrity by constraint:** two concurrent creates or renames to one name in a workspace yield exactly one success and one 409 `ENV_GROUP_NAME_EXISTS`; deleting a service removes its links (`UnlinkApp`), deleting an environment clears membership (FK `ON DELETE SET NULL`), and deleting a workspace removes its rows and its OpenBao env-group prefix. A deleted group cannot be resurrected under its id, with no tombstone.
- **Content saves keep their contract:** a stale `expectedRevision` answers 409; an interrupted save is restored or finalized by another instance; projections stay revision-ordered (ADR013 w5/m127). The existing save, clone and recovery behavior tests pass on the new store.
- **Code end state:** none of these remain anywhere under `lego/`: `legacyCtx`, `isLocator`, `writeMetaLocator`, the legacy union in `listGroupIDs`, `scanGroups`, `plantDeletedMetaFence`, `mutateMetaCAS`, `envGroupNameClaimPath`, `AuditNameClaims`, `MigratePaths`, `needsLinkMigration` / `migrateLinks` / `linksByID`, `ENV_GROUP_NAME_RESERVED`, `BEX_ENV_GROUP_PATH_MIGRATION`, `BEX_ENV_GROUP_NAME_CLAIM_AUDIT`, env-group store-off branches, and the cutover migrator itself. `docs/runbooks/env-group-path-migration.md` is removed. `make lint` (deadcode included) and `cd lego/backend && go test ./...` pass.
- **Wire contract:** Render-shaped REST/GraphQL/MCP env-group responses and the dashboard are unchanged apart from the deliberate breaks below, each recorded in ADR013 and ADR018.

## Deliberate breaks

- Env groups require the control-plane store; the store-off mode is gone.
- `ENV_GROUP_NAME_RESERVED` (the abandoned-reservation 409 with `retryAfterSeconds`) is removed, since a unique index has no abandoned reservations. `ENV_GROUP_NAME_EXISTS` stays.
- Env-group endpoints, or the whole API, are unavailable during the cutover window.
- The startup modes `BEX_ENV_GROUP_PATH_MIGRATION` and `BEX_ENV_GROUP_NAME_CLAIM_AUDIT` and their runbook are removed.

## What goes away (state at `d4eb27ba2`)

- **Non-secret OpenBao paths per group:** `env-groups/<gid>/meta` (name, workspace, environment, links, `linksByID`, timestamps; `type meta`, `lego/backend/internal/envgroups/service.go:258`), `/revision` (`patch.go:73`), the `/op` record (`recovery.go:49`), name claims `env-group-name-claims/<sha256>` (`service.go:1741`), legacy-tenant locators (`tenant.go:181`) and permanent resurrection fences `{"tombstoned":"1"}` (`plantDeletedMetaFence`, `service.go:995`, w4/m97).
- **Machinery that exists only because a KV store has no index, constraint or transaction:** `scanGroups` and its bounded fan-out (`service.go:483`, w8/m113), the legacy union (`tenant.go:122`), `mutateMetaCAS` (`meta.go:99`), name-claim reservation and reclaim (`service.go:1768`), the fences, the lazy migrations (`migrateLinks` `links.go:131`; ownerless groups attributed to `core.DefaultTenant` inside `readMeta`, `service.go:2085`), `MigratePaths` (`migrate.go:59`) and `AuditNameClaims` (`nameclaims.go:53`). The startup wiring is at `lego/backend/cmd/api/main.go:381-400`, config at `cmd/api/config.go:193-194`.
- **Postgres has no env-group table today.** The foreign-key targets are `tenants(id)`, `environments(id)` and `apps(id)`; migrations live in `lego/backend/internal/store/migrations/`.

## Source + Goal linkage

- **Source:** user decision 2026-10-18, during the w8/m113 diagnosis: non-secret env-group metadata does not belong in OpenBao, and the migration should be as clean as possible — "the clean end-state code matters more than not breaking clients; breaking is acceptable." Origin trace: `7eedb6175` (w1/m16, 2026-07-09) stored everything in OpenBao ("reusing the existing OpenBao store (no new backend)"), even though the plan ([w1/m16 t001](../../w1/done/m16/t001.md)) called a group "a control-plane store object … plus an OpenBao path for its values" and left "store vs OpenBao metadata" open. The Postgres store had existed since `aebbd4342` (w1/m2, 2026-07-06). Later changes kept extending the same choice: `c798a3dee` and `e3145bf95` (2026-07-14, workspace and environment into meta), `cacde0622` (2026-08-17, 15s scan cache), `ffc1cd6bb` (w2/m80, 2026-08-19, prefix sweep and legacy union, cache retired) and `2755efcc4` (2026-09-08, CAS metadata). Symptom: [w8/m113](../done/m113/README.md) — `bex services` timed out in workspace bex because every scan reads about 260 fenced ids at roughly 50ms each ([profile](../done/m113/done/t001.md)).
- **Supersedes [w8/119](../119.md) in full:** fence ageing, the one-read scan and a process-wide read budget all disappear with the scan. It also deletes w8/m113's concurrent scan.
- **Goal linkage:** dependable hosting ([ADR008](../../../docs/ADR008-vision.md)); OpenBao as the secret store ([ADR013](../../../docs/ADR013-secrets.md)) beside bex-api's Postgres control plane ([ADR006](../../../docs/ADR006-bex-api.md)); Render parity ([ADR018](../../../docs/ADR018-render-parity.md)) for env-group lists and the CLI services journey.
- **Expected outcome:** env-group and environment reads become indexed SQL with no OpenBao calls. Name uniqueness, link cleanup and deletion become constraints and transactions instead of hand-built CAS records. The envgroups package loses its KV workaround layer, and `bex services` in workspace bex answers in under 10s.
- **Why now:** m113's concurrent scan removed the timeout, but the cost stays linear in groups ever deleted and grows with every QA fixture in that workspace. Every new env-group feature currently extends a KV workaround. The w2/m80 dual-read window has been open since 2026-08-19. The cutover is cheapest while the data is small (17 first-party workspaces, per DO_NOT_DO 2026-08).
- Render parity closing task applies (t009): env groups are exposed on REST, GraphQL, MCP and the dashboard.
- **Production writes:** t007 runs only inside a window the user explicitly authorizes, after a fresh OpenBao snapshot ([ADR031](../../../docs/ADR031-platform-data-backup.md)).
- **Size:** 590m is larger than usual on purpose. The cutover and the deletion of the migrator belong to one end state, and splitting them would ship a milestone that leaves dual-layout code behind.
