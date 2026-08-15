# w2 · m68 — ADR059 hibernation: spike → snapshot/rehydrate → pin, retention, billing

**Worker:** worker2 **Goal:** the Hibernated tier of ADR059 becomes real — an idle agent workspace is reclaimed to an encrypted filesystem snapshot in object storage and rehydrates on connect/turn within the p50<~5s / p95<~15s budget; retention (7d default, dirty-git extension) and the opt-in **pinned** never-expire primitive land with storage metering, quotas, and a see/stop/unpin surface. **Status:** todo — t001 (the D7 spike) is the sequencing gate for t003+; t002 can proceed in parallel.

## Tasks (in order)

| id   | title                                                                                        | est | depends_on       |
| ---- | -------------------------------------------------------------------------------------------- | --- | ---------------- |
| t001 | D7 spike: OpenSandbox `Suspend` snapshot — where stored, survives Terminate?, scales? → pick A/B | 45m | —                |
| t002 | Data model: Hibernated state + pin flag + idle/retention timestamps on the session/workspace row | 40m | —                |
| t003 | Hibernate mechanism (A: retain `Suspend` snapshot / B: tar mutable mount → encrypted object storage) | 60m | t001, t002       |
| t004 | Rehydrate on connect/turn/Resume + resume-latency instrumentation against the SLOs            | 60m | t003             |
| t005 | Retention sweep: 7d default, dirty-git extension, pinned skips delete; object-store GC backstop | 45m | t003             |
| t006 | Pin/unpin + quotas + storage metering + the see/stop surface (REST/GraphQL/MCP + dashboard)   | 60m | t002, t005       |
| t007 | Render parity sweep (bex extension; cross-surface consistency)                                | 30m | t006             |
| t008 | Simplify pass over the milestone's changes                                                    | 30m | t007             |
| t009 | Test coverage: state machine, hibernate/rehydrate failure modes, retention, pin/quota         | 60m | t007             |
| t010 | Closeout (requires a live hibernate→rehydrate acceptance with recorded resume latency)        | 20m | t009             |

## Definition of done

On a real cluster: an idle workspace transitions Active → Hibernated (pod gone, encrypted snapshot present in object storage under the workspace prefix), then a connect/new turn rehydrates it with the working tree (including uncommitted edits and `~/.zed_server`) intact — measured resume latency recorded against the p50<~5s / p95<~15s budget. An unpinned hibernated workspace is deleted after the retention window (extended when the git tree is dirty); a **pinned** one is never auto-deleted but is metered (storage), counted against a per-plan pin quota, and unpins back onto the clock. The tenant can list workspaces with their state + storage cost and stop/delete/unpin on all three surfaces + dashboard. Corrupt-snapshot restore falls back to a clean re-clone with an explicit failure note, never a half-restore. All suites green.

## Source + Goal linkage

- **Source:** [docs/ADR059-agent-sandbox-hibernation.md](../../../docs/ADR059-agent-sandbox-hibernation.md) (Proposed; flips to Accepted when t001 picks A/B) — D2 state machine, D3 storage, D4 performance, D5 retention+pin, D6 cost visibility, D7 spike.
- **Goal linkage:** pillar 5 — turns fire-and-forget sessions into persistent (and pinnable) agent workspaces at snapshot cost instead of live-pod cost; the differentiator ADR047 D9 named.
- **Expected outcome:** idle workspaces cost object-storage cents, not pod compute; "come back tomorrow and keep editing" works; "never expire" exists as a bounded, billed primitive.
- **Why now:** m67 ships the Active tier, but hours-scale grace is the ceiling without hibernation; every day beyond it either loses user work (reap) or burns compute (long grace). The design is fully decided — only the A/B implementation choice waits on t001.
- **Render parity task INCLUDED:** t006 adds fields/verbs across REST/GraphQL/MCP + dashboard. Render has no agent-workspace product — t007 records the bex-extension position in ADR018's ledger and enforces cross-surface consistency rather than comparing to a Render behavior.

## Notes

- t001's outcome must be written back into ADR059 (Status → Accepted, D3 A-or-B chosen) before t003 starts.
- Security invariants from the ADR are non-negotiable in review: per-workspace prefix + ACL, ADR050 encryption, `bex-pre-snapshot` credential scrub before any snapshot, uid 10001 preserved through restore.
