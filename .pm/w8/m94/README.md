# w8 · m94 — Supply SQLite CLI in native builds and runtime

**Worker:** worker8 **Goal:** ordinary native SQLite seed/import/export commands can use the supplied executable without replacing a working language library. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [Supply a reviewed native sqlite3 executable and dependencies](t001.md) | 70m | — |
| t002 | [Audit shared native consumers and preserve library and cache behavior](t002.md) | 35m | t001 |
| t003 | [Verify SQLite CLI Render parity across native authoring surfaces](t003.md) | 20m | t002 |
| t004 | [Simplify the changed SQLite CLI supplier](t004.md) | 15m | t003 |
| t005 | [Test real seed/import and readonly exports with healthy library controls](t005.md) | 40m | t003 |
| t006 | [Accept original fresh native defaults and close the milestone](t006.md) | 15m | t004, t005 |

**Total:**6 tasks/195m. [Finding](finding.md) · [complete evidence](evidence/cli-sweep409/README.md).

## Definition of done

A reviewed immutable multiarch sqlite3 executable with complete required libraries is available through intended native build/pre-deploy/runtime/static phases, preserving the healthy Python sqlite3 module and shared primary/companion/env/PATH/root/cache/exclusion behavior. Meaningful real generated-image tests pass. After publication, both unchanged native clients' fresh original first Free Python SQLite services reach Live without an install prefix and pass exact source/program/SQL, independent library/CLI three-row checks, database/file/runtime/native-log proofs and exact cleanup/baseline preservation and quota accounting. Shared authoring contracts stay compatible and unverified scope is explicit. Install controls or source code alone do not close the milestone.

## Source + Goal linkage

- **Source:** Authorized ordinary functional CLI sweep409,2026-10-17: both unchanged native POST201/exit0 first builds fail missing sqlite3 after healthy independent Python-library seed controls; both one-line install controls Live and six real readonly runtime exports pass.
- **Goal linkage:** [ADR008](../../../docs/ADR008-vision.md), [Render parity ADR018](../../../docs/ADR018-render-parity.md) and [native assembly ADR060](../../../docs/ADR060-build-worker-reliability-and-performance.md): ordinary hosting commands should work with named supplied utilities.
- **Expected outcome:** Native SQLite seed/import/export commands deploy and run without installing the executable in tenant code or rewriting CLI input.
- **Why now:** Both independent native clients and actual selected-base controls isolate the absent executable while the library and exact SQL/data work; there is no supplier or newer fix. PostgreSQL/Pandoc tool work has separate dependency and acceptance boundaries.
- **Render parity:** Included because native image contents affect tenant build/runtime behavior across REST/GraphQL/MCP/Blueprint/dashboard; no new schema field or CLI fork.
