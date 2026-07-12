# w1 · m8 — Instance tiers: one catalog, Render-shaped plan API, limits everywhere

**Worker:** worker1 **Goal:** make the tier ladder a single reviewed artifact (`lego/types/tiers/tiers.yaml`, Render-consistent names/sizes), expose Render's `plan` on the public API mapped onto `spec.tier`, and get every real App onto a tier — so pods carry requests==limits and the metrics page's "No limit configured — percentage is undefined" state disappears for anything a user runs. **Status:** done (2026-07-08)

## What already exists (verified 2026-07-08 — don't rebuild)

- The ladder itself, three times over: operator `tierResources` (`app_controller.go:158`), store `var tiers` + `normalizeTier` (`store/api.go:322-341`), and the docs table (`docs/ADR003-control-plane.md` §Tiers). All three already mirror Render's published ladder — this milestone **collapses copies**, it does not invent sizes.
- Store-managed Apps already default: `normalizeTier("") → "free"`, and the projector already maps the row's `Tier` onto `spec.tier` (`store/reconciler.go:184,211`). The gap is **legacy bare-CR apps** (e.g. prod `beancount-cms`, created before the control plane) and the **public Render surface**, which has no `plan` field at all.
- **Prices are Metronome's, not ours** — recorded in `store/api.go:327` and `migrate.go:19`. The catalog therefore carries **no money**: id, Render spelling, cpu, memory. (Display pricing for a future dashboard plans page comes from wherever billing truth lands, later and deliberately.)
- The row-first single-writer seam for intent changes exists: `apps.IntentStore` (one method, `SetAppSuspended`) — a plan change adds a sibling method, same pattern.

## Tasks (in order)

| id   | title                                                                                                      | est | depends_on |
| ---- | ------------------------------------------------------------------------------------------------------------ | --- | ---------- |
| t001 | Tier catalog: `lego/types/tiers` package — embedded YAML + loader + validation (no prices) — **DONE**       | 40m | —          |
| t002 | Collapse the two code copies onto it: operator `tierResources`, store `tiers`/`normalizeTier` — **DONE**    | 30m | t001       |
| t003 | Public API: `plan` on the service view + a plan-change verb (row-first), unknown plan ⇒ 400 — **DONE**      | 45m | t001       |
| t004 | Tier the legacy bare-CR apps + end-to-end verify (limits on pods, percentage metrics live) + docs — **DONE (mock cluster; prod deferred to post-/ship)** | 30m | t002, t003 |
| t005 | Simplify — run `/simplify` over the code this milestone changed — **DONE**                                   | 20m | t004       |
| t006 | Test coverage — catalog validation, plan⇄tier round-trip, row-first plan change, 400 paths — **DONE**       | 30m | t004       |

## Definition of done

`lego/types/tiers/tiers.yaml` is the only place the ladder exists in code (operator map and store list deleted, both reading the embedded catalog; the docs table becomes a pointer); the public REST/GraphQL report Render's `plan` on every service and accept a plan change per Render's `serviceDetails.plan` spelling — row-first for store-managed apps, CR-patch for bare-CR apps — rolling the pods onto the catalog's requests==limits; unknown plans are rejected 400 listing valid names while bare-CR `spec.tier` stays an open string; prod's `beancount-cms` (and any other untiered app) carries a tier, verified live: its metrics-page Percentage tab shows 0–100 charts with a "Limit …" label instead of "No limit configured"; the catalog file contains no pricing; `make test` + backend tests green, with a CI-run test that fails on a malformed catalog.

## Source + Goal linkage

