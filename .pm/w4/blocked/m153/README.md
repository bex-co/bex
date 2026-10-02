# w4 · m153 — Preserve Key Value data when re-enabling journaling

**Worker:** worker4 **Goal:** switching a managed Key Value from Snapshot only to Journal + Snapshot preserves the current keyspace instead of replaying an older journal. **Status:** blocked (t001, t002 and t005 done; pinned Linux image verification and deployed TLS/UI acceptance remain)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Define and gate the persistence transition state — **DONE** | 35m | — |
| t002 | Complete the journal handoff before rolling Valkey — **DONE** | 55m | w4/m153/t001 |
| t003 | Verify shared writers, mode transitions and controls | 40m | w4/m153/t002 |
| t004 | Render parity and truthful persistence copy | 20m | w4/m153/t002, w4/m153/t003 |
| t005 | Simplify the milestone changes — **DONE** | 20m | w4/m153/t004 |
| t006 | Prove data survival with real Valkey restarts | 50m | w4/m153/t004, w4/m153/t005 |
| t007 | Closeout after live data acceptance | 10m | w4/m153/t006 |

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

## Current verdict — 2026-10-02

See [current implementation](current-implementation.md) for the replacement implementation, passing native Valkey 7/8 data matrix and remaining Linux-image/release/QA gates. t001/t002/t005 are done; t003/t004/t006/t007 retain unobserved acceptance. The archive below is historical and must not be applied over the new implementation.

## Earlier blocked attempt — 2026-10-02

The complete 19-file implementation is preserved in [implementation.patch](implementation.patch), with its transition table, checks and limitations in [verification.md](verification.md). **No product source changes from m153 were shipped.** The patch was reverse-checked and removed from the working source so other w4 items can ship without carrying an unverified persistence change. All task frontmatter remains open until the patch is applied, fully verified and landed.

**Gate — local environment owner / worker4:** provide a healthy isolated Docker engine, apply the patch and pass the complete 42-case pinned Valkey 7/8 matrix. The shared daemon repeatedly stalled `start`, `exec` and helper operations beyond both 45-second and 120-second budgets. The latest run completed all 21 Valkey 7 cases, but Valkey 8 hit a Docker start timeout; this is not a passing full suite. The two configured responsive Docker contexts resolve to the same socket, and no alternate active engine was available. Restarting the shared daemon would affect other workstreams and was not performed.

**Following gate — release/QA owner:** after the verified patch ships and its operator/CRD/dashboard release is active, replay the exact Free/public TLS/UI data-survival sequences, three-surface state, rename controls and complete fixture/session cleanup. The original live acceptance remains required.
