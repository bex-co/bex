# w8 · m98 — Supply jq and rsync for native asset builds

**Worker:** worker8 **Goal:** normal native JSON-manifest and local asset-copy builds run with their documented utilities without tenant installation workarounds. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [Supply a reviewed native jq executable and dependencies](t001.md) | 50m | — |
| t002 | [Supply a reviewed native rsync executable and dependencies](t002.md) | 70m | t001 |
| t003 | [Audit shared native bases, phases and both utility consumers](t003.md) | 35m | t001, t002 |
| t004 | [Verify jq and rsync Render parity across native inputs](t004.md) | 20m | t003 |
| t005 | [Simplify the two-utility supplier changes](t005.md) | 15m | t004 |
| t006 | [Test generated-image manifest generation and local copying](t006.md) | 60m | t004 |
| t007 | [Accept original first asset deployments and close the milestone](t007.md) | 20m | t005, t006 |

**Total:**7 tasks/270m. [Two independently isolated omissions](finding.md) · [complete evidence](evidence/cli-sweep419/README.md).

## Definition of done

Reviewed immutable multiarch jq and rsync payloads with compatible required libraries/data are usable through intended native build/pre-deploy/runtime/static phases. Real generated-image tests independently catch each missing tool and verify exact combined JSON-manifest/local-copy/Unicode/space-name/byte/exclude/stale-own-output behavior, network-disabled runtime and meaningful failures. Audit intended bases/architectures/companions/env/PATH/root/cache/excluded paths. After publication both unchanged native clients' original fresh first Free Node services become Live without any install line and pass real source/program/data/manifest/copy/HTTP/log/shared-readback and exact cleanup/baseline/current-quota checks. Limits are explicit; staged apt controls or code alone do not meet DoD.

## Source + Goal linkage

- **Source:** Authorized ordinary functional CLI sweep419,2026-10-17: both original native POST201/exit0 first builds fail missing-jq; both jq-only builds generate correct manifests then fail missing-rsync; both complete-tool replacements Live with six real manifest/file-copy/HTML responses.
- **Goal:** [ADR008](../../../docs/ADR008-vision.md), [Render parity ADR018](../../../docs/ADR018-render-parity.md), [native assembly ADR060](../../../docs/ADR060-build-worker-reliability-and-performance.md): regular hosting build/runtime tools should work as documented.
- **Outcome:** Native asset builds and runtime scripts can filter JSON and copy local files without rewriting commands or CLI requests.
- **Why now:** Actual independent native stages plus selected-base controls isolate two missing executables in one ordinary asset workflow; there is no supplier/newer fix. Shared delivery/audit/acceptance is coherent, while each missing tool has its own implementation and regression task.
- **Render parity:** Included because native image contents affect tenant builds/runtime through REST/GraphQL/MCP/Blueprint/dashboard. No new API field, version selector or launcher fork.
