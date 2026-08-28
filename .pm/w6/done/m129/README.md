# w6 · m129 — A resource whose deletion has not finalized is hidden from the list but still counted against the plan quota: "Services 11 / 100" over 6 visible services

**Worker:** worker6 **Goal:** the usage number and the resource list tell one story, so a tenant blocked by a cap can find what is consuming it **Status:** done

## Tasks (in order)

| id   | title                                                                                | est | depends_on |            |
| ---- | -------------------------------------------------------------------------------------- | --- | ---------- | ---------- |
| t001 | Decide and implement how the list and the quota counter are reconciled                   | 60m | t003       | — **DONE** |
| t002 | Apply the outcome to all three counters, not just services                               | 35m | t001       | — **DONE** |
| t003 | Verify the enforcement claim: does the ResourceQuota count terminating objects?           | 30m | —          | — **DONE** |
| t004 | Render parity                                                                             | 20m | t001, t002 | — **DONE** |
| t005 | Simplify                                                                                  | 20m | t004       | — **DONE** |
| t006 | Test coverage                                                                             | 40m | t004       | — **DONE** |
| t007 | Closeout                                                                                  | 15m | t005, t006 | — **DONE** |

## Definition of done

- **The two numbers are reconcilable.** Either the list (or an adjacent surface) accounts for every resource the counter includes, or the counter counts only what the list shows **and** creates are gated on that same number. Today GraphQL `workspaceLimits` reports `services.used = 11` while `GET /v1/services?limit=100` returns **6**, with no surface bridging them.
- **A completed delete returns the quota.** Read `workspaceLimits` before a create, after it, and after the delete finishes teardown — the figure returns to its pre-create value.
- **A resource stuck mid-teardown is discoverable**, or the product states plainly that quota is held until teardown completes. `w3/m46/t002`'s comment currently makes the opposite trade explicitly ("the operator's own alerts/audit surface it, not the tenant list"), so this is a decision to revisit, not an oversight to patch.
- **Postgres and Key Value behave the same way as services** under the chosen rule — exercised with a datastore actually mid-delete, not merely read from code.
- **A cap-hit create still returns the existing named ResourceQuota error** from `mapServiceCapError` (`apps/service.go:2050-2061`), unchanged.

## Resolution (shipped)

**Shape (a) — make the held quota discoverable; the counter stays truthful.** `ResourceLimits` now reports a `Terminating` count beside `Used` for each of services/Postgres/Key Value. `Used` still counts every CR (== what enforcement gates on); `Terminating` is the subset finishing deletion, which the resource list drops. The two reconcile by construction: **the list shows `Used - Terminating`**. The naive filter (shape b) was rejected in code comments and here — filtering terminating CRs out of `Used` would make the number smaller than enforcement and refuse a create with no explanation; k8s cannot be told to discount an object it still holds in etcd, so the enforcement side can't be made to match a filtered display.

- **Backend:** `ResourceCapView.Terminating` + a single generic `usageCount` helper (one rule for all three kinds); GraphQL/REST/MCP all carry the field (REST + MCP serialize the struct, so they stayed consistent for free). `apps.Service.List`'s w3/m46 comment updated so it no longer contradicts the shipped behaviour — it now records that the hidden row is still accounted for under `terminating`.
- **Dashboard:** the resource-cap tile shows "N finishing deletion" with an explanatory tooltip when `terminating > 0`, so 11/100 reads as 6 active + 5 finishing deletion. This is where journey 16's promise ("disappears from every … usage view") is met without reversing w3/m46's deliberate list hide.
- **Tests:** backend fixtures force a real `DeletionTimestamp` (finalizer-held) on an App/Database/KeyValue and pin `Used - Terminating == what the list shows`, plus the Hobby-cap-of-1 boundary; dashboard tests pin the reconciliation sub-line and the `terminating`-defaults-to-0 mapping. The cap-hit error path is untouched and its existing test still passes.

