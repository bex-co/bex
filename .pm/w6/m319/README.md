# w6 · m319 — Each plan includes Render's outbound bandwidth, and a card-less workspace past it spins down

**Worker:** worker6 **Goal:** outbound bandwidth is charged only above Render's per-plan allowance, and a workspace without a payment method that uses it up has its services suspended until the month turns, as on Render. **Status:** in progress — t001–t007 done 2026-11-10; closeout t008 waits for the production read after the deploy

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [Per-plan included bandwidth in the catalog and the estimate](done/t001.md) — **DONE** | 1h | — |
| t002 | [Stripe receives only the bandwidth above the allowance](done/t002.md) — **DONE** | 1h | t001 |
| t003 | [Past the allowance without a payment method, services spin down until the month turns](done/t003.md) — **DONE** | 2h | t001 |
| t004 | [`includedBandwidth` on every usage surface, with warning emails](done/t004.md) — **DONE** | 1.5h | t001 |
| t005 | [Render parity](done/t005.md) — **DONE** | 30m | t002, t003, t004 |
| t006 | [Simplify](done/t006.md) — **DONE** | 15m | t005 |
| t007 | [Test coverage](done/t007.md) — **DONE** | 30m | t006 |
| t008 | [Closeout](t008.md) | 10m | t007 |

## Definition of done

- A Hobby workspace's estimate and Stripe emission exclude its first 5 GB of outbound bandwidth a month (tests).
- A card-less workspace past its allowance has its services suspended with the billing suspender, and binding a card resumes them (amended 2026-11-10: verified by the sweeper, apps-service and real-Postgres store tests; dev-N runs without Stripe, so the sweeper is not wired there).
- `includedBandwidth` reads the same on REST, GraphQL, MCP and the dashboard; ADR030 records the allowance.

## Source + Goal linkage

- **Source:** inbox note `w6/116` (filed 2026-11-10 while shipping `w6/m308`), promoted by `/loopx w6`; the note is kept as `source-w6-116.md`.
- **Goal linkage:** ADR008 Render parity first; Render includes Hobby 5 GB, Pro 25 GB and Scale 1 TB of outbound bandwidth and spins down a card-less workspace's services past it ([outbound bandwidth](https://render.com/docs/outbound-bandwidth)).
- **Expected outcome:** customers stop paying for traffic Render includes, and a card-less workspace has Render's bound.
- **Why now:** the same shape `w6/m308` just built for build minutes; the code paths are fresh.
- **Render parity task:** included (usage surfaces, suspension and billing are tenant-visible).
- **Recommendation recorded, not a user decision:** the allowance uses Render's GB figures on bex's existing public egress meter (no private-link traffic exists in bex).
