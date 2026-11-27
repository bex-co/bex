# w4 · m690 — Render's price tables are watched: a Render price change fails a weekly job that names the `pricing.yaml` tier it moves

**Worker:** worker4 **Goal:** every row of Render's compute, Postgres, Key Value and cron price tables has a disposition, each bex tier is tested against Render's price × 0.70, and a weekly job fails when render.com/pricing changes any captured row. **Status:** done (2026-11-27; the dispatched `render-pricing` run on `main` passed, run 45946771123)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Capture Render's compute, Postgres, Key Value and cron price tables and its pricing FAQ as fixtures — **DONE** | 45m | — |
| [t002](done/t002.md) | A test gives every Render plan row a disposition and checks each bex tier against Render's price × 0.70 — **DONE** | 1h30m | t001 |
| [t003](done/t003.md) | A weekly job re-reads render.com/pricing and fails on any changed fixture row — **DONE** | 1h | t002 |
| [t004](done/t004.md) | ADR030 §9 and ADR018 point at the guard — **DONE** | 20m | t003 |
| [t005](done/t005.md) | Simplify — **DONE** | 15m | t004 |
| [t006](done/t006.md) | Test coverage — **DONE** | 30m | t004, t005 |
| [t007](done/t007.md) | Closeout — **DONE** | 10m | t006 |

Total: **7 tasks, about 4h30m**.

## Definition of done

- `lego/backend/internal/pricing/testdata/` holds Render's four price tables and its billing FAQ, each matching render.com/pricing on its read date.
- A test maps every Render plan row to a bex tier at Render's price × 0.70 (within a cent), a gap with its owner, or n/a with a reason, and fails on a one-cent change or a missing row.
- The Key Value connection-limit disagreement between Render's pricing page and its compute-plans page is recorded with the source bex follows.
- A weekly job fails on any changed, added or removed row of the plan table, the features page, the price tables or the FAQ, naming the bex constant; a dispatched run on `main` passes.
- ADR030 §9 and ADR018 name the guard.

## Source + Goal linkage

- **Source:** `/pm-brainstorm one aspect of render parity for w6`, 2026-11-26 (the aspect: what Render publishes outside its API pins that bex's parity ledger never read — its docs pages, its pricing page, and the Cloudflare headers its docs tell apps to use), filed to w4 by `/pm them to w4 and ship`. Re-checked at `origin/main` `f32c8164fb` before filing: no open, blocked or done item on any workstream owns it. Item 4.
- **bex today:** the plan comparison rows are guarded by `TestEveryRenderPlanRowHasADisposition` against fixtures re-captured by hand (2026-11-16); the price tables exist only as `pricing.yaml` comments and hard-coded test values. Every captured row still matched the live page on 2026-11-27.
- **Render:** render.com/pricing lists 15 compute, 24 Postgres, 9 Key Value and 11 cron plans; its Key Value connection column (`256mb` 200, `1g` 250) disagrees with its compute-plans page (250, 1,000).
- **Goal linkage:** [ADR030](../../../../docs/ADR030-pricing.md)'s promise of Render's prices less 30%, kept true without a person re-reading the page.
- **Expected outcome:** a Render price change reaches the board within a week, naming the `pricing.yaml` tier to move.
- **Why now:** Render is running a Starter price experiment (`enable_starter_price`), and Render changed its plans on 2026-04-23 and 2026-08-26; the current guard fires only after a manual re-capture.
- **Render parity:** omitted: fixtures, a test and a job, with no REST, GraphQL, MCP or dashboard change.
