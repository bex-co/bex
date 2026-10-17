# w8 · m101 — Supply Webpack for native production bundles

**Worker:** worker8 **Goal:** ordinary native production bundling emits a working JavaScript service with a supplied noninteractive compiler/CLI, without tenant installation workarounds. **Status:** blocked — needs the user's choice of native tool-supply mechanism ([decision](../m91/design.md)); no task started

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [Supply reviewed Webpack, compatible CLI and dependency closure](t001.md) | 75m | — |
| t002 | [Audit bundler delivery across native bases and phases](t002.md) | 40m | t001 |
| t003 | [Verify Webpack native parity across authoring surfaces](t003.md) | 20m | t002 |
| t004 | [Simplify bundler supplier changes](t004.md) | 15m | t003 |
| t005 | [Test actual generated-image production bundles and failures](t005.md) | 55m | t003 |
| t006 | [Accept original first bundled services and close milestone](t006.md) | 20m | t004, t005 |

**Total:** 6 tasks / 225m. [Researched finding](finding.md) · [complete evidence](evidence/cli-sweep422/README.md).

## Definition of done

A reviewed immutable multiarch Webpack payload with compatible noninteractive CLI and complete dependency/data closure is usable through intended native build/pre-deploy/runtime/static phases. Actual generated-image tests verify multi-module production bundling, emitted-bundle execution, independent Unicode/data/module results, compiler statistics and source/output identities; reject unresolved modules and prove noninteractive network-disabled runtime compilation. Audit intended bases/architectures/companions/env/PATH/cache/excluded paths. After publication both unchanged clients' original first Free services become Live without an install line and pass source/module/bundle/results/HTTP/log/shared-readback and exact cleanup/baseline/current-quota checks. Limits are explicit; install controls or code alone do not meet DoD.

## Source + Goal linkage

- **Source:** Authorized ordinary functional CLI sweep422, 2026-10-17: both unchanged native creates POST 201/exit 0, both original first builds fail missing-webpack, both single pinned core/CLI-install replacements Live with six actual generated-bundle HTTP responses.
- **Goal:** [ADR008](../../../../docs/ADR008-vision.md), [Render parity ADR018](../../../../docs/ADR018-render-parity.md), [native assembly ADR060](../../../../docs/ADR060-build-worker-reliability-and-performance.md): normal native bundling commands should work as documented.
- **Outcome:** Native builds can bundle their modules and run the output without rewriting requests or adding tenant install workarounds.
- **Why now:** Actual first failures and supplier-only controls isolate missing compiler/CLI delivery; no existing source/board/history repair supplies it. Full dependency closure, engine compatibility and shared-base adoption require deliberate review.
- **Render parity:** Included because native tool contents affect tenant build/runtime behavior through REST/GraphQL/MCP/Blueprint/dashboard. No new API field, version selector or launcher fork.
