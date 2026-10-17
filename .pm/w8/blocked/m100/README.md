# w8 · m100 — Supply SWIG for native C-binding builds

**Worker:** worker8 **Goal:** ordinary Python binding builds generate, compile and import their C extension using the documented native SWIG tool without tenant installation workarounds. **Status:** blocked — needs the user's choice of native tool-supply mechanism ([decision](../m91/design.md)); no task started

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [Supply reviewed native SWIG with library and typemap data](t001.md) | 60m | — |
| t002 | [Audit SWIG delivery across shared native bases and phases](t002.md) | 35m | t001 |
| t003 | [Verify native SWIG parity across authoring surfaces](t003.md) | 20m | t002 |
| t004 | [Simplify native binding-tool supplier changes](t004.md) | 15m | t003 |
| t005 | [Test generated-image binding generation, compilation and import](t005.md) | 50m | t003 |
| t006 | [Accept original first C-binding deployments and close milestone](t006.md) | 20m | t004, t005 |

**Total:**6 tasks/200m. [Researched finding](finding.md) · [complete evidence](evidence/cli-sweep421/README.md).

## Definition of done

A reviewed immutable multiarch SWIG payload with compatible executable/libraries/include/language/typemap data is usable through intended native build/pre-deploy/runtime/static phases. Real generated-image tests verify wrapper/proxy generation, gcc compilation/import and exact independent C arithmetic/Unicode results and source/generated/extension identities, reject an invalid interface and prove data-tree availability and network-disabled runtime generation. Audit intended bases/architectures/companions/env/PATH/cache/excluded paths. After publication both unchanged clients' original fresh first Free Python services become Live without a SWIG install and pass source/interface/extension/result/HTTP/log/shared-readback and exact cleanup/baseline/current-quota checks. Limits are explicit; install controls or code alone do not meet DoD.

## Source + Goal linkage

- **Source:** Authorized ordinary functional CLI sweep421,2026-10-17: both unchanged native creates POST201/exit0, both original first builds fail missing-swig, both one-line SWIG-install replacements Live with six real compiled-C-extension HTTP responses.
- **Goal:** [ADR008](../../../../docs/ADR008-vision.md), [Render parity ADR018](../../../../docs/ADR018-render-parity.md), [native assembly ADR060](../../../../docs/ADR060-build-worker-reliability-and-performance.md): regular native binding build tools should work as documented.
- **Outcome:** Native Python C-binding builds can generate their wrappers and run the resulting extension without rewriting commands or CLI requests.
- **Why now:** Real first failures and SWIG-only controls isolate missing delivery; source/board/history has no existing supplier repair. Required SWIG data trees and shared-base adoption need deliberate reviewed delivery.
- **Render parity:** Included because native tool contents affect tenant builds/runtime across REST/GraphQL/MCP/Blueprint/dashboard. No new API field, version selector or launcher fork.
