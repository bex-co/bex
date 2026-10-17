# w8 · m97 — Supply Ghostscript for native PDF rendering

**Worker:** worker8 **Goal:** ordinary native PDF-processing apps can invoke the promised renderer without tenant installation workarounds. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [Supply reviewed native Ghostscript with its runtime resources](t001.md) | 80m | — |
| t002 | [Audit shared native bases, phases, resources and exclusions](t002.md) | 35m | t001 |
| t003 | [Verify Ghostscript native Render parity and authoring inputs](t003.md) | 20m | t002 |
| t004 | [Simplify the Ghostscript supplier](t004.md) | 15m | t003 |
| t005 | [Test actual generated-image PDF conversion and resource lookup](t005.md) | 50m | t003 |
| t006 | [Accept original first PDF deployments and close the milestone](t006.md) | 15m | t004, t005 |

**Total:**6 tasks/215m. [Finding](finding.md) · [complete evidence](evidence/cli-sweep418/README.md).

## Definition of done

A reviewed immutable multiarch Ghostscript supplier with compatible executable/library/init/resource/font/color data is available through intended native build/pre-deploy/runtime/static phases. Meaningful actual generated-image PDF/pixel and resource lookup checks pass with nonzero error propagation and audited architecture/base/companion/env/GS_LIB/PATH/locale/cache/exclusion behavior. After publication both unchanged native clients' fresh original first Free services become Live without install lines and pass exact source/program/PDF/rendered-image/native-log/shared-readback and exact cleanup/baseline/current-quota checks. Limits are documented; apt controls or code alone do not meet DoD.

## Source + Goal linkage

- **Source:** Authorized ordinary functional CLI sweep418,2026-10-17: both unchanged native POST201/exit0 first builds fail missing-gs, both one-line Ghostscript controls Live with six real PDF conversions and independently checked PNG pixels.
- **Goal:** [ADR008](../../../docs/ADR008-vision.md), [Render parity ADR018](../../../docs/ADR018-render-parity.md), [native assembly ADR060](../../../docs/ADR060-build-worker-reliability-and-performance.md): regular hosting build/runtime tools work as documented.
- **Outcome:** Native PDF conversion commands run without rewriting tenant dependencies or CLI requests.
- **Why now:** Both native clients and exact selected-base controls isolate the missing renderer with real source/PDF/pixel proofs; no supplier or newer fix exists. Adjacent image/media utility issues have separate consumers and closure requirements.
- **Render parity:** Included because image contents affect tenant build/runtime through REST/GraphQL/MCP/Blueprint/dashboard; no new API field or launcher fork.
