# w2 · m68 — ADR059 hibernation: spike → snapshot/rehydrate → pin, retention, billing

**Worker:** worker2 **Goal:** the Hibernated tier of ADR059 becomes real — an idle agent workspace is reclaimed to an encrypted filesystem snapshot in object storage and rehydrates on connect/turn within the p50<~5s / p95<~15s budget; retention (7d default, dirty-git extension) and the opt-in **pinned** never-expire primitive land with storage metering, quotas, and a see/stop/unpin surface. **Status:** DONE (2026-08-15) — **t001–t009 DONE (2026-08-15)**; t010 (live hibernate→rehydrate acceptance) was removed in the 2026-08-15 board cleanup as infeasible pre-enablement: it required a live cluster with the `BEX_AGENT_SNAPSHOT_S3_*` contract provisioned plus human confirmation, and prod has neither. The live walk (idle → hibernate → object present/pod gone → rehydrate with tree intact → latency vs the p50<~5s / p95<~15s budget) is **now the enablement step**: run it when the S3 contract is first provisioned, before relying on the tier. The whole tier is **env-gated OFF by default** (`BEX_AGENT_SNAPSHOT_S3_*` unset ⇒ reclaim stays Terminate, byte-identical to w2/m67), so it ships safely with the live acceptance as the enablement gate. Implemented: D7 spike → **B**; the `agent_sessions` hibernation data model (migration 0073 + claim/hibernate/rehydrate/pin/retention store methods); the S3 `SnapshotStore` (per-workspace prefix, presigned PUT/GET, bucket-default SSE); the Completer reclaim seam (idle → snapshot → `hibernated`, fallback Terminate on any failure) with dirty-git-extended 7d retention sweep; Resume/Steer rehydrate (fresh sandbox + presigned restore URL) with resume-latency instrumentation; the driver's `restoreWorkspace` setup hook (curl→tar, uncommitted work preserved, real tar round-trip test); pin/unpin verbs + pin quota + storage metering; the REST/GraphQL/MCP + dashboard surface (pin/unpin, hibernated chip, storage cost). All backend + driver + dashboard suites green; the tenant sandbox never receives a durable credential (presigned URLs only).

## Tasks (in order)

| id   | title                                                                                        | est | depends_on       |
| ---- | -------------------------------------------------------------------------------------------- | --- | ---------------- |
| t001 | D7 spike: OpenSandbox `Suspend` snapshot — where stored, survives Terminate?, scales? → pick A/B | 45m | —                | — **DONE** (verdict B) |
| t002 | Data model: Hibernated state + pin flag + idle/retention timestamps on the session/workspace row | 40m | —                | — **DONE** |
| t003 | Hibernate mechanism (A: retain `Suspend` snapshot / B: tar mutable mount → encrypted object storage) | 60m | t001, t002       | — **DONE** (B) |
| t004 | Rehydrate on connect/turn/Resume + resume-latency instrumentation against the SLOs            | 60m | t003             | — **DONE** |
| t005 | Retention sweep: 7d default, dirty-git extension, pinned skips delete; object-store GC backstop | 45m | t003             | — **DONE** |
| t006 | Pin/unpin + quotas + storage metering + the see/stop surface (REST/GraphQL/MCP + dashboard)   | 60m | t002, t005       | — **DONE** |
| t007 | Render parity sweep (bex extension; cross-surface consistency)                                | 30m | t006             | — **DONE** |
| t008 | Simplify pass over the milestone's changes                                                    | 30m | t007             | — **DONE** |
| t009 | Test coverage: state machine, hibernate/rehydrate failure modes, retention, pin/quota         | 60m | t007             | — **DONE** |
| t010 | Closeout (requires a live hibernate→rehydrate acceptance with recorded resume latency)        | 20m | t009             | — **REMOVED 2026-08-15** (live acceptance infeasible pre-enablement; deferred to the enablement step above) |

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
