# w6 · m308 — Each plan includes pipeline minutes, and builds stop with "Build Minutes Exhausted"

**Worker:** worker6 **Goal:** build minutes are charged only above Render's per-plan allowance, and a workspace without a payment method stops building when the allowance runs out, as on Render. **Status:** done 2026-11-10 — verified on production after the 21:47 UTC deploy

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [Per-plan included minutes in the price catalog and the estimate](done/t001.md) — **DONE** | 1h | — |
| t002 | [Stripe receives only the minutes above the allowance](done/t002.md) — **DONE** | 1.5h | t001 |
| t003 | ["Build Minutes Exhausted": builds stop without a payment method](done/t003.md) — **DONE** | 2h | t001 |
| t004 | [`buildMinutes` on every usage surface, with warning emails](done/t004.md) — **DONE** | 1.5h | t001 |
| t005 | [Render parity](done/t005.md) — **DONE** | 30m | t002, t003, t004 |
| t006 | [Simplify](done/t006.md) — **DONE** | 15m | t005 |
| t007 | [Test coverage](done/t007.md) — **DONE** | 30m | t006 |
| t008 | [Closeout](done/t008.md) — **DONE** | 10m | t007 |

## Definition of done

- A Hobby workspace's estimate and Stripe emission exclude its first 500 build minutes of the month (tests; amended 2026-11-10: the emitter, estimate and real-DB export tests — no Stripe test-mode harness exists and dev-N runs without Stripe).
- A workspace with no payment method that has used its allowance gets "Build Minutes Exhausted" on its next deploy, and binding a card lets it build again (amended 2026-11-10: verified by envtest `build_minutes_exhausted_envtest_test.go` and the sweeper tests; dev-N runs without Stripe, so the sweeper is not wired there).
- `buildMinutes {used, included}` reads the same on REST, GraphQL, MCP and the dashboard.
- ADR030 §13's "implementation pending" is updated to shipped.

## Source + Goal linkage

- **Source:** this session's 2026-11-10 discussion of the tenant build experience against Render, then `/pm-brainstorm for w6` (same session) and `/pmx them all to w6`; docs/ADR030 §13 (2026-11-10) and the ADR018 § Gap backlog row.
- **Goal linkage:** ADR008 Render parity first; Render includes Hobby 500 / Pro 1,000 / Scale 5,000 minutes, buys overage automatically, and stops pipeline tasks when minutes run out without a payment method or past the spend limit ([build pipeline](https://render.com/docs/build-pipeline)).
- **Expected outcome:** customers stop paying for minutes Render includes, and builds have Render's budget bound once the per-workspace cap is gone (`m306`).
- **Why now:** today every build minute is estimated and sent to Stripe (whether production billing runs in live mode was not verified). Schedule after `m307`, so the allowance counts minutes on Render's basis.
- **Render parity task:** included (usage surfaces and deploy failure wording are tenant-visible).
- **Recommendations recorded, not user decisions:** Enterprise gets Scale's 5,000 minutes until a contract says otherwise; a mid-month plan change uses the plan in force when each hour is emitted. Both rest on ADR030 §13's parity-sufficient reading.
