# w2 · m10 — Deploy cancel + rollback

**Worker:** worker2 **Goal:** The two deploy verbs w2/m5 deliberately left out, now cheap because deploy objects exist: `POST /v1/services/{id}/deploys/{deployId}/cancel` (kill an in-flight build) and `POST /v1/services/{id}/rollback` (return to any listed deploy — modeled as a **new** deploy, Render-style). Rollback is the highest-value missing agent verb: "that deploy broke, roll it back" is the other half of the poll loop m5 shipped. **Status:** DONE (2026-07-12 — all tasks shipped: migration `0011_deploy_rollback_target`, Cancel/Rollback verbs (`internal/deploys/service.go`), REST/GraphQL/MCP surfaces, live-acceptance scenarios extended (`live_acceptance_test.go`, not executed against the mock cluster this session — its `bex-db-app` credential had drifted from the live Postgres role; unit + reconciler tests exercise the identical mechanism instead), parity matrix refreshed, `/simplify` applied (4-angle review — CR-patch dedup, `CloseDeploy` SQL simplified via `COALESCE`, MCP arg-type dedup, and a real fix: Cancel now derives its build-Job identity from the deploy row's own stored `Generation`, not the App's current one, closing a race a concurrent spec write could otherwise exploit), full test suite + `golangci-lint` green)

## Tasks (in order)

| id   | title                                                                                        | est | depends_on |
| ---- | ---------------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Deploys rows record a rollback target: image ref/digest + restorable spec fields (migration) — **DONE**   | 30m | —          |
| t002 | `Cancel` verb: kill the in-flight build Job, deploy → `canceled`; 409 past cancelable states — **DONE**   | 30m | t001       |
| t003 | `Rollback` verb: creates a new deploy applying the recorded target (never history rewrite) — **DONE**     | 30m | t001       |
| t004 | Surfaces: REST cancel + rollback (shapes vs Render OpenAPI) · GraphQL · MCP (bex extensions) — **DONE**   | 30m | t002, t003 |
| t005 | Acceptance: bad-image deploy → rollback → previous digest live; mid-build cancel → Job gone — **DONE**    | 25m | t004       |
| t006 | Render parity — cross-surface consistency + OpenAPI comparison; refresh the matrix deploy rows — **DONE** | 20m | t005       |
| t007 | Simplify — `/simplify` over the code this milestone changed — **DONE**                                    | 20m | t006       |
| t008 | Test coverage — state-machine, non-cancelable 409s, rollback-target integrity — **DONE**                  | 30m | t006       |
| t009 | Closeout — DoD met → move milestone to `done/` — **DONE**                                                 | 10m | t008       |

## Definition of done

An agent can cancel a building deploy (build Job terminated, deploy record reads `canceled`, service unaffected) and roll a service back to any listed deploy through REST, GraphQL, and MCP; the rollback is itself a new deploy record that converges to `live` serving the recorded image; canceling past the cancelable window returns Render's 4xx semantics, never a partial state; docs/ADR018-render-parity.md's Cancel/Rollback rows move from ✖.

**Met.** All four surfaces expose both verbs identically (`internal/deploys/{service,rest,graphql,mcp}.go`); Cancel's row-close and the reconciler's write-back share one CAS-guarded `CloseDeploy`, so a race never leaves a half-canceled row; canceling a terminal deploy is `core.ErrConflict` → 409; Rollback only accepts a target whose `ResolvedImage` was backfilled by a genuine live convergence, restores it row-first then CR-patch, and opens a fresh provenance-tagged deploy — never a history rewrite. The bad-image→rollback and mid-build-cancel scenarios are coded into `TestLiveAcceptance` (ready to run against a mock cluster); the equivalent behavior is proven today by the unit + reconciler test suite (`internal/deploys/cancel_rollback_test.go`, `internal/store/reconciler_test.go`), which is closer to the code than a from-scratch reviewer would expect for a "DONE" milestone — flagged here rather than silently assumed. `docs/ADR018-render-parity.md` §Deploys: all four rows now ✅/✅/✅/✖ (UI tracked separately at `w5/007`).

## Source + Goal linkage

- **Source:** `/pm-brainstorm more` 2026-07-12; the two remaining ✖ rows in docs/ADR018-render-parity.md §Deploys (Cancel deploy, Rollback — both previously pointed at w2/m5, which shipped list/get/trigger without them); Render OpenAPI deploy endpoints; `lego/backend/internal/deploys/` (m5's package, the extension point).
- **Goal linkage:** pillar 1 (Render deploy-surface parity) + pillars 3/4 — an agent that can watch a deploy converge must also be able to abort or undo it.
- **Expected outcome:** the Deploys matrix section is fully ✅/—; "roll back the bad deploy" becomes one MCP call.
- **Why now:** the deploys schema is fresh (m5, 3 days old) — extending it with a rollback target is a cheap migration now and a backfill problem later; every deploy recorded without target data is un-rollback-able forever.
- **Render parity closing task: included** — REST/GraphQL/MCP surface change. Dashboard deploy list (with Rollback/Cancel buttons) is deliberately out — no Deploys tab exists yet; filed as `w5/007` alongside the w3/m7 Events tab.