- **Source:** architecture discussion 2026-07-08 (this session) — triggered by the shipped w3/m4.5 metrics page showing "No limit configured — percentage is undefined" for `beancount-cms`; ladder verified against Render's live docs (render.com/docs/compute-plans: Free 0.1/512MB · Starter 0.5/512MB · Standard 1/2GB · Pro 2/4GB · Pro Plus 4/8GB · Pro Max 4/16GB · Pro Ultra 8/32GB), which all three existing copies already mirror.
- **Goal linkage:** the standing render.com feature-parity goal (Render services always expose a `plan`; bex's public surface exposes none) and `GOAL.md` #2 — percentage metrics are only honest when limits exist. Extends w1/m2's control plane along its existing seams (row `Tier`, projector, `IntentStore`).
- **Expected outcome:** one reviewed YAML in the contract module feeding operator + store + API; `PATCH` a service's plan and the pod resizes; prod metrics Percentage tab works for every user-facing app.
- **Why now:** w3/m4.5 just made the missing-limits gap user-visible on prod; the w1/m2 seams this hangs off are fresh; and the ladder already exists in three places that can only drift further apart.

## Out of scope (this milestone)

- Pricing anywhere in bex code (Metronome's, per the recorded decision) and the dashboard plans/scaling page — follow-ups once the catalog exists.
- A public `plans` catalog endpoint; app **create** on the public surface (doesn't exist yet — creation is the internal tenant API, already tier-validated, or bare CR).
- Free-tier idle semantics (w1/m4 scale-to-zero); runtime-editable ladder (ConfigMap loader) — the embedded file is deliberately build-time.

## Completion notes (2026-07-08)

- **Catalog** (`lego/types/tiers/{tiers.yaml,tiers.go}`): 7 tiers embedded via `go:embed`, parsed once at package init (`mustLoad`), validated (unique ids/renderPlans, parseable quantities, default present) — 14 tests in `tiers_test.go` including every malformed-catalog case. Zero new module dependencies (`k8s.io/apimachinery` was already a direct dep of `lego/types`). `RenderPlans()` sits alongside `IDs()` as the API-facing sibling list.
- **Collapse** (t002): the operator's `tierResources` map and the store's `tiers`/`normalizeTier` list are both gone; `resourcesForTier` and `normalizeTier` now read the one catalog. `grep -rn "pro-ultra"` outside the catalog/tests returns nothing.
- **Plan verb** (t003): `AppView.Plan` (omitted for untiered apps), `Service.SetPlan` (row-first via a new `IntentStore.SetAppTier`, CR-patch for bare-CR apps, `core.ErrBadRequest` on unknown plans), fanned out to REST (`PATCH /v1/services/{id}` on `serviceDetails.plan`), GraphQL (`updateServicePlan` mutation + `plan` field — marked as an unconfirmed-live-capture bex extension, following the suspend/resume/restart naming convention), and MCP (`update_service_plan` tool). `TestAuthzGuardsEveryVerb` and `TestSurfaceParityAndWiring` picked up the new verb automatically.
- **Legacy apps + live verify** (t004): verified end-to-end on the **local mock cluster** (fresh operator + bex-api binaries, a deliberately untiered `legacy-app` bare CR) — REST PATCH resized the pod to `standard` (1 CPU/2Gi, requests==limits), the GraphQL mutation resized it again to `pro` (2 CPU/4Gi), an unknown plan was rejected 400 with the valid-plan list and left the CR untouched, and `CPU_LIMIT`/`MEMORY_LIMIT` metrics picked up the new limits with zero metrics-code changes — confirming the w3/m4.5 "No limit configured" state is gone once a tier is set. **Prod's `beancount-cms` was not tiered** — prod's currently-deployed `bex-api` predates this milestone and has no `plan` verb to call; that step needs a `/ship` first, then a live PATCH against `api.bex.co`. Docs (`docs/ADR003-control-plane.md`, `docs/ADR010-observability.md`) now point at the one catalog file instead of repeating the ladder.
- **Simplify** (t005, 2 review agents): applied — extracted the shared `writeThroughStore` helper (`setSuspended`/`SetPlan` were near-identical row-first-then-patch bodies), added `tiers.RenderPlans()` so `apps/service.go` stopped re-deriving it via a second `ByID` lookup per entry, and had the REST PATCH handler's no-op branch call the existing `get` closure instead of duplicating it. Declined: splitting `RenderPlan` into a separate package from `{ID,CPU,Memory}` to keep it out of the operator's transitive dependency — reverses the deliberate one-file decision from the original design (avoiding two-file drift is worth more than saving the operator an unused struct field it never reads).
- **Test coverage** (t006): confirmed no gaps — every behavior in the milestone's definition of done has direct or end-to-end coverage (catalog validation, plan⇄tier round-trip position-checked against `IDs()`, row-first vs bare-CR, view omission, 400 + valid-plan-list, authz sweep, empty→default and unknown→400 through the real HTTP path in `store/api_test.go`).
- **Follow-up (same day, user review):** the catalog was restructured from one undifferentiated ladder into **named families** — `compute:` (App instance types) and `postgres:` (Database instance types) — exposed as `tiers.Compute` / `tiers.Postgres`. This also collapsed a **third** hardcoded ladder discovered during the review: the operator's `dbPlans` map in `database_controller.go` (`basic-256mb`, `basic-1gb`, Render's family-size Postgres naming) now reads `tiers.Postgres`, and `resolvePlan`/`cnpgClusterSpec` consume `tiers.PostgresTier` directly.
- All three modules (`types`, `backend`, `operator`) build, vet, and test clean; `make test` (operator envtest suite) and both `golangci-lint` runs show zero new findings (one pre-existing, unrelated `authz_test.go` finding untouched by this work).