**t003 — settled.** Enforcement is a Kubernetes **object-count** ResourceQuota (`count/apps.app.bex.co` / `count/databases.app.bex.co` / `count/keyvalues.app.bex.co`, `store/namespaces.go:501-503`). Object-count quotas count every object present in the namespace, and a terminating CR held by a finalizer still exists in etcd (it is returned by LIST/GET) until the finalizer clears — so it genuinely consumes quota. This is deterministic, documented k8s behaviour, and the fake-client test demonstrates the mechanism directly: a finalizer-held, delete-requested CR still LISTs and is counted. **Not captured this session:** a live `kubectl get resourcequota -o yaml` `status.used` reading while an object terminates — the local mock cluster was not running and envtest does not run the quota controller, so no live figure was taken. The conclusion rests on the quota-key type + documented semantics, not on a live capture; a live `status.used` reading remains the one open confirmation.

**DoD live checks (production) — not run this session.** No cluster/QA access this session, so the production `workspaceLimits`-vs-`/v1/services` reconcile and the create→delete→re-read cycle were not executed live. The counter/list identity they would confirm is pinned by the tests above; the live production capture remains the outstanding manual verification.

## Source + Goal linkage

- **Source:** live `/qa-find-bugs` hunt of `dashboard.bex.co`, **66th run**, 2026-08-28, journey 16 ("it disappears from every list, project page, **and usage view**"). Workspace `tea-d98210cbbpdc73dcrkvg` (Pro). **Read-only** — nothing was created or deleted. Probes: `GET /v1/services?limit=100`, the GraphQL query below, and the Billing page at `https://dashboard.bex.co/billing` (note `/usage` redirects there).

  The Billing page's "Included usage" card renders:

  ```
  Services    11 / 100
  Postgres     3 / 25
  Key Value    2 / 25
  ```

  Confirmed from the source, GraphQL `workspaceLimits`:

  ```json
  { "services": {"used": 11, "limit": 100},
    "postgres": {"used": 3,  "limit": 25},
    "keyValues":{"used": 2,  "limit": 25} }
  ```

  But `GET /v1/services?limit=100` returns **six**: `agentmarketcap-1` [Running], `beancount-cms-v2` [Running], `beancount-forum` [Running], `eden-dash-v3` [Running], `eden-cms-v2` [Running], `qa-20260826-webhook-renamed` [Failed].

  **Postgres (3) and Key Value (2) match reality exactly** — which is what makes services' 11-vs-6 a real divergence rather than a misread of the page.

- **Root cause — the same App CRs, counted two ways.** The counter, `lego/backend/internal/workspaces/service.go:941-955`:

  ```go
  var apps appv1alpha1.AppList
  if listErr := s.ListByTenant(ctx, &apps, tenantID); listErr != nil { ... }
  ...
  out.Services.Used  = len(apps.Items)
  out.Postgres.Used  = len(dbs.Items)
  out.KeyValues.Used = len(kvs.Items)
  ```

  No deletion filter on any of the three. The list, `lego/backend/internal/apps/service.go:1131`, drops them **deliberately**:

  ```go
  // A deleting App is dropped from the list the moment its deletion is
  // requested (w3/m46) ... Trade-off: a delete stuck on a failing finalizer
  // becomes invisible here (the operator's own alerts/audit surface it, not
  // the tenant list) ...
  if !list.Items[i].DeletionTimestamp.IsZero() {
      continue
  }
  ```

  Same objects, opposite treatment, and no surface reconciling the two. The datastore counters (`:954-955`) carry the identical missing filter — they agree with reality today only because no Postgres or Key Value happens to be mid-delete. **Correct by luck, not by construction.**

- **The counter is not lying about the quota — and this is what decides the fix.** Enforcement does **not** run through this GraphQL counter. `lego/backend/internal/store/namespaces.go:487` builds a per-namespace Kubernetes **ResourceQuota** from the same `QuotaCapsForPlan`, and `lego/backend/internal/apps/service.go:2050-2061` (`mapServiceCapError`) translates that ResourceQuota's rejection into the user-facing cap error. A Kubernetes ResourceQuota counts objects until they are actually removed, so a still-terminating App genuinely consumes quota. The displayed `11` is an **honest** report of what enforcement will do; the defect is that the list gives the user no way to see or act on the five consuming it.

  **That rules out the naive fix.** Filtering deleting Apps out of the counter would make the number *look* right and make it a lie — the user would then be refused a create at `6/100` with no explanation at all.

