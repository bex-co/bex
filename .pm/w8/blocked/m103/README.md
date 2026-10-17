# w8 · m103 — Supply native Python packaging and development tools

**Worker:** worker8 **Goal:** ordinary native Node builds can generate Python package metadata, install an owned wheel and compile/import their Python C extension with the documented prerequisites, without tenant install workarounds. **Status:** blocked — needs the user's choice of native tool-supply mechanism ([decision](../m91/design.md)); no task started

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [Supply compatible native Python setuptools and metadata tools](t001.md) | 50m | — |
| t002 | [Supply compatible native pip and target wheel installation](t002.md) | 60m | t001 |
| t003 | [Supply matching Python development headers and compiler inputs](t003.md) | 65m | t002 |
| t004 | [Audit all three prerequisites across shared native bases and phases](t004.md) | 40m | t001, t002, t003 |
| t005 | [Verify Python prerequisite parity across native authoring surfaces](t005.md) | 20m | t004 |
| t006 | [Simplify coherent Python-tool supplier changes](t006.md) | 15m | t005 |
| t007 | [Test generated-image metadata, wheel and C-extension behavior](t007.md) | 65m | t005 |
| t008 | [Accept original first integrated services and close milestone](t008.md) | 20m | t006, t007 |

**Total:** 8 tasks / 335m. [Three independently isolated omissions](finding.md) · [complete evidence](evidence/cli-sweep424/README.md).

## Definition of done

Reviewed immutable multiarch setuptools, pip and matching development-header/config/library payloads work coherently with each intended native base/primary Python through applicable build/pre-deploy/runtime/static phases. Actual generated-image tests independently catch each missing prerequisite, then execute real metadata generation, local-only target wheel installation, C compilation/import and Node/Python HTTP behavior with independent data/source/module/extension identities, meaningful failures and network-disabled artifact use. Audit intended bases/architectures/companions/env/PATH/PYTHONPATH/cache/excluded paths while preserving already-working primary Python and existing tools/libraries. After publication both unchanged native clients' original fresh first Free Node services become Live without any install line and pass complete HTTP/log/shared-readback/exact cleanup/baseline/current-quota checks. Limits are explicit; a combined diagnostic install or code alone does not meet DoD.

## Source + Goal linkage

- **Source:** Authorized ordinary functional CLI sweep424, 2026-10-17: both original first builds fail missing setuptools; both setuptools-only builds prove metadata then fail missing pip; both packaging builds prove local wheel installation then fail missing Python.h; both complete-prerequisite fourth deployments Live with six real integrated HTTP responses.
- **Goal:** [ADR008](../../../../docs/ADR008-vision.md), [Render parity ADR018](../../../../docs/ADR018-render-parity.md), [native assembly ADR060](../../../../docs/ADR060-build-worker-reliability-and-performance.md): native package and extension builds should work with their documented preloaded tools.
- **Outcome:** Node/Python integrations can build and use their genuine modules without rewriting supported requests or installing platform-promised prerequisites themselves.
- **Why now:** Four real native stages plus selected-base application controls independently isolate three missing prerequisites in one workflow; no matching supplier repair exists. Coherent per-base Python ABI/distro delivery and a shared acceptance/audit are useful together, while each omission has its own implementation/regression task.
- **Render parity:** Included because native tool contents affect tenant build/runtime behavior through REST/GraphQL/MCP/Blueprint/dashboard. Preserve existing fields and runtime selectors.
