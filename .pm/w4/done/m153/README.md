# w4 · m153 — Preserve Key Value data when re-enabling journaling

**Worker:** worker4 **Goal:** switching a managed Key Value from Snapshot only to Journal + Snapshot preserves the current keyspace instead of replaying an older journal. **Status:** done — 2026-10-02; all 7 tasks done. Both live TLS data-survival sequences passed on the deployed operator/dashboard ([live acceptance](live-acceptance.md)).

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Define and gate the persistence transition state — **DONE** | 35m | — |
| t002 | Complete the journal handoff before rolling Valkey — **DONE** | 55m | w4/m153/t001 |
| t003 | Verify shared writers, mode transitions and controls — **DONE** | 40m | w4/m153/t002 |
| t004 | Render parity and truthful persistence copy — **DONE** | 20m | w4/m153/t002, w4/m153/t003 |
| t005 | Simplify the milestone changes — **DONE** | 20m | w4/m153/t004 |
| t006 | Prove data survival with real Valkey restarts — **DONE** | 50m | w4/m153/t004, w4/m153/t005 |
| t007 | Closeout after live data acceptance — **DONE** | 10m | w4/m153/t006 |

Total: **7 tasks, about 3h 50m**. This schedules the fix; no product code was changed by the hunt.

## Definition of done

Repeat [finding.md](finding.md)'s Free/public fixture, using its actual external TLS client and UI settings sequence. These criteria come from probes run in sweep 15; the broader transition/error matrix is assigned to t003/t006.

- Start with Journal + Snapshot, write a baseline value and counter, then switch to Snapshot only. The original value and counter remain present, the expiry keeps its original deadline, and runtime `appendonly` becomes `no`.
- Write a new non-expiring key and increment the counter while Snapshot only is active. From a fresh detail page, switch back to Journal + Snapshot. After convergence, the new key and updated counter remain present, the baseline still matches, the expiry keeps its deadline, and runtime `appendonly` is `yes`. A successful PING alone does not satisfy this criterion.
- Repeat the stronger control: in Snapshot only, write a second marker, set the counter to 23 and issue `SAVE`. Change Maxmemory Policy to `noeviction` and verify the resulting ordinary restart retains the marker and 23. Then freshly reload the page and switch to Journal + Snapshot; the same saved marker and 23 survive that restart too.
- REST, GraphQL, MCP and the dashboard report the applied persistence setting coherently. During the handoff, they expose an existing truthful transition/failure state rather than claiming completion before safe conversion. Existing underscore/hyphen input normalization remains compatible.
- Rename the own fixture and freshly reload it: its resource ID, endpoint and connection fields remain unchanged. Delete it afterwards, verify API/list removal and exact resource cleanup, and revoke the verification session.

## Source + Goal linkage

- **Source:** continuous `$qa-find-bugs`, production sweep 15 on 2026-10-02, with `muse.env` and w4 selected by the user. [Finding](finding.md) contains two fresh-page data-loss reproductions, an explicitly saved/ordinary-restart control, complete API and data-plane probes, and the prior-work gap analysis.
- **Goal linkage:** ADR008 dependable managed hosting; [ADR021](../../../../docs/ADR021-keyvalue-management.md) managed Valkey persistence; [ADR018](../../../../docs/ADR018-render-parity.md) post-create persistence settings; [ADR031](../../../../docs/ADR031-platform-data-backup.md) data protection and the existing restore precedent.
- **Expected outcome:** strengthening persistence does not roll a working database back to the last period when journaling was enabled.
- **Why now:** the default store can enter this path through ordinary settings. Two saves completed with Available while a new key disappeared and a counter reverted (2→1, then 23→1).
- **Render parity included:** the data behavior and settings copy are tenant-facing across REST/GraphQL/MCP/UI. Free persistence is an intentional bex divergence; transitions involving Off must be distinguished from transitions between durable modes.
- **Dedupe:** uncovered runtime transition gap in w6/done/m127 and w4/done/066. w7/done/m69 already handles AOF conversion for throwaway restores, but the managed-resource reconciler does not call that script. w1/blocked/m166 addresses endpoint/readiness flapping; a successful authenticated PING still passes on the wrong keyspace.
- **Limits:** reproduced on Valkey 8.1.9, Free/public, through dashboard GraphQL writes. REST/MCP reads agree; their write paths, Blueprint/direct CR changes, Valkey 7, other modes and failure races are source-traced or unverified, assigned to t003/t006. All hunt resources were removed and its session revoked; the known TLS Secret residue was cleaned by exact identity under w4/m149's existing finding.

## Blocked closeout — 2026-10-02

The operator and dashboard implementation and local checks are complete ([verification](verification.md)). The production-deploy pipeline must release both components; QA must then execute both original Free/public TLS data-survival sequences, three-surface/UI state checks, rename and exact fixture/session cleanup. t006 retains this live acceptance and t007 remains open.

## Additional pre-ship review

Concurrent archived implementation history exposed legacy in-progress rollout source selection, unchanged-fleet rollout, missing same-mode journal and unknown-UI-mode cases. These follow-ups are implemented and covered by additional controller/real-engine/UI regressions. Full operator verification with both pinned engines and all-module lint passed; see verification.md for exact evidence and the legacy unknown-source limitation.

The earlier unshipped implementation patch is retained as history alongside [its original verification](prior-implementation-verification.md). The current implementation supersedes it; do not reapply it.

## Concurrent shipped implementation

Commit c92ebb526 shipped an independent implementation while this work was in progress. Its [implementation record](current-implementation.md) is retained as history. This change reconciles that implementation with live-process handoff and the additional checks described in [verification](verification.md). Hosted controls in t003/t004 remain open alongside t006/t007.

## Live acceptance — 2026-10-02

On deployed `1263d12ae` (includes `c92ebb526` and `125f4ddfc`), a fresh Free/public Valkey 8 fixture went Journal→Snapshot→new writes→Journal and kept the new key and counter 2. It then went Snapshot + `SAVE` (marker, counter 23)→`noeviction` restart→UI switch to Journal + Snapshot from a fresh page and kept marker and 23. In every case the TTL deadline was unchanged and `appendonly` was runtime-verified. REST/GraphQL/MCP/UI agreed, showing `config_restart`/Restarting during each handoff. Rename kept ID and connection fields. Delete reached zero artifacts, including TLS. Details: [live-acceptance.md](live-acceptance.md).
