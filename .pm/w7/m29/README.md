# w7 · m29 — Execute and record the ADR031 restore drills

**Worker:** worker7 **Goal:** Every platform backup chain (etcd, OpenBao, bex-db) is proven restorable by an actually-executed drill, with corrected runbooks and drill records in ADR031. **Status:** todo

## Tasks (in order)

| id   | title                                                       | est | depends_on       |
| ---- | ----------------------------------------------------------- | --- | ---------------- |
| t001 | etcd snapshot → restore drill (mock cluster)                | 60m | —                |
| t002 | OpenBao Raft snapshot → restore drill                       | 45m | —                |
| t003 | bex-db barman PITR → new-cluster drill                      | 60m | —                |
| t004 | Record drill outcomes + re-drill cadence; fix runbook steps | 30m | t001, t002, t003 |
| t005 | Simplify                                                    | 30m | t004             |
| t006 | Test coverage                                               | 30m | t004             |
| t007 | Closeout                                                    | 15m | t006             |

## Definition of done

Each of the three restore drills has been executed end-to-end on a disposable/mock environment (restore into new, prod untouched). `docs/ADR031-platform-data-backup.md`'s drill-record section carries a dated entry with outcome and corrections for each of the three chains, and every runbook step that turned out wrong during a drill is fixed in the doc.

## Source + Goal linkage

- **Source:** `/pm-brainstorm more milestones for each worker to work on until feature parity` 2026-07-14 (item 7); `w2/done/m27`'s closing residual — "operational drills require live cluster execution post-deploy" — never picked up.
- **Goal linkage:** GOAL #7 (security review) follow-through; `docs/ADR031-platform-data-backup.md` §restore runbooks + drill records.
- **Expected outcome:** the backup chains stop being assumptions — each has a recorded, dated, successful restore; runbooks are corrected from observed reality.
- **Why now:** w7's queue is empty; every day the drills stay unrun, backup correctness is a hope. Drills belong before they're needed, not after.
- **Render parity omitted:** no REST/GraphQL/MCP/UI surface change — operational drills + runbook docs only.
