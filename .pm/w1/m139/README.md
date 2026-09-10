# w1 · m139 — Live-verification debt sweep on dev-1

**Worker:** worker1 **Goal:** ten residuals that closed milestones recorded as "not verified live — no cluster access, deferred to the next QA pass" (and that nothing on the board owns) each get a dated pass/fail evidence line from a live run on the `dev-1` stack, with every failure fixed in-task when small or filed as an inbox note with `file:line`. **Status:** todo

## Tasks (in order)

| id   | title                                                                                                                                                                  | est | depends_on                         |
| ---- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------- | --- | ---------------------------------- |
| t001 | Bring up `dev-1` beside the live `dev-3` stack; record the two-workstream isolation walk `w1/m72/t007` waived                                                          | 30m | —                                  |
| t002 | Scaling matrix: `background_worker`, `private_service`, `static_site`, `cron_job` follow the same free/paid replica policy the `web_service` m118 scaled live          | 45m | t001                               |
| t003 | Free-tier sleep and wake: create → idle past the window → hibernate → wake on request (m116); note how wake-up requests are labelled in Loki                            | 45m | t001                               |
| t004 | Datastore plan parity: GraphQL `createKeyValue` free-plan forcing (m127), GraphQL/MCP Blueprint plan exposure + live plan `update` (m125), Key Value parameter capture (m133) | 45m | t001                               |
| t005 | Blueprint round-trip: export for disk/cron/static/health-check kinds + `sync: false` on apply (m114); MCP + `render.yaml` create legs and the first-deploy Canceled probe (m130 / m46 residue) | 60m | t001                               |
| t006 | Small live checks: cron `99 99 * * *` → 400 with the App still healthy (w9/m91); a pre-fix `private_service` loses its Ingress on reconcile (m46)                      | 30m | t001                               |
| t007 | Render parity                                                                                                                                                          | 30m | t002, t003, t004, t005, t006       |
| t008 | Simplify                                                                                                                                                               | 20m | t007                               |
| t009 | Test coverage                                                                                                                                                          | 30m | t007                               |
| t010 | Closeout                                                                                                                                                               | 15m | t009                               |

## Definition of done

- Every residual named in t001–t006 has a dated **PASS** or **FAIL** line in this README's Evidence section, with the exact command/verb, the surface exercised (REST / GraphQL / MCP / UI), and the observed result. "Reasoned from code shape" is not evidence.
- Every **FAIL** is either fixed inside its task (≤ ~30m, with a regression test) or filed as a `w1/NNN` inbox note carrying `file:line`, and the source milestone's "Unverified" bullet is annotated with the outcome and pointer.
- `dev-1` and `dev-3` were up simultaneously during the sweep with no port, namespace, Kratos-origin, or Hydra-issuer collision — the isolation `w1/m72` claimed but never walked.
- No residual listed here is still phrased "deferred to the next QA pass" anywhere on the board.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w1` 2026-09-09 proposal 2. A sweep of every milestone closed since 2026-08-20 found the single most repeated unowned residual is "live verification deferred — no cluster/QA/prod access this session". The subset runnable on the local CAPD cluster: `.pm/w6/done/m114/README.md:120`, `m116:36,105`, `m118:61`, `m125:116`, `m127:112`, `m130:118-121`, `m133:112-113,149-150`, `m46:54`, `.pm/w9/done/m91/README.md:36-37`, `.pm/w1/done/m72/README.md:17` (t007 waived). Prod-only items (m91–m97 in w4, m99 in w6, w7/m77) are excluded — they need production access a worker does not have.
- **Goal linkage:** pillar 1 (Render-compatible REST/GraphQL/MCP + dashboard) — correctness of parity work already shipped. The board's own rule is that a milestone lands in `done/` when its observable end state is real; these ten closed with that state inferred, not observed.
- **Expected outcome:** the debt list stops compounding; any defect hiding behind "identical code shape, not independently reproduced" surfaces with evidence; the two-workstream isolation `dev-N` was built for is finally demonstrated.
- **Why now:** the shared CAPD cluster is healthy for the first time in weeks (`w1/084` ran its convergence checks on two live nodes on 2026-09-09) and `w3/m79` is already exercising `dev-3`, so the concurrent-workstream walk costs nothing extra. w1's queue is empty. The window may not last — the cluster has been lost twice since m72.
- **Render parity:** included — in-task fixes may touch REST/GraphQL/MCP/UI, and t004/t005 explicitly compare surfaces against each other; any drift found is filed, not silently accepted.

## Evidence

_(filled by t001–t006; one dated line per residual)_
