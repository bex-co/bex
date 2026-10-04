# w2 · m96 — Charges and webhook deliveries name their resource, even after deletion

**Worker:** worker2 **Goal:** a charge line and a webhook delivery row keep the display name their resource had while the usage accrued or the event fired, and say when that resource no longer exists. A bare `srv-…` or sandbox UUID appears only for rows recorded before the fix, never for a resource bex once knew the name of. **Status:** blocked (2026-10-03 live acceptance: rename **FAILS** on production — fix `DisplayName`-aware usage name resolution is local and **awaits /ship** + deploy, then a re-probe; workspace-purge check needs an operator because workspace creation is payment-gated for every plan and no product surface can read `resource_display_names` for a deleted tenant). t001–t007 done; t008 open.

## Scope transfer — 2026-09-28

User approved moving t009 implementation to `w1/112` in w1. Its original file was removed after preserving its scope/evidence at the destination; t009 is retired, not done and must not be reused. Historical references below describe the original filing. Closeout now depends on the external item plus all existing acceptance obligations. This milestone remains open.

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Retain a tenant-scoped last display name per metered id, written on create and rename, surviving deletion, purged with the workspace — **DONE** | 45m | — |
| t002 | Usage cost rows resolve names from the retained record on REST, GraphQL and MCP; sandboxes label by session repo and branch — **DONE** | 35m | t001 |
| t003 | Charges card renders `name (deleted)` for a resource that no longer resolves and falls back to the id only when no name was recorded — **DONE** | 20m | t002 |
| t004 | Webhook delivery view carries `serviceName` from the stored payload; the deliveries table shows the linked name with the id secondary — **DONE** | 30m | — |
| t005 | Render parity — **DONE** | 15m | t003, t004 |
| t006 | Simplify — **DONE** | 15m | t005 |
| t007 | Test coverage — **DONE** | 40m | t005 |
| t008 | Closeout | 10m | t007, w1/112 |

## Backfill decision (t002 step 5)

**Backfill what is still knowable, at migration time; leave the genuinely-lost as bare ids.** Migration `0121` runs two INSERTs:

- **Services** — every live `apps` row, using the same `COALESCE(NULLIF(display_name,''), name)` rule readers already use. So every service that exists when the deploy lands is named forever after, including once it is deleted.
- **Sandboxes** — every `agent_sessions.sandbox_id`, plus every `agent_session_dispatches.previous_sandbox_id` joined back to its session, labelled `repo` or `repo (branch)`. A session outlives the sandbox it ran in (sessions are archived, never dropped), which is exactly how the 13 metered UUIDs that no live `sandboxes` or `agentSessions { sandboxId }` query lists still get a name.

**Not backfilled:** a service, Postgres or Key Value **already deleted** before the migration. Its name exists nowhere the backfill can reach — the row is gone from `apps`, the CR is gone from the cluster. Audit events were considered as a source and rejected: they are retained for 90 days (`BEX_AUDIT_RETENTION_DAYS`), so the coverage would be arbitrary and period-dependent, and a partially-named historical month reads as a bug rather than as history. Those rows keep their bare id. The Charges card renders the id **with** a `(deleted)` marker: `3757d0ff1` ("tombstone every charge row whose resource is gone", 2026-09-22) superseded the original "no marker" rule, so every charge row whose resource no longer resolves is tombstoned, named or not (re-verified live 2026-10-02: `srv-dae9lb988i5c7399e9m0` reads as a bare id with `deleted: true`).

