# w4 · m97 — Preserve environment-group metadata during concurrent edits

**Worker:** worker4 **Goal:** Successful environment-group changes survive overlapping requests, and group membership stays consistent with linked App references. **Status:** todo

**Estimate:** 330m (5h 30m) including the standing closing tasks. No pending external prerequisite.

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Add conditional metadata updates that preserve unrelated concurrent changes | 60m | — |
| t002 | Keep rename claims and Environment moves consistent with committed metadata | 60m | t001 |
| t003 | Coordinate link, unlink, and delete with App references and current membership | 60m | t001, t002 |
| t004 | Preserve newer metadata during content commits, compensation, and stale-link pruning | 45m | t002, t003 |
| t005 | Render parity | 30m | t004 |
| t006 | Simplify | 20m | t005 |
| t007 | Test coverage | 45m | t005, t006 |
| t008 | Closeout | 10m | t007 |

Task ids in depends_on are relative to w4/m97 unless written as a full wN/mN/tNNN id. Completed dependency files resolve through done/ locations; their ids remain stable after the move.

## Definition of done

- Controlled requests against two Service instances sharing the same durable store reproduce and then prevent content-save versus rename, link versus link, link/unlink versus delete, and stale-link-prune versus metadata-edit lost updates.
- Successful renames leave metadata and workspace name claims consistent; failed or conflicting mutations do not release another group's claim or restore an obsolete name.
- Scope changes validate current membership and cannot silently admit a link from an incompatible Environment. Same-workspace and fresh sensitive authorization remain enforced.
- Committed link/unlink/delete outcomes agree with App envFromSecrets/filesFromSecrets references. A failure between resource writes has an explicit, retryable or compensated outcome without clobbering a newer operation.
- Content commits and compensation preserve unrelated newer metadata; stale-link pruning removes only the discovered stale ids. A deleted group cannot be resurrected by a delayed writer.
- A dev-4 drill using the real OpenBao and Kubernetes boundaries records the controlled interleavings and resulting metadata, claims, references, and error classes without secret values; all scratch resources are cleaned up.
- REST, GraphQL, MCP, and dashboard preserve existing names-only reads and rollout modes. Relevant backend tests and required checks pass; dashboard checks pass if its code or generated contract changes.

## Source + Goal linkage

- **Source:** User-approved `$pm-brainstorm for w4` proposal, 2026-09-08. Findings are verified by source inspection; controlled concurrency and live dev-4 reproduction remain implementation acceptance work.
- **Source:** `lego/backend/internal/envgroups/patch.go:253` commits metadata fetched before content admission; `service.go:1502` (`touch`) and `service.go:1650` (`writeMeta`) replace the complete metadata map without CAS. `patch.go:321` (`pruneStaleLinks`) writes an earlier metadata snapshot after rollout.
- **Source:** `service.go` contains the interacting rename, Move, link/unlink, delete, and Blueprint group-link seams. The same group can receive requests through REST, GraphQL, MCP, and the dashboard.
- **Source:** Completed [w2/m73](../../w2/done/m73/README.md) protects competing content patches and establishes scoped editing. Completed [w6/m108](../../w6/done/m108/README.md) removes deleted links after rollout. This milestone repairs metadata interactions between those paths; it does not reimplement their shipped workflows.
- **Goal linkage:** [V0 roadmap](../../GOAL.md), goals 1 (reliable application deployment/lifecycle) and 5 (multi-tenant control-plane correctness), plus [ADR008](../../../docs/ADR008-vision.md)'s API-first, deterministic agent workflows.
- **Expected outcome:** Concurrent callers receive either a coherent committed result or an explicit conflict. Successful metadata edits are preserved, and group membership agrees with the service Secret references.
- **Why now:** A content save can restore an older name or link set after another request succeeds. A lost link also excludes its service from subsequent shared-configuration rollouts. w4 has available capacity, and the changes are concentrated in the shared environment-group core.
- **Render parity:** included because the fix changes an existing tenant-facing contract on REST, GraphQL, MCP, and dashboard. [ADR018](../../../docs/ADR018-render-parity.md)'s **Environment groups (+ link / unlink)** row (line 113 at source review) is already ✅ across those surfaces. This repairs the shipped guarantee; it does not claim a new missing feature. [Render's environment-group documentation](https://render.com/docs/configure-environment-variables#environment-groups) establishes shared configuration, membership, scope, and rollout behavior; Bex concurrency/recovery guarantees require Bex evidence.
- **Closing work:** t005 Render parity, t006 Simplify, t007 Test coverage, and t008 Closeout are included in the task count and estimate. Test coverage depends on both parity and the completed simplify pass.

## Scope and guardrails

- Reuse the existing environment-group core and versioned secret-store boundary; do not solve cross-replica concurrency with a process-local mutex.
- Keep migration compatibility from w2/m80, including workspace-prefixed writes and the existing legacy-read window. This does not authorize the destructive migration cleanup in w2/035.
- Preserve fresh sensitive authorization, service-local value precedence, generated-value custody, and the existing save_only/deploy/rebuild contract.
- Metadata concurrency is this milestone's deliverable. Durable interrupted-operation recovery is the dependent m98; do not conceal a stranded operation by resetting its busy marker.

## Evidence

Pending. Materialization schedules implementation and verification; it is not a completion claim. Record commands, fixture identities, observable results, evidence paths, limitations, and cleanup here as work proceeds. Preserve the workstream's isolated dev-4 environment and avoid other workers' namespaces and ports.
