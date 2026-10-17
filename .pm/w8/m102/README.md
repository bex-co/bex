# w8 · m102 — Supply pigz for native asset builds

**Worker:** worker8 **Goal:** native build scripts can compress their assets with the documented build-time pigz tool and publish correct gzip bytes without tenant installation workarounds. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [Supply reviewed build-time pigz and required libraries](t001.md) | 50m | — |
| t002 | [Audit build-time compression across native bases and phases](t002.md) | 35m | t001 |
| t003 | [Verify build-only pigz parity across authoring surfaces](t003.md) | 20m | t002 |
| t004 | [Simplify compression-tool supplier changes](t004.md) | 15m | t003 |
| t005 | [Test generated-image compression and published gzip bytes](t005.md) | 50m | t003 |
| t006 | [Accept original first compressed-asset services and close milestone](t006.md) | 20m | t004, t005 |

**Total:** 6 tasks / 190m. [Researched finding](finding.md) · [complete evidence](evidence/cli-sweep423/README.md).

## Definition of done

A reviewed immutable multiarch pigz payload with compatible required libraries is usable before intended native authored web/static builds. Real generated-image tests verify actual compression and independent exact gzip inflation/header/CRC32/ISIZE/output identity for empty/binary/Unicode data and meaningful errors. Native service/static publication serves the built bytes without requiring runtime pigz. Audit intended bases/architectures/companions/env/PATH/cache/excluded paths. After publication both unchanged clients' original first Free Python services become Live without an install line and pass source/input/artifact/HTTP/log/shared-readback and exact cleanup/baseline/current-quota checks. Limits are explicit; install controls or code alone do not meet DoD.

## Source + Goal linkage

- **Source:** Authorized ordinary functional CLI sweep423, 2026-10-17: both unchanged native creates POST 201/exit 0, both original first builds fail missing-pigz, both one-line install replacements Live with four JSON artifact proofs and two independently verified raw gzip downloads.
- **Goal:** [ADR008](../../../docs/ADR008-vision.md), [Render parity ADR018](../../../docs/ADR018-render-parity.md), [native assembly ADR060](../../../docs/ADR060-build-worker-reliability-and-performance.md): normal native asset build tools should work as documented.
- **Outcome:** Native builds can compress and publish owned assets without rewriting commands or CLI requests.
- **Why now:** Actual original failures and supplier-only controls isolate a missing build tool; source/board/history has no matching repair. Compatible shared-base delivery and exact publication behavior need reviewed implementation and real artifact tests.
- **Render parity:** Included because native build contents affect tenant hosting behavior through REST/GraphQL/MCP/Blueprint/dashboard. Pigz is documented as build-only; no runtime tool, performance guarantee, new API field or CLI fork is required.
