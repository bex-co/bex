# w9 · m178 — `outcome.cost` reports sandbox compute in one USD total (Devin parity)

**Worker:** worker9 **Goal:** a session's `outcome.cost` carries `computeUSD` (its sandbox compute, priced like the bill) and `totalUSD` (model + compute) next to `modelUSD`, so a customer sees one number per session that matches their invoice. **Status:** in progress — t001–t003, t005 done; next t004 parity and the live check after deploy

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Attribute sandbox compute to its session — **DONE** | 60m | — |
| t002 | Price per-session compute into `outcome.cost` — **DONE** | 60m | t001 |
| t003 | Dashboard shows the session's total cost — **DONE** | 30m | t002 |
| t004 | Render parity (Devin reference for agent surfaces) | 20m | t002, t003 |
| t005 | Simplify — **DONE** | 15m | t004 |
| t006 | Test coverage | 45m | t004, t005 |
| t007 | Closeout | 10m | t006 |

## Definition of done

- On production, a finished session's `outcome.cost` shows `modelUSD`, `computeUSD` and `totalUSD` as decimal strings on REST, GraphQL and MCP. `computeUSD` equals the session's attributed `sandbox_compute_seconds` × the published sandbox price (checked read-only against the meter tables). `totalUSD = modelUSD + computeUSD`. The dashboard session page shows the total with its breakdown. No raw `sandboxSeconds` is exposed.

## Source + Goal linkage

- **Source:** [w9/078](../done/078.md) (was w21/006), user decision 2026-11-06 (below).
- **Goal linkage:** ADR094 D7 cost reporting; ADR008 pillar 5; Devin parity (one consumption figure per session).
- **Expected outcome:** CI callers and dashboard users can attribute agent spend per session without reconciling two units.
- **Why now:** the user decided the unit; model cost already ships, so compute is the remaining half of the figure.
- **Parity:** included — agent surface; Devin reference.

## User decision (2026-11-06)

- **Source:** explicit user decision ("好" to the recommendation in w9/078, 2026-11-06).
- **Chosen:**
  - Report sandbox compute folded into USD, as `computeUSD` plus `totalUSD` next to `modelUSD`.
  - Do not expose a raw `sandboxSeconds`.
  - Price it from the existing `sandbox_compute_seconds` meter with the same rate as the bill (`pricing.sandbox`).
- **Basis:** Devin shows one consumption figure per session (ACUs), with no separate compute seconds.
- **Accepted cost:** a billing-adjacent schema change that attributes each sandbox meter row to its session (t001).

## Live check (2026-11-06, production 24a9203ee)

- Production pin 24a9203ee (gitops `d65dc2f2b`). An authenticated `GET /v1/agent-skills` and `GET /v1/agent-secrets` answered 200. QA user via CLI device login, workspace `bex` (QA user is an admin there). Every fixture was deleted and verified 404; the CLI grant was revoked (204, then 401) and the Kratos session logged out.
- **PASS — one total.** The finished session `ags-dbmu0pa7qbhc739703q0`, whose sandbox ran after the deploy, has `outcome.cost = {modelUSD "0.002913", computeUSD "0.001293", totalUSD "0.004206"}`. `totalUSD` equals model + compute, and `computeUSD` is greater than 0.
- **Not run:** the read-only SQL cross-check against `agent_session_sandboxes`/`usage_hourly` (no read-only DB path in this run), and the dashboard cost line (browser).

## Browser check (2026-11-06, production 24a9203ee)

QA user in the `bex` workspace, Playwright at 1440×900 and 375×812. Fixtures used the `qa-ui-` prefix and were all deleted; the Kratos session was logged out.

- **PASS — cost line.** Session pages read, for example:
  - "Cost $0.011215 · model $0.008902, compute $0.002313" (`ags-dbmu4366kvbs738fl6qg`);
  - "Cost $0.005133 · model $0.00228, compute $0.002853" (`ags-dbmu6qu6kvbs738fl8cg`).
- **Observations, not failures:**
  - The line is absent right after a fast turn and appears on a later read, once compute has accrued.
  - Compute keeps growing after the turn ends while the sandbox sits in its idle grace ($0.002853 → $0.003833 on the same session).
  - The page header still reads "$0.01 model cost", which is model cost only and differs from the total below it.

## Recheck (2026-11-06, production b8370c30c)

Production pin b8370c30c (gitops `ca9b90b8f`, deploy run 42238432288 success). Signed in as the QA user in the browser; API calls were made from it with the session cookie (REST, and MCP over `/mcp/cloud-agents`). The CLI device flow stopped at a Kratos reauthentication prompt, so no CLI grant was issued. Every fixture was deleted (DELETE 204, then GET 404), the Kratos session was logged out with this run's jar, and the run dir was removed.

- **PASS — header cost.** On `ags-dbmvit7uqs1c73d5q1o0` the header button reads "$0.017958 total cost". Its popover shows "Model $0.016638 · compute $0.00132", matching the spend line "Cost $0.017958 · model $0.016638, compute $0.00132". Session `ags-dbmvi4hpc2us73bhtdg0` had `outcome.cost` `{modelUSD 0.000457, computeUSD 0.001831, totalUSD 0.002288}` (sum exact).
- **Session header skeleton:**
  - 1440×900: the header block is 57px in both states and the body starts at y=105 in both (body 714 vs 713px). The previous 6px shift is fixed.
  - **375×812, remaining +18px:** the ready header is 75px vs the 57px skeleton. At narrow width the ready header wraps to a second line, likely the new "total cost" button, and the skeleton reserves one line.
