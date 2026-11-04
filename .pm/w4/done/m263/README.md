# w4 · m263 — A Key Value plan change warns that it restarts the store, and that a Persistence Off store loses its data, as Render does

**Worker:** worker4 **Goal:** an owner who changes a Key Value's instance type is told first that the store restarts and is unavailable for a minute or two, and that a Persistence Off store loses every key. The dashboard stops promising "a rolling update that preserves data". **Status:** done 2026-11-04 (code, tests and copy shipped; the live production replay of the DoD follows the release that pins this build)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | Dashboard: honest Instance type copy, and the typed data-loss confirmation for a Persistence Off store | 40m | — |
| [t002](t002.md) | API, GraphQL, MCP and Blueprint: describe the restart and data loss on every plan-change entrypoint; record it in ADR021 | 25m | w4/m263/t001 |
| [t003](t003.md) | Postgres sibling: replace the same "operator … reconcile — rolling update" copy with what a single-instance resize does | 30m | — |
| [t004](t004.md) | Render parity | 15m | w4/m263/t002, w4/m263/t003 |
| [t005](t005.md) | Simplify | 10m | w4/m263/t004 |
| [t006](t006.md) | Test coverage | 30m | w4/m263/t004 |
| [t007](t007.md) | Closeout | 15m | w4/m263/t005, w4/m263/t006 |

Seven tasks, 165m (2h45m). One **major** finding. This milestone schedules the work only.

## Finding