**Sandboxes from before `w1/112`** (`de9ac4d1c`) keep the `<tier> sandbox` label (for example `starter sandbox`): `sandbox_meter_states` never stored an image and the pre-fix sandboxes are reaped, so `plan · image` cannot be reconstructed for them, and `w1/112` explicitly scoped out renaming them rather than fabricating lost image metadata. Sandboxes created since then retain `plan · image` (or their session's repo/branch, or `agent session <id>` for a repo-less session) at create.

This is why the DoD's first bullet is written the way it is: the three `srv-…` rows from 2026-09-14 stay bare ids unless those services were still alive when the migration ran, and the fresh probe (create → meter → delete) is the real test.

## Decisions

- **Postgres/Key Value/sandbox names are captured at _meter read_ time, not at their create/rename verbs** — a deliberate deviation from t001 step 2, which asked for hooks in all four packages' create paths. Services **do** get the create/rename hooks it asked for (`store.CreateApp`, in the app's own transaction, and `store.SetAppDisplayName`), because they have a store row to hang them off. The other three do not: their creates live in `internal/postgres`, `internal/keyvalue` and `internal/sandbox`, and a missed path there fails **silently** — you find out months later when a charge line has no name and the name is already unrecoverable. `resolveServiceNames` is, by construction, the one place every _metered_ resource is enumerated, so capturing there cannot miss a path; and "the name it had while the usage accrued" is precisely the value the charge line wants. The write is batched, skips unchanged names, and is best-effort — a failure is logged and never fails the usage read.
- **Purge rides the tenants cascade, not a new `WorkspacePurger`.** `resource_display_names.tenant_id` is `REFERENCES tenants (id) ON DELETE CASCADE`, so the existing `DeleteTenant` in the workspace teardown removes every row. Adding a purger would have been a second thing to keep in sync for no gain. Proven by `TestResourceDisplayNamesPG` against real Postgres.
- **Keyed by id, so a re-created resource is a separate line.** Two services that shared a display name across a delete/re-create are two rows and two charge lines — which is what the DoD asks for, and what billing correctness requires.
- **`deleted` is a third state, not the absence of a name.** `serviceName` set + `deleted: false` = live. Set + `deleted: true` = gone, name retained. Empty + `deleted: false` = bex never recorded a name (a pre-migration row). The card renders those three as _name_, _name (deleted)_, and the bare id.
- **A live name always beats a retained one**, so renaming a service moves its charges to the new name on the next read, and the retained record is moved forward at the same time.
- **Sandbox labels are derived by SQL join, not stored on the sandbox.** A sandbox has no name field and is reaped long before its charges stop mattering. `SandboxLabels` reads the owning agent session (current sandbox _or_ a dispatch's `previous_sandbox_id`), so a rehydrated session labels every sandbox in its chain.

## Verification note

The migration, its two backfills, the cascade purge and the sandbox join were run against **real Postgres 17** locally (`TestResourceDisplayNamesPG`, `TestSandboxLabelsPG`), not only against fakes — the SQL is executed, not just shipped. CI runs the same tests against its own ephemeral Postgres.

## Simplify + coverage notes (t006, t007)

- **Applied:** the ad-hoc `ResourceKind + "/" + ID` key that `resolveServiceNames` and `nameResourceEstimates` each spelled by hand is now one exported `store.ResourceDisplayNameKey`, shared with the new accessors and the tests — the two spellings had no guard keeping them equal.
- **Applied:** `SetAppDisplayName` resolves the `COALESCE(NULLIF(display_name,''), name)` fallback itself instead of leaving the empty-means-fall-back rule for the retained record's readers to re-implement.
- **Declined:** collapsing `ResourceDisplayNames` and `SandboxLabels` into one accessor. They answer different questions from different tables (a durable record vs a live join over sessions/dispatches) and only the sandbox one has a fallback chain; merging would need a kind-switch inside the query.
- **Coverage:** four unit tests in `internal/usage/retained_names_test.go` (deleted keeps its name, sandbox labelling incl. the reaped case, rename capture + unchanged-name skip, estimates carry both fields), one cross-surface test (`TestRetainedNamesAgreeAcrossAdapters`), two real-Postgres store tests, plus the t004 delivery tests. The webhook `serviceName` extractor is table-driven over ten payload shapes including malformed and pre-field ones.

## Corrections to the task specs, found while implementing

- **MCP _does_ have a delivery read.** t004's "Out of scope" said "MCP delivery reads (none exist today; note if that changes in t005)". `listDeliveries` returns `[]DeliveryView`, so MCP picks `serviceName` up through the JSON tag for free — recorded here as t005 asked.
- **A delivery's `serviceId` is not a deletion signal.** The stored attempt keeps its id string forever, so absence of a name means "recorded before this field existed", not "deleted". The deliveries table therefore detects deletion by resolving the id against the live service list, fails **open** while that list is loading or errored, and only marks `srv-…` subjects (a `dpg-…` datastore subject is never in the services list and must not be mislabelled).

## Definition of done

Run each bullet on production (workspace `bex` / `tea-d98210cbbpdc73dcrkvg`) after the deploy, with fixtures created and deleted inside the run:

- **Deleted services are named in Charges.** On `/billing` → Charges → Expand all, period `2026-09`, the three rows that read `srv-dae9lb988i5c7399e9m0`, `srv-da7o6ovvqdcc73bpn9hg` and `srv-dah41v9c7cos73dm39ug` at filing time (2026-09-14) show a display name with a `(deleted)` marker, or a bare id **only** if the backfill decision recorded below explicitly leaves pre-fix rows unnamed. A fresh probe — create a web service, let one usage rollup pass, delete it — shows `<name> (deleted)` on the next rated period, on the Charges card and in `usage(ownerId) { estimatedCost { resources { serviceName } } }`.
- **Sandboxes are labelled.** The `271ec9ce-a32b-4128-bb43-02a9e57b01b6` row ($69.98 at filing) and every other sandbox row shows either its owning agent session's repo and branch or `plan · image`, never a bare UUID.
- **Live resources show their current name.** Rename a fixture service; the Charges row shows the new name on the next read. Delete it and re-create one with the same display name; the two stay separate lines keyed by id.
- **A delivery row names its service.** Create a webhook subscribed to Deploy events, deploy a fixture, open `/webhook/<whk-id>` → Activity → Recent deliveries. The Service column shows the service's name linked to the service, with the `srv-…` id visible as secondary text or tooltip.
- **A delivery for a deleted service keeps its name.** Delete the fixture service; the same delivery row still shows the recorded name (with a deleted marker), not a bare id. REST and GraphQL delivery reads both carry `serviceName`.
- **Workspace deletion purges retained names.** Deleting a throwaway workspace (the `w1/m61` teardown path) leaves no rows in the retained-name record for that tenant.
- **Backfill decision recorded** in this README under `## Backfill decision` by t002.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w1` 2026-09-15 (proposal 3), absorbing [w1/088](../../../w1/done/088.md) (Billing → Charges names deleted services and every sandbox only by bare id) and [w1/090](../../../w1/done/090.md) (a webhook's Recent deliveries table names each row's service only by bare `srv-` id). Both found by the live `/qa-find-bugs` hunt of 2026-09-14 (passes 6 and 13).
- **Goal linkage:** pillar 2 of [ADR008](../../../../docs/ADR008-vision.md) — billing lines and delivery history are machine-readable state, and an opaque id fails that bar. Sandbox charges are real money on the reopened pillar 5.
- **Expected outcome:** "what did this cost me" and "which service's event failed" are answerable from the page itself, including for resources deleted since.
- **Why now:** `w1/088` explicitly asks to be promoted if name retention needs a migration, and it does. Both notes share one root cause — names resolved only from live rows (`usage/service.go:286-313`, `worker.go:346-349` payload not projected to the delivery view) — so one retained-name record closes both.
- **Render parity included:** REST `GET /v1/usage`, GraphQL `usage` and MCP share `monthToDateAt`, so the name resolution moves together on all three; the webhook delivery view is read on REST and GraphQL and must agree on `serviceName`. Render's own billing view for deleted services and its Recent deliveries column were not captured at filing and t005 records them.

## Live re-probe (2026-09-26, `/qa-find-bugs` pass 219)

Production, workspace `bex`, deployed `726042a28`, `muse.env` QA credentials. The fixtures were a free web service `srv-das9tr8d0qnc73d7a44g` (`qa-20260926-whk`) and webhook `whk-das9trod0qnc73d7a460`, which POSTed to the fixture's own URL so nothing left bex. Both were deleted, and both `GET`s now return 404.

- **Deleted services are named in Charges — PASS (within the backfill decision).**
  - `usage(ownerId) { estimatedCost { resources { serviceId serviceName deleted } } }` and `/billing` → Charges → Expand all both show services created and deleted after the deploy with their name and `(deleted)`: `qa-20260917-web (deleted)` (`srv-dalo003…`, created 2026-09-17T05:52Z) and `qa-20260917-p2-web (deleted)`.
  - The three 2026-09-14 ids, plus 17 `srv-dak…` ids and `red-dakv3hqsh60c73ao4li0`, read as a bare id with `(deleted)`. Their xid timestamps all fall between 2026-09-05 and 2026-09-16T02:20Z, before `0e490af54` (2026-09-16T05:39Z) could have deployed, so they were deleted before migration 0121 ran. That matches the Backfill decision.
  - **Discrepancy:** the Backfill decision says pre-migration bare ids render **without** a `(deleted)` marker, but the card shows `srv-dae9lb988i5c7399e9m0(deleted)`. `3757d0ff1` ("tombstone every charge row whose resource is gone", 2026-09-22) superseded that sentence. Update the decision text at closeout.
  - The "fresh probe" (create → one rollup → delete) was not run end to end in this pass. The `qa-20260917-*` rows are that probe's shape from an earlier run.
- **Sandboxes are labelled — FAIL (intent).** All 14 sandbox rows read `starter sandbox (deleted)`, including `271ec9ce-a32b-4128-bb43-02a9e57b01b6` at **$78.61**. None is a bare UUID, but none shows repo/branch or `plan · image`, and the costly one cannot be told apart from the other thirteen. Root cause and fix are in **t009**.
- **Live resources show their current name — NOT RUN.** It needs a rename to show up on the next rated read after a usage rollup. Not attempted this pass.
- **A delivery row names its service — PASS.** A manual deploy on the fixture produced `deploy_started` / `deploy_ended` deliveries (HTTP 200). `/webhook/<whk>` → Recent deliveries showed `qa-20260926-whk`, linked to `/services/srv-das9tr8d0qnc73d7a44g`, with the id as secondary text and a `title` tooltip. REST `GET /v1/webhooks/<whk>/events` carried `serviceName: "qa-20260926-whk"`.
- **A delivery for a deleted service keeps its name — PASS.** After `DELETE /v1/services/<srv>` (204), the same rows read `qa-20260926-whk(deleted)`, with the service link removed and the id tooltip kept. REST `serviceName` and GraphQL `webhookDeliveries(endpointId:) { serviceName }` both still returned `qa-20260926-whk`.
- **Workspace deletion purges retained names — NOT RUN.** It needs a throwaway workspace. The cascade is proven only by `TestResourceDisplayNamesPG`.
- **Backfill decision recorded — PASS** (see the discrepancy under bullet 1).


## Dependency refresh — 2026-10-02

The completed record at [w1/done/112](../../../w1/done/112.md) satisfies the transferred sandbox-label implementation dependency: `de9ac4d1c` shipped and its session-less sandbox was created, terminated and read back with an identifiable label through REST/GraphQL on 2026-09-29. That record explicitly leaves the browser walk and live repo-less-session check unprobed. This milestone retains its own remaining production rename, purge, fresh usage-rollup and live acceptance obligations in t008; it is not closed by the transferred note.

## Live acceptance — 2026-10-03

Production, workspace `bex`, QA session (isolated cookie jar, revoked at the end). Rollup cadence is hourly (`usage.Interval`), closing the previous full UTC hour. Fixtures: free image web services `srv-db0a2s8ehcmc739j12u0` (`qa-20261002-m96a`, created 06:32:17Z) and `srv-db0a3b0ehcmc739j1320` (`qa-20261002-m96b`, created 06:33:16Z); both deleted (`DELETE` → 204, `GET` → 404).

- **Fresh probe (create → rollup → delete) — PASS in form.** Both ids first appeared in `usage(ownerId) { estimatedCost { resources { serviceId serviceName deleted } } }` at 07:01:49Z under their names, `deleted: false`; after `DELETE` both read `deleted: true` with a name, on GraphQL and REST `GET /v1/usage` (which agree).
- **Live resources show their current name — FAIL.** `PATCH /v1/services/srv-db0a2s8ehcmc739j12u0 {"displayName":"qa-20261002-m96-renamed"}` → 200 at 07:03Z, and `GET` returns the new `displayName`. Reads at 07:03:31Z, 07:04:24Z and 07:06:23Z, on GraphQL and REST, still showed `qa-20261002-m96a`. After deletion the charge reads `qa-20261002-m96a (deleted)`, the **pre-rename** name. Fixture B was given the same display name after A's deletion (to test "same display name, separate lines"): the two ids stayed two lines (PASS for that half), but B read `qa-20261002-m96b` live and `qa-20261002-m96b (deleted)` after deletion.
  - **Root cause:** `usage.resolveServiceNames` (`lego/backend/internal/usage/service.go`) named services from `store.App.Name`, the immutable creation name. `store.App` did not carry `display_name` at all. Worse, the read treats that name as "live" and recaptures it into `resource_display_names`, overwriting the renamed value `SetAppDisplayName` had just retained. So the "a live name always beats a retained one" decision is applied to the wrong field.
  - **Fix (local, awaits /ship):** `store.App` gains a read-only `DisplayName` (`COALESCE(a.display_name,'')` in `appColumns`/`scanApp`/`ListDesiredApps`). The resolver uses it, falling back to `Name`. Regression tests: `TestRenamedServiceBillsUnderItsDisplayName` (fails without the fix: retained name `created-as`, want `shown-as`), plus a `GetApp` display-name assertion in `TestResourceDisplayNamesPG`. Verified with the full store suite on a disposable Postgres 17, and the full backend `go test ./...`. After deploy, re-run the rename probe: create, wait one rollup, rename, read, delete, read.
- **Sandboxes are labelled — PASS within the recorded decision.** Period `2026-09`: `sbx-datp1888upic73bnk7gg` (the `w1/112` probe) reads `starter · alpine:3`. The 16 pre-`w1/112` UUID rows (including `271ec9ce-a32b-4128-bb43-02a9e57b01b6`) and `sbx-dat20rbncejs739qiv9g` read `starter sandbox` (`deleted: true`), never a bare UUID. Their images were never stored, and `w1/112` scoped out renaming them. The Backfill decision now records this.
- **Workspace deletion purges retained names — OPEN (operator).** `workspaceCreationPolicy(plan:"hobby")` returns `{mode:"all", paymentRequired:true}`, so a throwaway workspace needs a real Stripe payment setup. That is not a safe QA fixture, and none was created. Even with one, no product surface exposes `resource_display_names` for a deleted tenant, so the check needs either an operator-run teardown plus a read-only DB query, or an explicit acceptance of the `TestResourceDisplayNamesPG` cascade proof (`ON DELETE CASCADE` from `tenants`).
- **Backfill decision — updated.** The pre-migration bare-id rows now render with `(deleted)` (`3757d0ff1`). Re-verified on `srv-dae9lb988i5c7399e9m0`, `srv-da7o6ovvqdcc73bpn9hg` and `srv-dah41v9c7cos73dm39ug`: `serviceName: ""`, `deleted: true`.
- **Charges card:** not walked in a browser this pass. It renders the same GraphQL `estimatedCost.resources` read recorded above.

