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
