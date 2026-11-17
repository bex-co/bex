# w6 · m455 — A new service's first bytes of egress are billed: a router series that starts inside the hour counts from zero

**Worker:** worker6 **Goal:** the hourly egress meter bills the bytes a Traefik router series counted before its first scrape, without ever billing more than was served, and billing switches to that rule only after an old-versus-new comparison on recorded production hours proves it. **Status:** done — 2026-11-17: on production a new probe service that served 10,000,000 body bytes and was deleted bills 10,000,000 for its hour, the same on REST, GraphQL and MCP

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [Verify Traefik's router-metric cleanup: a removed router's series goes away, and a recreated one counts from zero](done/t001.md) — **DONE** | 45m | — |
| t002 | [The new-series rule in `egressquery`, proven in Prometheus's engine](done/t002.md) — **DONE** | 1h30m | t001 |
| t003 | [Old against new on recorded production hours](done/t003.md) — **DONE** | 1h30m | t002 |
| t004 | [Switch the egress meter and the month-to-date read to the rule; ADR023 records it](done/t004.md) — **DONE** | 45m | t003 |
| t005 | [Re-run the m333 probe on production: the bytes served are the bytes billed](done/t005.md) — **DONE** | 45m | t004 |
| t006 | [Render parity](done/t006.md) — **DONE** | 30m | t005 |
| t007 | [Simplify](done/t007.md) — **DONE** | 15m | t006 |
| t008 | [Test coverage](done/t008.md) — **DONE** | 30m | t006 |
| t009 | [Closeout](done/t009.md) — **DONE** | 10m | t007, t008 |

## Definition of done

- A `promtool` test evaluates the production billing expression in Prometheus's engine and shows five things. A router series that first appears inside the hour counts from zero. A series with a reset in the hour is still skipped by the health gate. A series gone stale before the hour ends (a deleted service) still counts. A series missing at the hour's start because its Traefik target was not scraped then is not counted from zero. A series present at the start bills exactly what `increase()` billed before.
- Old against new on every recorded production hour Prometheus still holds: the two differ only on series the rule classifies as new, and none of those series reappeared carrying an earlier cumulative value. No hour counts more than was served.
- On production, after the deploy that switches billing, a new probe service in `bex-canary` that serves 10,000,000 body bytes and is deleted bills 10,000,000 bytes for its hour, the same on REST, GraphQL and MCP.
- ADR023 records the rule, its guard and the comparison.

## Source + Goal linkage

- **Source:** w6/123 ([done/123.md](../123.md)), found by w6/m333 t005; the user's `/unblock` decision of 2026-11-16 (D2, "授权修复，先验证再切换": authorize the fix, verify before switching), recorded in that note.
- **Goal linkage:** ADR008 Render parity first. Render bills all outbound bandwidth ([outbound bandwidth](https://render.com/docs/outbound-bandwidth), ADR030 §14), and ADR023's rule is "undercount, never invent".
- **Expected outcome:** a new service's first responses, and a router's first response with a new status code, are billed. In m333's probe that was 5.4 MB of 10 MB.
- **Why now:** every new service and every short-lived one undercounts today. m333 now bills a deleted service's last hours, so the first-scrape gap is the largest known undercount left in egress.
- **Render parity task:** included. Billed bandwidth is tenant-facing on REST, GraphQL, MCP and the Billing page, though no field changes.

## BLOCKED — 2026-11-17: the production deploy carrying the switch

- **Done.**
  - t001: Traefik restarts a series from zero once a successful scrape has missed it, and `up` tells that apart from a scrape gap.
  - t002: the rule and its engine tests.
  - t003: old against new on 97 production hours. They differ only on the 2,168 new series, +0.0325%.
  - t004: the meter and the month-to-date read switched; ADR023 records it.
  - t007 and t008.
  - Before the switch deployed, a real probe (t005 Progress) served 10,000,000 bytes from a new service. On production Prometheus, `increase()` reads 0 for that hour and the rule reads 10,000,000.
- **Gate (time, clears by itself).** `deploy.yml` batches deploys: it deploys main only once the production pin is 6 h old. At 09:04Z the pin was 1.3 h old, so the deploy carrying `f0fe4be41` is the hourly run at about 14:17Z on 2026-11-17.
- **How to check it cleared.** `gh run list --workflow deploy.yml` shows a successful run whose `deploy` job ran on a head containing `f0fe4be41`.
- **Then.** t005 re-runs the probe and reads its billed hour. t006 reads that hour on REST, GraphQL and MCP. t009 closes the milestone.

## Unblock — the deploy carrying the switch ran (2026-11-17)

- **Affected:** t005, t006, t009.
- **Source:** verified evidence, `/loopx w6` worker.
- **Finding.** Production pin commit `fc4fbf28a` ("pin platform images to db48cde2bae9", 12:08Z) carries the switch: `git merge-base --is-ancestor f0fe4be41 db48cde2bae9` exits 0. Its `deploy.yml` run 44076479949 (`workflow_dispatch` on `db48cde2bae9`) passed every job, and its `deploy` job succeeded 12:07:50Z–12:13:20Z. At 18:10Z both production `bex-api` pods run that pin's image (`bex-operator@sha256:ced89914…`), started 12:12Z. The earlier pins `5bcfe8939f5b` (09:41Z), `19103ba12f70` and `6984548a30f8` also carry `f0fe4be41`.
- **Out of scope here.** w6/133 (`e0c68862d`) extended the same rule to Postgres, Key Value and WebSocket egress; this milestone reads only the HTTP source.
- **Remaining gates:** none.
- **Next.** t005 re-runs the probe and reads its billed hour (10,000,000 bytes). t006 reads that hour on REST, GraphQL and MCP. t009 closes the milestone.

## Closeout — 2026-11-17

- **Definition of done.**
  - The `promtool` test is t002's `TestBilledOnPrometheus`.
  - Old against new on 97 recorded production hours is t003: +0.0325%, and the hours differ only on new series.
  - ADR023 § New series count from zero records the rule, its guard and the comparison (t004).
  - On production, t005's probe `qa-20261117-m455-egress-b` served 10,000,000 body bytes and was deleted, and its hour bills 10,000,000. `increase()` reads 3,000,000 for that hour. t006 read the same 10,000,000 on REST, GraphQL and MCP, and the Billing page shows its 0.00931 GB line.
- **Probe cleanup.** Both probe services (`srv-dbu1hnlikg6s738lasr0`, `srv-dbu9i28gt2jc73dje0g0`) are deleted, and `GET` answers 404. No API key was created, and the QA session was logged out.

