# w6 · m164 — Native-common qualification evidence restored from the pre-purge bundle into object storage

**Worker:** worker6 **Goal:** native-common qualification reads its candidate evidence again, from object storage verified by sha256, with no evidence in git. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [Extract every evidence file the test candidate references from the backup bundle and verify it](t001.md) | 40m | — |
| t002 | [Upload to object storage and commit a path → sha256/URL manifest](t002.md) | 50m | t001 |
| t003 | [evidence_file() fetches a missing path from the manifest and verifies its sha256](t003.md) | 60m | t002 |
| t004 | [Dispatch native-common and confirm plan/qualify read the evidence](t004.md) | 30m | t003 |
| t005 | [Simplify](t005.md) | 15m | t004 |
| t006 | [Test coverage](t006.md) | 20m | t005 |
| t007 | [Closeout](t007.md) | 10m | t006 |

## Definition of done

- native-common's plan and qualify jobs read every candidate evidence file (dispatch run cited) with none in git; `repo-artifact-guard` stays green.
- A tampered evidence object is refused by its sha256.

## Source + Goal linkage

- **Source:** `/pm-brainstorm w6` 2026-11-07, proposal 1; check against `b09eaf24b`: 10 of 10 candidate evidence paths missing after `dcdc774c1`.
- **Goal linkage:** ADR008 reliable delivery and w8/m91's native image custody chain.
- **Expected outcome:** native image publication can qualify again instead of failing at the first evidence read.
- **Why now:** it is already broken; the next native publish hits it. Touches w8's qualification code: coordinate with w8.
- **Render parity:** omitted — CI/infrastructure only, no REST/GraphQL/MCP/UI surface.
