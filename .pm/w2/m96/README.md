# w2 · m96 — Charges and webhook deliveries name their resource, even after deletion

**Worker:** worker2 **Goal:** a charge line and a webhook delivery row keep the display name their resource had while the usage accrued or the event fired, and say when that resource no longer exists. A bare `srv-…` or sandbox UUID appears only for rows recorded before the fix, never for a resource bex once knew the name of. **Status:** t001–t007 done; **t008 closeout BLOCKED** on live production verification (same credential gate as m95 — see the workstream README)

## Tasks (in order)

| id   | title                                                                                                                                  | est | depends_on |
| ---- | -------------------------------------------------------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Retain a tenant-scoped last display name per metered id, written on create and rename, surviving deletion, purged with the workspace — **DONE** | 45m | —          |
| t002 | Usage cost rows resolve names from the retained record on REST, GraphQL and MCP; sandboxes label by session repo and branch — **DONE** | 35m | t001       |
| t003 | Charges card renders `name (deleted)` for a resource that no longer resolves and falls back to the id only when no name was recorded — **DONE** | 20m | t002       |
| t004 | Webhook delivery view carries `serviceName` from the stored payload; the deliveries table shows the linked name with the id secondary — **DONE** | 30m | —          |
| t005 | Render parity — **DONE**                                                                                                               | 15m | t003, t004 |
| t006 | Simplify — **DONE**                                                                                                                    | 15m | t005       |
| t007 | Test coverage — **DONE**                                                                                                               | 40m | t005       |
| t008 | Closeout                                                                                                                               | 10m | t007       |

## Backfill decision (t002 step 5)

**Backfill what is still knowable, at migration time; leave the genuinely-lost as bare ids.** Migration `0121` runs two INSERTs:

- **Services** — every live `apps` row, using the same `COALESCE(NULLIF(display_name,''), name)` rule readers already use. So every service that exists when the deploy lands is named forever after, including once it is deleted.
- **Sandboxes** — every `agent_sessions.sandbox_id`, plus every `agent_session_dispatches.previous_sandbox_id` joined back to its session, labelled `repo` or `repo (branch)`. A session outlives the sandbox it ran in (sessions are archived, never dropped), which is exactly how the 13 metered UUIDs that no live `sandboxes` or `agentSessions { sandboxId }` query lists still get a name.

**Not backfilled:** a service, Postgres or Key Value **already deleted** before the migration. Its name exists nowhere the backfill can reach — the row is gone from `apps`, the CR is gone from the cluster. Audit events were considered as a source and rejected: they are retained for 90 days (`BEX_AUDIT_RETENTION_DAYS`), so the coverage would be arbitrary and period-dependent, and a partially-named historical month reads as a bug rather than as history. Those rows keep their bare id, and the Charges card renders the id (no `(deleted)` marker, since claiming deletion without a name adds nothing).

This is why the DoD's first bullet is written the way it is: the three `srv-…` rows from 2026-09-14 stay bare ids unless those services were still alive when the migration ran, and the fresh probe (create → meter → delete) is the real test.

## Decisions

