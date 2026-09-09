# w4 · m98 — Recover interrupted environment-group saves

**Worker:** worker4 **Goal:** An interrupted environment-group content operation can recover safely without permanently blocking the group or the workspace's group list. **Status:** todo

**Estimate:** 360m (6h) including the standing closing tasks. Starts after `w4/m97/t008`.

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Persist operation ownership, phase, and recovery state within existing secret storage | 60m | w4/m97/t008 |
| t002 | Recover interrupted mixed saves while preventing stale writers from committing | 75m | t001 |
| t003 | Make interrupted clone source locks recoverable without changing source contents | 45m | t002 |
| t004 | Keep group lists usable and expose actionable busy/recovery states across existing surfaces | 60m | t002, t003 |
| t005 | Render parity | 30m | t004 |
| t006 | Simplify | 30m | t005 |
| t007 | Test coverage | 45m | t005, t006 |
| t008 | Closeout | 15m | t007 |

Task ids in depends_on are relative to w4/m98 unless written as a full wN/mN/tNNN id. Completed dependency files resolve through done/ locations; their ids remain stable after the move.

## Definition of done

- A durable record identifies the owning operation and phase, the committed revision, and the secret-store state required for recovery. Secret material stays in existing secret custody and never reaches public status, logs, or audit payloads.
- Controlled termination after admission and each OpenBao/projection write boundary, followed by another instance's recovery, yields a coherent prior or new committed configuration within a documented bound; an acknowledged save is not discarded.
- Reclamation cannot steal an active operation, and a delayed or resumed old writer cannot overwrite a newer operation. Clearing busy solely because a timer elapsed does not count as recovery.
- Interrupted clones release or recover source protection without mutating the source or copying a mixed revision. Existing target authorization and creation compensation remain intact.
- Lists remain usable for unaffected groups. A busy or repair-required group has an explicit public state or error representation; it is not silently omitted or presented as current, healthy data.
- REST, GraphQL, MCP, and dashboard agree on available state, retry guidance, and conflict behavior. Retry does not regenerate an already committed secret or misrepresent a rollout as successful.
- A dev-4 termination/restart drill against real OpenBao and Kubernetes records recovery boundaries, results, stale-writer rejection, cleanup, and limitations. Relevant backend checks and dashboard checks pass.

## Source + Goal linkage

- **Source:** User-approved `$pm-brainstorm for w4` proposal, 2026-09-08. The failure paths are verified by source inspection; no production interruption or recovery has been performed.
- **Source:** `lego/backend/internal/envgroups/patch.go:187` persists only busy state and generation when claiming a content operation; its restoration snapshots live in the in-memory `groupPatchTxn`. `clone.go:64` also holds this persistent source lock.
- **Source:** `service.go:1513` refuses a busy or repair-required group, and `ListEnvGroupsFiltered` at `service.go:273` aborts the whole list when any hydrated group fails.
- **Source:** Depends on [w4/m97](../m97/README.md), which establishes metadata concurrency and lifecycle coordination. Completed [w2/m73](../../w2/done/m73/README.md) established staged saves and Clone; this milestone closes their process-interruption recovery gap.
- **Goal linkage:** [V0 roadmap](../../GOAL.md), goals 1 (reliable application deployment/lifecycle) and 2 (operational availability), plus [ADR008](../../../docs/ADR008-vision.md)'s API-first, deterministic agent workflows.
- **Expected outcome:** A surviving or restarted API instance recovers an interrupted save to a consistent committed state within a documented bound, and one busy group no longer prevents users from accessing unaffected groups.
- **Why now:** A process exit after the busy claim can leave subsequent reads and saves returning conflicts indefinitely. The source lock survives restart, while the information required to restore a partially written configuration does not. m97 must land first because recovery needs its mutation coordination.
- **Render parity:** included because the fix changes an existing tenant-facing contract on REST, GraphQL, MCP, and dashboard. [ADR018](../../../docs/ADR018-render-parity.md)'s **Environment groups (+ link / unlink)** row (line 113 at source review) is already ✅ across those surfaces. This repairs the shipped guarantee; it does not claim a new missing feature. [Render's environment-group documentation](https://render.com/docs/configure-environment-variables#environment-groups) establishes shared configuration, membership, scope, and rollout behavior; Bex concurrency/recovery guarantees require Bex evidence.
- **Closing work:** t005 Render parity, t006 Simplify, t007 Test coverage, and t008 Closeout are included in the task count and estimate. Test coverage depends on both parity and the completed simplify pass.

## Scope and guardrails

- Use m97's mutation coordination and existing OpenBao custody; do not introduce a second plaintext credential store, secret-bearing logs, or client-side recovery snapshots.
- Preserve the workspace migration/dual-read window and update group deletion/purge handling for any added recovery artifacts.
- Bound work per recovery pass and respect process cancellation. A lease alone is not a fence against late writes to OpenBao or Kubernetes.
- No new public administrative reset or force-unlock endpoint is implied. Recovery states must offer actions that can actually complete without concealing partial state.
- Keep this scoped to environment-group saves and clone source protection; do not generalize into an unrelated platform workflow engine.

## Evidence

Pending. Materialization schedules implementation and verification; it is not a completion claim. Record commands, fixture identities, observable results, evidence paths, limitations, and cleanup here as work proceeds. Preserve the workstream's isolated dev-4 environment and avoid other workers' namespaces and ports.
