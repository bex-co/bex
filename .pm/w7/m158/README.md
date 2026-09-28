# w7 · m158 — Repair recurring registry garbage-collection failures

**Worker:** worker7 **Goal:** Dependable push-to-deploy needs reclaimable registry storage. Diagnose the repeated failure before it becomes a capacity incident. **Status:** todo

**Estimate:** 150m implementation; 220m including closing tasks. Runtime observation windows may exceed active effort.

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Reconcile repository metadata, blobs and deletion history | 45m | — |
| t002 | Implement the supported GC repair and prevention path | 60m | t001 |
| t003 | Verify repeated GC and normal registry operations | 45m | t002 |
| t004 | Simplify | 20m | t003 |
| t005 | Test coverage | 40m | t003 |
| t006 | Closeout | 10m | t004, t005 |

## Definition of done

- [ ] The failing state and its ownership are documented with a bounded reproduction; storage leakage is measured or explicitly unproven.
- [ ] The reproduced GC failure clears without deleting live manifests or weakening repository isolation; the correction prevents recurrence under the same trigger.
- [ ] Two GC cycles succeed, live images remain usable, the disposable repository is cleaned up and before/after evidence identifies the tested revision.

- [ ] Required checks and closing tasks are complete, with dated runtime evidence for production-dependent outcomes.

## Source + Goal linkage

- **Source:** user-approved second platform-log brainstorm, 2026-09-28; `$pm all for w7`. Observations were captured against 016391810; materialization checkout is 242fe0c83. Revalidate before implementation.
- **Evidence:** The September 28 UTC capture contains 21 failed Zot GC runs between September 27 05:18 and September 28 03:44 UTC for tea-daif693dqjvc73e7as3g/tea-daif693dqjvc73e7as3g-qa-20260917-a735c9-cron2-cache: repo metadata not found for given repo name. Repeated GC failure is established; leaked storage is not yet proven.
- **Local evidence:** `/tmp/bex-platform-log-audit/` (manifest, events, node snapshots, container logs and Loki history). Temporary files can expire; the sanitized finding is preserved here.
- **Goal linkage:** ADR008 reliable self-hosted hosting and deterministic platform operation. Dependable push-to-deploy needs reclaimable registry storage. Diagnose the repeated failure before it becomes a capacity incident.
- **Expected outcome:** Two GC cycles succeed, live images remain usable, the disposable repository is cleaned up and before/after evidence identifies the tested revision.
- **Why now:** Dependable push-to-deploy needs reclaimable registry storage. Diagnose the repeated failure before it becomes a capacity incident.
- **Deduplication:** Follow-up to shipped w7/m86 cache repositories and deletion; this owns the observed GC failure, not cache enablement (047), registry migration, or the documented credential-activation restart mechanism.
- **Render parity omitted:** internal registry/platform monitoring only; no tenant-facing REST/GraphQL/MCP/UI contract change.

## Scheduling and boundaries

Approved second-round priority 1; keep existing m154–m157 work intact. The first round's root GitOps blocker m153 is now done; verify current sync before relying on it. These milestones do not depend on one another, although changes to shared Prometheus configuration must be coordinated. Implementation uses isolated dev-7 where applicable; do not disturb other workstreams' stacks. This filing performs no production mutation.
