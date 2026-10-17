# w8 · m107 — Supply ZIP creation in native builds and runtimes

**Worker:** worker8 **Goal:** native builds and download handlers create correct ZIP archives without an authored installer. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [Supply reviewed ZIP creation tools and required libraries](t001.md) | 45m | — |
| t002 | [Audit ZIP supply across native bases and phases](t002.md) | 40m | t001 |
| t003 | [Verify ZIP availability parity across hosting surfaces](t003.md) | 20m | t002 |
| t004 | [Simplify ZIP supplier changes](t004.md) | 15m | t003 |
| t005 | [Test generated-image ZIP creation and independent decoding](t005.md) | 50m | t003 |
| t006 | [Accept original first ZIP services and close milestone](t006.md) | 20m | t004, t005 |

**Total:** 6 tasks / 190m. [Researched finding](finding.md) · [complete evidence](evidence/cli-sweep434/README.md).

## Definition of done

Reviewed immutable ZIP contents and compatible required libraries are available before authored native builds and remain usable in the resulting compute runtime. Real generated-image tests create build/runtime archives and independently verify exact unique member names/bytes/sizes/CRC32 and stored/deflated methods for empty/binary/Unicode/space-named files, including meaningful errors. Audit intended primary lines/architectures/companions/PATH/env/cache and existing excluded paths. After publication both unchanged native clients' original first Free Node services become Live without installing ZIP and pass exact source/archive/JSON/raw-HTTP/native-log/shared-readback checks. Exact fixtures/children/URLs are removed, baseline preserved and current quotas reconciled. Implementation or install-only controls do not satisfy DoD; execution limits remain explicit.

## Source + Goal linkage

- **Source:** Authorized ordinary functional CLI sweep434, 2026-10-17: both unchanged creates POST 201/native 0; first builds fail missing ZIP; install-only controls create genuine build/runtime ZIPs verified independently through four JSON responses and two raw downloads.
- **Goal:** [ADR008](../../../docs/ADR008-vision.md), [Render parity ADR018](../../../docs/ADR018-render-parity.md), [native build assembly ADR060](../../../docs/ADR060-build-worker-reliability-and-performance.md): documented native tools work in ordinary hosting journeys.
- **Expected outcome:** Native asset builds and download handlers can create ZIP archives without tenant install workarounds.
- **Why now:** Actual missing-tool failures plus supplier-only controls isolate an uncovered native-image omission. Complete package/library delivery and shared base/phase/architecture checks require reviewed implementation and real artifact tests.
- **Render parity:** Included because native image contents affect tenant-facing hosting through REST/GraphQL/MCP/Blueprint/dashboard. Preserve existing fields, commands, source/env and CLI wire contracts; no new selector or client fork is needed.
