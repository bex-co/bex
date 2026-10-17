# w8 · m99 — Supply TypeScript for native builds and runtime

**Worker:** worker8 **Goal:** ordinary strict TypeScript builds emit a working JavaScript service using the documented compiler without tenant installation workarounds. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [Supply a reviewed immutable native TypeScript compiler](t001.md) | 75m | — |
| t002 | [Audit compiler delivery across shared bases and native phases](t002.md) | 40m | t001 |
| t003 | [Verify TypeScript native parity across authoring surfaces](t003.md) | 20m | t002 |
| t004 | [Simplify compiler supplier changes](t004.md) | 15m | t003 |
| t005 | [Test actual generated-image TypeScript compilation and errors](t005.md) | 55m | t003 |
| t006 | [Accept original first TypeScript deployments and close milestone](t006.md) | 20m | t004, t005 |

**Total:**6 tasks/225m. [Researched finding](finding.md) · [complete evidence](evidence/cli-sweep420/README.md).

## Definition of done

A reviewed immutable multiarch compiler payload with compatible executable/libraries/standard-library/data closure is usable through intended native build/pre-deploy/runtime/static phases. Real generated-image tests verify strict interface/enum compilation and emitted-JavaScript execution with independent source/output/data identities, reject an actual type error without emitting output, and prove network-disabled runtime compilation. Audit intended bases/architectures/companions/env/PATH/cache/excluded paths. After publication both unchanged native clients' original fresh first Free services become Live without a compiler install and pass source/typed-result/HTTP/log/shared-readback/exact cleanup/baseline/current-quota checks. Limits are explicit; diagnostic controls or code alone do not meet DoD.

## Source + Goal linkage

- **Source:** Authorized ordinary functional CLI sweep420,2026-10-17: both unchanged native creates POST201/exit0, both original first builds fail missing-tsc, both single pinned-compiler-install replacements Live with six real emitted-JavaScript/typed-result responses.
- **Goal:** [ADR008](../../../docs/ADR008-vision.md), [Render parity ADR018](../../../docs/ADR018-render-parity.md), [native assembly ADR060](../../../docs/ADR060-build-worker-reliability-and-performance.md): regular hosting compiler commands should work as documented.
- **Outcome:** Native TypeScript builds and runtime scripts can use an actual compiler without changing CLI requests or adding tenant install workarounds.
- **Why now:** Real independent native first failures and compiler-only controls isolate missing compiler delivery; neither source nor board/history has an existing supplier repair. Native package/data layout and shared-base adoption require a deliberate reviewed implementation.
- **Render parity:** Included because native tool contents affect tenant build/runtime behavior across REST/GraphQL/MCP/Blueprint/dashboard. No new API field, version selector or launcher fork.
