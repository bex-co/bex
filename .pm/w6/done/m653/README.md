# w6 · m653 — The other 49 Render templates that need no values deploy unchanged on bex

**Worker:** worker6 **Goal:** every `render-examples` Blueprint that bex can deploy without asking for a value (69 by w6/m644's count, 20 of them already run) has been deployed unchanged on production in `bex-canary` and ended in a recorded outcome, and a private service's check connects to it from another service instead of reading its address. **Status:** done 2026-11-26

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [List the 49 templates w6/m644 did not run, each with its documented check](done/t001.md) — **DONE** | 45m | — |
| t002 | [Check a private service from a peer in `scripts/render-examples-live.sh`](done/t002.md) — **DONE** | 45m | t001 |
| t003 | [Run the 49 templates on production in `bex-canary` and triage every failure](done/t003.md) — **DONE** | 3h | t002, w6/185 |
| t004 | [Record the full no-value coverage in ADR018 and ADR049](done/t004.md) — **DONE** | 30m | t003 |
| t005 | [Simplify](done/t005.md) — **DONE** | 20m | t004 |
| t006 | [Test coverage](done/t006.md) — **DONE** | 45m | t004 |
| t007 | [Closeout](done/t007.md) — **DONE** | 10m | t005, t006 |

## Definition of done

1. Every template on w6/m644's no-value list that m644 did not run (49 by its count; t001 recounts) was deployed unchanged in `bex-canary` by `scripts/render-examples-live.sh`, on a production pin the result page names.
2. Each one ends in exactly one of these outcomes:
   - live, with its documented check passing;
   - failed, with the failure filed as a w6 inbox note or matched to an open board item;
   - failed, with evidence that Render fails the same way.
3. A private service's check connects to its internal address from another service in the same workspace and reads the answer. `addr`, which only reads the address the first deploy detected, is no longer what passes a private service.
4. `docs/render-artifacts/render-examples-live.md` holds all 69 rows. No raw log or other evidence file is committed.
5. After the run, `bex-canary` holds none of the resources the run created.
6. ADR018's Blueprint row and ADR049's corpus-guard paragraph give the full count and the pin.

## Source + Goal linkage

- **Source:** `/pm-brainstorm one aspect of render parity for w6`, 2026-11-26 (the fifth run that day), item 1, filed by `/pm them to w6`. Its aspect was Render's one-click templates running unchanged on bex. It is the follow-up [w6/m644](../m644/README.md) left open, whose result page says "the same harness can widen the sample later".
- **Goal linkage:** ADR008 § Render parity first: a Render template that deploys with one click on Render must deploy unchanged on bex. The row is ADR018 [`deployment-sources-iac.md`](../../../../docs/ADR018/deployment-sources-iac.md), "Blueprint / `render.yaml` IaC".
- **Expected outcome:**
  - The no-value half of Render's templates is measured end to end: 69 of 69, not 20 of 69.
  - Each new failure class is a board item.
  - Private services are checked from another service, as their guides check them.
- **Why now:**
  - The first batch found a bex-side failure in 4 of 20 templates (w6/183, w6/184, w6/186, plus the GitHub budget in w6/185). Reading Render's docs found one stale claim across five aspects the same day.
  - The harness exists, so the remaining cost is mostly waiting on builds.
  - t003 waits on [w6/185](../../185.md) because anonymous GitHub reads are 60 an hour across every workspace. If w6/185 is parked instead, t003 runs at no more than 20 creates an hour and says so.
- **Render parity task omitted:** this milestone measures, extends a script and records. It changes no REST, GraphQL, MCP or dashboard surface. Each gap it finds is filed as its own item, which carries its own parity check.
- **Assumption, not a user decision:** as in w6/m644, templates that name paid plans are deployed as written in `bex-canary` (a first-party workspace) and deleted after each run.

## Closeout (2026-11-26, `/loopx w6`)

- **Result:** on production pin `9b91284a4a2e`, 34 of the 49 templates w6/m644 did not run went live unchanged and passed their documented checks, so 46 of the 69 no-value `render-examples` Blueprints run on bex. Ten fail on Render too: stale `latest` images, an image Docker Hub no longer serves, unpinned dependencies on new majors, and a Prisma older than OpenSSL 3. Five are bex gaps: [w6/186](../../186.md) (`elasticsearch`, `prometheus`, `rabbitmq`), [w6/184](../../184.md) (`langfuse4-template`) and the new [w6/189](../../189.md) (`temporal`). The run also filed [w6/187](../../187.md), [w6/188](../../188.md) and [w6/190](../../190.md). [render-examples-live.md](../../../../docs/render-artifacts/render-examples-live.md) has every row, and ADR018 and ADR049 cite it.
- **Peer check:** a private service passes only when a busybox worker in its workspace and environment reads its answer over the private network. Production proved both directions on `clickhouse`: port 8123 answered `Ok.`, and port 9999 failed at `check`. `addr` remains only for `temporal-frontend`'s gRPC.
- **Deviations from the tasks:**
  - w6/185 was not shipped, so t003 ran without waiting for it, at no more than 18 creates an hour (the last seven at 8), as the milestone allows. Three creates were still refused with `400` "not authorized" and rerun.
  - The peer grammar gained a `<protocol>:` prefix for a template that moves a well-known port (`redis:10000`).
  - `redash`'s first check text came from Redash 10. It was corrected to Redash 8's lines and run again.
  - The script now matches a service by its own name when the Blueprint lists it under a shortened slug (w6/190).
- **Cleanup:** every Blueprint, resource, disk, peer and project the runs created was deleted, and each by-id `GET` answered `404`. `bex-canary` holds only what it held before, plus another QA session's own `qa-20261126-…` fixtures. The run's API key is revoked, and the QA session is logged out.
