# w2 · m10 — Deploy cancel + rollback

**Worker:** worker2 **Goal:** The two deploy verbs w2/m5 deliberately left out, now cheap because deploy objects exist: `POST /v1/services/{id}/deploys/{deployId}/cancel` (kill an in-flight build) and `POST /v1/services/{id}/rollback` (return to any listed deploy — modeled as a **new** deploy, Render-style). Rollback is the highest-value missing agent verb: "that deploy broke, roll it back" is the other half of the poll loop m5 shipped. **Status:** todo

## Tasks (in order)

| id   | title                                                                                        | est | depends_on |
| ---- | ---------------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Deploys rows record a rollback target: image ref/digest + restorable spec fields (migration)   | 30m | —          |
| t002 | `Cancel` verb: kill the in-flight build Job, deploy → `canceled`; 409 past cancelable states   | 30m | t001       |
| t003 | `Rollback` verb: creates a new deploy applying the recorded target (never history rewrite)     | 30m | t001       |
| t004 | Surfaces: REST cancel + rollback (shapes vs Render OpenAPI) · GraphQL · MCP (bex extensions)   | 30m | t002, t003 |
| t005 | Acceptance: bad-image deploy → rollback → previous digest live; mid-build cancel → Job gone    | 25m | t004       |
| t006 | Render parity — cross-surface consistency + OpenAPI comparison; refresh the matrix deploy rows | 20m | t005       |
| t007 | Simplify — `/simplify` over the code this milestone changed                                    | 20m | t006       |
| t008 | Test coverage — state-machine, non-cancelable 409s, rollback-target integrity                  | 30m | t006       |
| t009 | Closeout — DoD met → move milestone to `done/`                                                 | 10m | t008       |

## Definition of done

An agent can cancel a building deploy (build Job terminated, deploy record reads `canceled`, service unaffected) and roll a service back to any listed deploy through REST, GraphQL, and MCP; the rollback is itself a new deploy record that converges to `live` serving the recorded image; canceling past the cancelable window returns Render's 4xx semantics, never a partial state; docs/ADR018-render-parity.md's Cancel/Rollback rows move from ✖.

## Source + Goal linkage

- **Source:** `/pm-brainstorm more` 2026-07-12; the two remaining ✖ rows in docs/ADR018-render-parity.md §Deploys (Cancel deploy, Rollback — both previously pointed at w2/m5, which shipped list/get/trigger without them); Render OpenAPI deploy endpoints; `lego/backend/internal/deploys/` (m5's package, the extension point).
- **Goal linkage:** pillar 1 (Render deploy-surface parity) + pillars 3/4 — an agent that can watch a deploy converge must also be able to abort or undo it.
- **Expected outcome:** the Deploys matrix section is fully ✅/—; "roll back the bad deploy" becomes one MCP call.
- **Why now:** the deploys schema is fresh (m5, 3 days old) — extending it with a rollback target is a cheap migration now and a backfill problem later; every deploy recorded without target data is un-rollback-able forever.
- **Render parity closing task: included** — REST/GraphQL/MCP surface change. Dashboard deploy list (with Rollback/Cancel buttons) is deliberately out — no Deploys tab exists yet; filed as `w5/007` alongside the w3/m7 Events tab.
