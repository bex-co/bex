# w8 · m93 — Supply Pandoc in native builds and runtime

**Worker:** worker8 **Goal:** ordinary native document-generating apps can build and serve converted HTML using supplied Pandoc. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [Supply a reviewed native Pandoc distribution](t001.md) | 75m | — |
| t002 | [Audit all shared native consumers and cache boundaries](t002.md) | 35m | t001 |
| t003 | [Verify native document-tool Render parity](t003.md) | 20m | t002 |
| t004 | [Simplify the changed native document supplier](t004.md) | 15m | t003 |
| t005 | [Test actual document conversion and affected neighbors](t005.md) | 40m | t003 |
| t006 | [Accept fresh native defaults and close the milestone](t006.md) | 15m | t004, t005 |

**Total:**6 tasks/200m. [Finding](finding.md) · [complete evidence](evidence/cli-sweep407/README.md).

## Definition of done

A reviewed pinned multiarch Pandoc supplier runs real document conversions through intended native build/pre-deploy/runtime/static phases, with primary/companion/env/PATH/root/cache/exclusion regressions and actual generated-image tests. After publication, both unchanged native clients' fresh original first Free services reach Live without any install prefix, and exact source/program/document, independently parsed runtime HTML and native logs pass. Exact fixture/children/URLs/local cleanup and baseline/quota restoration pass. Shared authoring contracts stay compatible; unverified scope is explicit. Install controls or code alone do not close this milestone.

## Source + Goal linkage

- **Source:** Authorized ordinary functional CLI sweep407,2026-10-17: both unchanged native POST201/exit0 first builds fail missing Pandoc, both one-line install controls Live, six fresh runtime conversion/independent HTML checks pass.
- **Goal linkage:** [ADR008](../../../docs/ADR008-vision.md), [Render parity ADR018](../../../docs/ADR018-render-parity.md) and [native assembly ADR060](../../../docs/ADR060-build-worker-reliability-and-performance.md): native hosting commands should work with the named utility contract.
- **Expected outcome:** Markdown-based document generators deploy and convert at runtime without installing a supplied tool in app code or changing CLI requests.
- **Why now:** Both independent clients and actual selected-base controls isolate the missing program; source has no supplier or newer fix. Existing FFmpeg/PostgreSQL-client work does not provide Pandoc.
- **Render parity:** Included because a native image change affects tenant build/runtime behavior across REST/GraphQL/MCP/Blueprint/dashboard, while preserving input/output schemas.