- **Severity:** major. The product promises the opposite of what happens, and the cost is a customer's data.
- **Repro (live, 2026-11-04 10:17 UTC, pin `fb6098dd2e37`, `bex-canary`):** create a Free Key Value at `/keyvalue/new` (fixture `qa-20261104-kv`, `red-dblgcbfpehks73fta78g`, deleted in-run). On its detail page the Instance type card reads: _"Change the instance type. The operator resizes resources on the next reconcile — a rolling update that preserves data."_ The plan change itself was **not** submitted, because Free → Starter is a paid upgrade.
- **Expected (Render, [render.com/docs/key-value](https://render.com/docs/key-value), fetched 2026-11-04):** "Your instance will be unavailable for a minute or two during the upgrade." "Upgrading requires a restart. This can result in data loss based on your instance's persistence mode." "Free instances don't support data persistence, so their data is always lost on upgrade."
- **Actual, from source:**
  - Card copy: `dashboard/src/features/keyvalue/locales/en.ts:506-509` (`keyvalue.planDescription`).
  - Confirm dialog: `en.ts:523-526` (`keyvalue.planPickerConfirmBody`) says only "Connections drop briefly during the rolling restart". `key-value-plan-section.tsx:118-136` opens a plain `ConfirmDialog` with `destructive={false}` and no typed phrase, whatever the store's persistence mode.
- **Root cause (the restart is real):**
  - `keyvalue/service.go:942-973` `setPlan` patches `spec.plan` with no persistence check.
  - The operator puts the plan into the Valkey container's `Resources` (`lego/operator/internal/controller/keyvalue_controller.go:959`, `kvResources(intent.plan)`; also `keyvalue_persistence.go:83`). That is a StatefulSet pod-template change with no in-place resize policy, so the store's only pod is replaced.
  - With `persistenceMode: off` there is nothing on disk to reload, so every key is gone. w4/m233 measured the same mechanism on the allowlist path: `DBSIZE` 0 after a roll.
  - w4/m233 t008 already records that "a plan change … still reads `config_restart`", so the restart is known; only the warning is missing.
- **Precedent on the same page:**
  - The Persistence Mode card requires `sudo clear key value <name>` for a change to or from Off (`key-value-persistence-mode-section.tsx:131`).
  - The Maxmemory card says "Saving restarts the store, so connections drop for up to a minute".
  - The first external-access enable on an Off store is refused on REST/GraphQL/MCP without that phrase (`keyvalue/service.go:1104-1117`, w4/m233). Blueprints refuse it outright (`:1124-1130`, w4/358).
  - The plan change is the remaining restart that says nothing.
- **Blast radius (every plan-change entrypoint):**
  - REST `PATCH /v1/key-value/{id}` with `plan` (`keyvalue/rest.go:366`).
  - GraphQL `updateKeyValuePlan` (`keyvalue/graphql.go:231`).
  - MCP `update_key_value` with `plan` (`keyvalue/mcp.go:81`, description at `:173`).
  - Blueprint `plan` (`apps/blueprint_plan.go:594-595`), including the unattended auto-sync worker.
  - The dashboard plan section.
  - The Postgres card shares the copy (`dashboard/src/features/databases/locales/en.ts:1274`), but Postgres data is on disk, so only the "rolling" and jargon parts are wrong there (t003).
- **Adjacent classes:**
  - A plan change on a Journal + Snapshot or Snapshot only store keeps data up to its last write or snapshot, but still has the restart outage. It gets the restart copy, not the typed phrase.
  - A suspended store has no process to restart, so no prompt.
  - Changing to the same plan is not a change (the Save button is already disabled).
- **Parity-sufficient choice:** Render's API changes the plan without a confirmation, so REST/GraphQL/MCP keep accepting the plan change as they do now. They describe the consequence instead (t002). A stricter API refusal like m233's would break the pinned CLI's `keyvalues update --plan`, so it is a separate follow-up if wanted, not part of this milestone.
- **Unverified:**
  - The live data loss itself: it needs a paid plan change on an owned store.
  - The outage length on bex.
  - Whether Blueprint plan sync on an Off store should be refused (as w4/358 did for `ipAllowList`) or only described. t002 decides with Render's Blueprint behavior as the reference.
  - The confirm dialog text was read from source, not opened live.
- **Dedupe:**
  - `grep -ril "rolling update that preserves data\|planDescription" .pm` returns only skeleton evidence JSON (m226, m218, 298–303). No open or done item covers the claim or the warning.
  - `git log -S"rolling update that preserves data"` shows the copy is unchanged since w2/m16 (`1e91dc33b`).
  - No `.pm/DO_NOT_DO.md` entry applies.
  - ADR021 line 45 records the free-tier persistence divergence (persistence is offered on Free). That makes this warning matter more, not less: a Free store on bex can hold durable data that the copy implies is safe.

## Definition of done

Live on production after the release pins this build, on a fresh owned Free Key Value in `bex-canary`. Nothing paid is ever submitted.

- **Card copy.** The Instance type card contains no "operator", "reconcile" or "preserves data". It says the store restarts and is unavailable for about a minute or two, and that data survives according to the persistence mode (Off loses everything).
- **Off store asks for the phrase.** With Persistence Mode set to Off, selecting Starter and pressing Save opens a destructive confirmation whose text names the data loss, and whose confirm button stays disabled until `sudo clear key value <name>` is typed. Pressing Cancel sends no `updateKeyValuePlan` request: check the Network panel, where the count of GraphQL requests named `updateKeyValuePlan` is 0.
- **Persisted store gets the restart warning only.** With Journal + Snapshot, the same flow names the restart and outage, with no typed phrase. Cancel sends nothing.
- **API descriptions.** The MCP `update_key_value` description and the GraphQL `updateKeyValuePlan` description each state the restart and Persistence Off data loss. A REST `PATCH /v1/key-value/{id}` `{"plan": …}` with `dryRun` still answers 200, unchanged from Render.
- **Postgres card.** `/postgres/<id>` Instance type copy contains no "operator" or "reconcile", and says what a resize does on a single-instance database (t003's verdict).
- **Checks pass.** Dashboard `yarn lint` and `yarn test`. Backend `go test ./...` (real-DB gate) if t002 touches Go.
- **Cleanup.** No fixture remains, and only that run's session is revoked.

## Source + Goal linkage

- **Source:** live `/qa-find-bugs w4` loop, 2026-11-04 10:17 UTC (`muse.env`, workspace `bex-canary`, pin `fb6098dd2e37`, main `2f7106a24`). Evidence is this README's Finding section and the fixture IDs above; the screenshots under `.playwright-mcp/` are local only.
- **Goal linkage:** [ADR021](../../../docs/ADR021-keyvalue-management.md) (Key Value plans and persistence modes), [ADR008 § Render parity first](../../../docs/ADR008-vision.md#render-parity-first) and truthful product state. Render documents the restart and data loss on this exact control.
- **Expected outcome:** nobody empties a cache-turned-datastore by picking a bigger plan, and the page's three restarting controls (Instance type, Maxmemory, Persistence) warn in the same way.
- **Why now:** this is the last silent restart on the Key Value page after w4/m233 and w4/358. Upgrading is the most natural action on a store that is running out of memory, which is exactly when its data matters.
- **Render parity included:** the dashboard and the REST/GraphQL/MCP/Blueprint descriptions change. t004 compares Render's documented upgrade behavior and records any remaining divergence in ADR018.