- **Postgres/Key Value/sandbox names are captured at *meter read* time, not at their create/rename verbs** — a deliberate deviation from t001 step 2, which asked for hooks in all four packages' create paths. Services **do** get the create/rename hooks it asked for (`store.CreateApp`, in the app's own transaction, and `store.SetAppDisplayName`), because they have a store row to hang them off. The other three do not: their creates live in `internal/postgres`, `internal/keyvalue` and `internal/sandbox`, and a missed path there fails **silently** — you find out months later when a charge line has no name and the name is already unrecoverable. `resolveServiceNames` is, by construction, the one place every *metered* resource is enumerated, so capturing there cannot miss a path; and "the name it had while the usage accrued" is precisely the value the charge line wants. The write is batched, skips unchanged names, and is best-effort — a failure is logged and never fails the usage read.
- **Purge rides the tenants cascade, not a new `WorkspacePurger`.** `resource_display_names.tenant_id` is `REFERENCES tenants (id) ON DELETE CASCADE`, so the existing `DeleteTenant` in the workspace teardown removes every row. Adding a purger would have been a second thing to keep in sync for no gain. Proven by `TestResourceDisplayNamesPG` against real Postgres.
- **Keyed by id, so a re-created resource is a separate line.** Two services that shared a display name across a delete/re-create are two rows and two charge lines — which is what the DoD asks for, and what billing correctness requires.
- **`deleted` is a third state, not the absence of a name.** `serviceName` set + `deleted: false` = live. Set + `deleted: true` = gone, name retained. Empty + `deleted: false` = bex never recorded a name (a pre-migration row). The card renders those three as *name*, *name (deleted)*, and the bare id.
- **A live name always beats a retained one**, so renaming a service moves its charges to the new name on the next read, and the retained record is moved forward at the same time.
- **Sandbox labels are derived by SQL join, not stored on the sandbox.** A sandbox has no name field and is reaped long before its charges stop mattering. `SandboxLabels` reads the owning agent session (current sandbox *or* a dispatch's `previous_sandbox_id`), so a rehydrated session labels every sandbox in its chain.

## Verification note

The migration, its two backfills, the cascade purge and the sandbox join were run against **real Postgres 17** locally (`TestResourceDisplayNamesPG`, `TestSandboxLabelsPG`), not only against fakes — the SQL is executed, not just shipped. CI runs the same tests against its own ephemeral Postgres.

## Simplify + coverage notes (t006, t007)

- **Applied:** the ad-hoc `ResourceKind + "/" + ID` key that `resolveServiceNames` and `nameResourceEstimates` each spelled by hand is now one exported `store.ResourceDisplayNameKey`, shared with the new accessors and the tests — the two spellings had no guard keeping them equal.
- **Applied:** `SetAppDisplayName` resolves the `COALESCE(NULLIF(display_name,''), name)` fallback itself instead of leaving the empty-means-fall-back rule for the retained record's readers to re-implement.
- **Declined:** collapsing `ResourceDisplayNames` and `SandboxLabels` into one accessor. They answer different questions from different tables (a durable record vs a live join over sessions/dispatches) and only the sandbox one has a fallback chain; merging would need a kind-switch inside the query.
- **Coverage:** four unit tests in `internal/usage/retained_names_test.go` (deleted keeps its name, sandbox labelling incl. the reaped case, rename capture + unchanged-name skip, estimates carry both fields), one cross-surface test (`TestRetainedNamesAgreeAcrossAdapters`), two real-Postgres store tests, plus the t004 delivery tests. The webhook `serviceName` extractor is table-driven over ten payload shapes including malformed and pre-field ones.

## Corrections to the task specs, found while implementing

- **MCP *does* have a delivery read.** t004's "Out of scope" said "MCP delivery reads (none exist today; note if that changes in t005)". `listDeliveries` returns `[]DeliveryView`, so MCP picks `serviceName` up through the JSON tag for free — recorded here as t005 asked.
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

- **Source:** `/pm-brainstorm for w1` 2026-09-15 (proposal 3), absorbing [w1/088](../../w1/done/088.md) (Billing → Charges names deleted services and every sandbox only by bare id) and [w1/090](../../w1/done/090.md) (a webhook's Recent deliveries table names each row's service only by bare `srv-` id). Both found by the live `/qa-find-bugs` hunt of 2026-09-14 (passes 6 and 13).
- **Goal linkage:** pillar 2 of [ADR008](../../../docs/ADR008-vision.md) — billing lines and delivery history are machine-readable state, and an opaque id fails that bar. Sandbox charges are real money on the reopened pillar 5.
- **Expected outcome:** "what did this cost me" and "which service's event failed" are answerable from the page itself, including for resources deleted since.
- **Why now:** `w1/088` explicitly asks to be promoted if name retention needs a migration, and it does. Both notes share one root cause — names resolved only from live rows (`usage/service.go:286-313`, `worker.go:346-349` payload not projected to the delivery view) — so one retained-name record closes both.
- **Render parity included:** REST `GET /v1/usage`, GraphQL `usage` and MCP share `monthToDateAt`, so the name resolution moves together on all three; the webhook delivery view is read on REST and GraphQL and must agree on `serviceName`. Render's own billing view for deleted services and its Recent deliveries column were not captured at filing and t005 records them.