- **Stakes by plan** — `lego/backend/internal/store/plans.go:104-110`:

  ```go
  case PlanHobby, "", "free":
      return QuotaCaps{Services: int64(LimitsFor(PlanHobby).MaxServices), Postgres: 1, KeyValues: 1}
  default: // pro, scale, enterprise
      return QuotaCaps{Services: 100, Postgres: 25, KeyValues: 25}
  ```

  On this Pro workspace, five phantom services out of 100 is invisible. On **Hobby**, one terminating Postgres consumes the **entire** Postgres quota, and the user sees an empty database list beside a refusal to create.

- **Goal linkage:** [docs/ADR030-pricing.md](../../../docs/ADR030-pricing.md) and [docs/ADR040-billing-metronome.md](../../../docs/ADR040-billing-metronome.md). `w7/m9` shipped this counter ("resource limits surface + dashboard cap-hit UI", `2e324cb8`) and is the precedent to extend — it was not wrong to count CRs; it simply never met the case where the list hides some of them.

- **Expected outcome:** the usage number and the resource list tell one story, and a tenant blocked by a cap can find what is consuming it.

- **Why now:** journey 16's promise is that a deleted resource "disappears from every list, project page, and usage view". It disappears from the list and stays in the usage view — and because the usage view mirrors **real enforcement**, the disappearance from the list is the half that misleads. It compounds `w3/m46/t009`, where a static site has now been terminating for **~17.5 hours**: every hour of that is an hour of quota held invisibly.

- **Precedent — extend, do not re-litigate.** `w3/m46/t002` deliberately hid deleting Apps from the list and recorded the trade-off in its own code comment; `w3/m46/t009` tracks the stuck teardown that makes the window long. **Neither covers the usage counter** — m46's definition of done is four static-site inconsistencies (suspended cert, list row, `clearCache`, SPA fallback) and mentions no quota, cap or usage surface. This milestone is the consequence neither owns.

- **Render parity:** included (t004). `workspaceLimits` is a **bex extension** — Render exposes no equivalent resource-cap query — so the parity question is narrow: confirm nothing in REST/MCP reports a conflicting count, and record the bex-only status rather than inventing a Render row.

- **Blast radius:** `ResourceLimits` (`workspaces/service.go:925-957`) has **1** caller — the GraphQL `workspaceLimits` field (`workspaces/graphql.go:76-84`) — and one dashboard consumer, `use-resource-limits.ts` feeding `resource-caps.tsx`. `QuotaCapsForPlan` has **2** callers: this counter and `store/namespaces.go:487`'s ResourceQuota builder, which is the enforcement side and must not drift from whatever the display does.

- **Adjacent classes:** a resource deleted and **fully finalized** (must leave both surfaces); one **mid-teardown** (the case at issue); one **suspended** rather than deleting (still exists, still counts — correctly); and a workspace **exactly at** its cap, which must keep receiving the existing named error.

- **Unverified this run — carried as work, not presented as observation:** the composition of the 11-vs-6 gap. Exactly **one** terminating App is known (`qa-20260827-static`, `srv-da7tf87krsvc73c3mcng`, still `phase: Deleting` with `updatedAt` frozen at `2026-08-27T06:26:24Z`), which demonstrates the mechanism; the other four are consistent with earlier hunts' deleted fixtures leaving CRs behind but were **not** confirmed — no cluster access this run, so no App CR was listed directly. Also unverified: that the Kubernetes ResourceQuota counts terminating objects (**t003** owns it), and whether the dashboard shows a cap-hit warning at 11/100 (the near-limit threshold is 0.8, so nothing triggered here).
