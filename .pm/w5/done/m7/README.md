# w5 · m7 — Service Settings + instance-type picker (Render parity)

**Worker:** worker5 **Goal:** mirror Render's instance-type management UX — a Settings tab showing the current instance type (name · CPU · RAM + Update link) and a dedicated plan-picker page (radio cards, current pre-selected, Save disabled until changed) — wired to the m8 plan API, so changing an App's size is a dashboard flow instead of a curl. **Status:** done (2026-07-08)

## Render reference (captured live 2026-07-08, srv-cr1aprdds78s739qrbg0)

- **Settings page** → "Instance Type" row: current tier name, separator, "0.5 CPU / 512 MB", **Update** link → `/web/srv-…/plan`.
- **Plan page**: heading "Pick an Instance Type"; a radiogroup of cards — **Free** ($0 · 512 MB RAM · 0.1 CPU) visually separated from the paid ladder (Starter $7 [pre-checked = current] · Standard $25 · Pro $85 · Pro Plus $175 · Pro Max $225 · Pro Ultra $450), each card showing name / $·month / RAM / CPU; footer "Need a custom instance type? … up to 512 GB RAM and 64 CPUs" + **Cancel** / **Save Changes** (disabled until the selection differs). Snapshots: `.playwright-mcp/render-plan-picker.yml`, `page-2026-07-08T04-59-28-316Z.yml`.
- **bex deltas, deliberate:** no prices on cards (pricing is Metronome's — the catalog carries none); no custom-instance mailto; the API surface already exists from w1/m8 (REST `PATCH serviceDetails.plan`, GraphQL `updateServicePlan`, MCP `update_service_plan`) — Render has **no** public list-instance-types REST/MCP API (their dashboard hardcodes the picker), so the catalog read ships as a GraphQL query only, a bex extension consistent with the dashboard-contract vocabulary.

## Tasks (in order)

| id   | title                                                                                          | est | depends_on             |
| ---- | ----------------------------------------------------------------------------------------------- | --- | ---------------------- |
| t001 | GraphQL `instanceTypes` catalog query (id, name, cpu, memory) from lego/types/tiers — **DONE**  | 30m | —                      |
| t002 | Settings tab: route + nav item + Instance Type summary row with Update link — **DONE**         | 40m | t001                   |
| t003 | Plan-picker page: radio cards, current pre-selected, confirm + updateServicePlan, toasts — **DONE** | 45m | t001                   |
| t004 | Live verify against prod via dev tunnels (flip a plan end-to-end) + screenshot — **DONE (mock cluster full flow; prod graceful-degradation only, real flip deferred to post-/ship)** | 25m | t002, t003             |
| t005 | Simplify — run `/simplify` over the code this milestone changed — **DONE**                     | 20m | t004                   |
| t006 | Test coverage — catalog query, picker states (pre-select/disable/confirm), mutation wiring — **DONE** | 30m | t004                   |

## Definition of done

The service nav gains a Settings tab whose Instance Type row shows the App's current plan as Render does (name · CPU · RAM, "Update" linking to the picker); the picker page lists every catalog tier as radio cards (no prices), pre-selects the current plan, keeps Save disabled until the selection changes, confirms with rollout warning, calls `updateServicePlan`, and lands back on Settings showing the new type; an untiered (bare-CR, plan-less) App shows an honest "no instance type set" state instead of a fake selection; the tier list comes from a new GraphQL `instanceTypes` query backed by `lego/types/tiers` (no hardcoded ladder copy in the dashboard — the fourth-copy trap); verified live against prod by flipping a real App's plan from the UI and watching the pod resize; dashboard tests + lint + build green.

## Source + Goal linkage

- **Source:** user request 2026-07-08 — "learn from dashboard.render.com …/settings about update instance types; mirror including UI and GQL API, and MCP/REST if such APIs exist"; element inventory captured live (see Render reference above). Builds directly on w1/m8 (catalog + plan API, shipped 2026-07-08) — this is the UI half m8 explicitly left out of scope.
- **Goal linkage:** the standing render.com feature-parity goal; completes the tier story user-visibly (m8 made plans changeable by API; today the only prod plan changes were done by an agent with curl — a dashboard flow is the product).
- **Expected outcome:** an operator resizes an App from the dashboard in two clicks; the metrics page's Limit labels/percentages update after the roll; no more agent-mediated PATCHes for routine sizing.
- **Why now:** m8's API is freshly deployed and already exercised on prod (beancount-cms and eden-cms-v2 moved to `standard` via raw GraphQL 2026-07-08) — the UI is the missing last mile, and the picker needs the catalog query before any other consumer invents a hardcoded tier list in the frontend.

## Out of scope

- Prices anywhere in the UI (Metronome's, per the m8 decision) and the custom-instance-type contact flow.
- REST/MCP list-instance-types endpoints (Render has none; add only if a non-dashboard consumer appears).
- The rest of Render's Settings page (name/region/build/deploy sections) — this milestone ships the Settings *tab* with the Instance Type section only; other sections are future milestones (e.g. the existing env-vars tab stays where it is).
- Store adoption of bare-CR apps (separate candidate milestone, w1).

## Completion notes (2026-07-08)

- **Catalog query** (t001): `Service.InstanceTypes(ctx)` (`core.RelCanView`-guarded, auto-swept by `TestAuthzGuardsEveryVerb`) projects `tiers.Compute` in ladder order onto `{id, name, cpu, memory}` — `id` is the Render plan spelling `SetPlan` already accepts, `name` derives from the internal hyphenated id via `tierDisplayName`. GraphQL-only, as scoped (no REST/MCP consumer exists yet); the fragment comment records why.
- **Settings tab** (t002): new route + nav item; `InstanceTypeRow` resolves the service's `plan` against the catalog (via `useInstanceTypes().byID`) and renders name/CPU/RAM, a raw-id fallback if the catalog no longer recognizes a plan, or an honest "No instance type set" state for an untiered App — never a fake selection.
- **Plan picker** (t003): `InstanceTypePicker` renders every tier as a radio card (Free separated from the paid ladder, no prices), pre-selects the current plan, disables Save until the selection changes, confirms via the repo's existing `AlertDialog` idiom, fires `updateServicePlan`, toasts, and navigates back to Settings.
- **Live verify** (t004): full flow (Settings → Update → pick → confirm → toast → Settings showing the new tier) verified end-to-end against the **local mock cluster**, which carries this milestone's backend. Also ran the dev-server-tunnel pattern against **prod** (same recipe as w1/m8's verify) to confirm graceful degradation ahead of shipping: the Settings row falls back to the raw plan string and the picker shows its "Couldn't load instance types" error card, since prod's currently-deployed bex-api predates this milestone. A real prod plan flip through this UI is deferred to post-`/ship`, the same constraint recorded for w1/m8's t004.
- **Simplify** (t005, 2 parallel review agents): one confirmed fix applied — `dashboard/src/routes/services.$serviceId.plan.tsx` mounted `InstanceTypePicker` before `useServer`'s query resolved, so `currentPlan` was `null` on first render and the picker's `useState` seed froze there, silently breaking the "current plan pre-selected" behavior on the real (non-cached) navigation path; now gated on `loading` the same way the Settings route already gates `InstanceTypeRow`. Also removed `TestInstanceTypesRequiresAuth`/`stubChecker` from `apps_test.go` — redundant with `TestAuthzGuardsEveryVerb`'s automatic reflection sweep, and no other verb in the file carries a bespoke per-verb auth test. Both review passes found the reuse patterns (formatters, confirm-dialog, hook shapes) and layering (backend keeps raw quantities, frontend does display-only unit formatting) already correct — no further changes.
- **Test coverage** (t006): `formatInstanceCPU`/`formatInstanceMemory` unit tests; `useInstanceTypes` (mapping, null-filtering, loading, `byID` lookup, query-error passthrough) and `useUpdatePlan` (success/error toast, busy flag lifecycle) hook tests; `InstanceTypeRow` (resolved/raw-fallback/no-plan states, Update link href) and `InstanceTypePicker` (pre-select + Save-disabled, enabling on a new pick, confirm→mutation→navigate, mutation failure not navigating, catalog-error card) component tests — all via this repo's established mocked-hook/real-router conventions. 60 new/updated dashboard tests, 353 total, plus the existing Go backend suite, all green; `yarn typecheck`, `yarn lint`, and `go build ./... && go vet ./... && go test ./...` all clean.
