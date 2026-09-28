# w7 · m158 — Repair recurring registry garbage-collection failures

**Worker:** worker7 **Goal:** Dependable push-to-deploy needs reclaimable registry storage. Diagnose the repeated failure before it becomes a capacity incident. **Status:** blocked

**Estimate:** 150m implementation; 220m including closing tasks. Runtime observation windows may exceed active effort.

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Reconcile repository metadata, blobs and deletion history — **DONE** | 45m | — |
| t002 | Implement the supported GC repair and prevention path | 60m | t001 |
| t003 | Verify repeated GC and normal registry operations — **DONE** | 45m | t002 |
| t004 | Simplify — **DONE** | 20m | t003 |
| t005 | Test coverage — **DONE** | 40m | t003 |
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

## Evidence (2026-09-28 UTC, worker7) — repair done, prevention parked

**Diagnosis (t001, production read-only):** Zot `v2.1.18`, one repository failing: `tea-daif693dqjvc73e7as3g/…-qa-20260917-a735c9-cron2-cache` (its App no longer exists). Every GC run logged `failed to get repoMeta … repo metadata not found for given repo name` from `removeTagsPerRetentionPolicy` (`pkg/storage/gc/gc.go:426`) and abandoned the repo, so its blobs were never collected (12 failures in the last 24h). Mechanism, from the Zot source at the deployed tag: App teardown deletes every manifest; the next GC past `gcDelay` normally removes the blobs **and** the repository. If Zot restarts first (the 2026-09-27T16:37Z restart; credential activation restarts it routinely), `parseStorage` rebuilds metaDB only from manifests on disk, so an empty repository gets **no** repo meta, and the retention step (policy `**` covers every repo) then fails forever. The same code is unchanged in `v2.1.21`, the latest release; no upstream issue or fix exists (searched). Leaked size: 11 blobs (9 + 2 repair blobs), bytes not measurable (no shell in the image, Zot metrics not scraped).

**Repair (t002 part 1), production 07:21Z:** as `bex-builder` (its existing `**` grant), through a port-forward, pushed one tiny artifact to the stranded repository and deleted its manifest by digest (`sha256:8881c817…`). Credential held only in a mode-0600 scratch file, deleted after. No filesystem access and no live manifest touched.

**Verified (t003):** GC runs for that repository after the repair: 07:22:12Z **success**, 9 blobs collected; 07:56:06Z **success**; 08:24:21Z **success**, "removed all blobs, removing repo", 2 blobs collected — the repository is gone. Zot logged **0** errors 07:44–08:24Z; live registry traffic in the last hour: 173 `GET 200`, 1 `GET 404`. Runbook recorded in ADR060 D4 (`7578df72c`).

**Why parked (t002 part 2, t006):** the DoD requires preventing recurrence under the same trigger, and that is a Zot defect (retention must treat missing repo meta as "no tags to retain", or `parseStorage` must create meta for empty repos). bex cannot prevent Zot restarts between teardown and GC. The options are all outside this session's authority: file the defect/PR upstream with project-zot (an outward-facing publication), or carry a patched Zot build. Meanwhile the ADR060 repair recovers any recurrence in minutes.
