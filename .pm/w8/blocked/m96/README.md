# w8 · m96 — Supply GNU gettext for native message compilation

**Worker:** worker8 **Goal:** ordinary native internationalized apps can compile and use their message catalogs without installing promised system tools in tenant code. **Status:** blocked — needs the user's choice of native tool-supply mechanism ([decision](../m91/design.md)); no task started

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [Supply reviewed GNU gettext tools in native images](t001.md) | 70m | — |
| t002 | [Audit shared native bases, locale behavior and exclusions](t002.md) | 30m | t001 |
| t003 | [Verify GNU gettext Render parity across authoring surfaces](t003.md) | 20m | t002 |
| t004 | [Simplify the GNU gettext supplier](t004.md) | 15m | t003 |
| t005 | [Test real Django compilation and runtime catalog behavior](t005.md) | 45m | t003 |
| t006 | [Accept original native defaults and close the milestone](t006.md) | 15m | t004, t005 |

**Total:**6 tasks/195m. [Finding](finding.md) · [complete evidence](evidence/cli-sweep416/README.md).

## Definition of done

A reviewed immutable multiarch GNU gettext utility set and complete compatible library/data/locale closure are available through intended native build/pre-deploy/runtime/static phases. Real generated-image Django compilation, runtime external compilation, independent catalog parsing and exact Unicode/format/plural translations pass, with meaningful failures and audited sibling/architecture/phase/env/PATH/locale/cache/exclusion behavior. After publication both unchanged native clients' fresh original first Free services become Live without install lines, pass source/program/PO/catalog/translation/log/shared-readback checks and complete exact cleanup with baseline/current-inventory/quota reconciliation. Limits remain explicit; apt controls or source changes alone do not meet DoD.

## Source + Goal linkage

- **Source:** Authorized functional CLI sweep416,2026-10-17: both valid native POST201/exit0 first builds fail Django compilemessages missing-msgfmt after successful dependency install; both one-line gettext controls Live with exact real compiled-catalog and six runtime translation responses.
- **Goal:** [ADR008](../../../../docs/ADR008-vision.md), [Render parity ADR018](../../../../docs/ADR018-render-parity.md), [native assembly ADR060](../../../../docs/ADR060-build-worker-reliability-and-performance.md): regular hosting build/runtime dependencies work as documented.
- **Outcome:** Standard translation builds and message utilities run without rewriting tenant commands or CLI requests.
- **Why now:** Both native clients and exact selected-base controls isolate the missing GNU utility while Django and catalog data work. No matching supplier/fix exists; adjacent utility issues have different consumers/closure requirements.
- **Render parity:** Included because native image contents affect tenant builds/runtime through REST/GraphQL/MCP/Blueprint/dashboard; no new product control or launcher fork.
