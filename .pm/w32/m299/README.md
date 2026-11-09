# w32 · m299 — Remaining Render SDK gaps: relative remote paths and mint during resume

**Worker:** worker32 **Goal:** the two residuals the m284 static review found are fixed, so the SDK walk can grade every method green **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Verify Render's behavior for both residuals | 30m | — |
| t002 | Accept relative remote paths and resolve them under the exec user's home | 60m | t001 |
| t003 | Mint answers at once against a suspended sandbox | 60m | t001 |
| t004 | Docs: remove the residuals | 15m | t002, t003 |
| t005 | Render parity | 20m | t004 |
| t006 | Simplify | 15m | t005 |
| t007 | Test coverage | 30m | t005 |
| t008 | Closeout | 10m | t006, t007 |

## Definition of done

- On a dev or production stack, the unmodified Python SDK's `copy_to("f.txt", "rel/f.txt")` lands in the sandbox user's `$HOME/rel/f.txt`, and `copy_from` reads it back.
- An SDK `exec` against a suspended sandbox succeeds with default timeouts.
- Both residual lines are gone from the checklist.

## Source + Goal linkage

- **Source:** `/pm-brainstorm more for w32`, 2026-11-09, item 3. The residuals recorded by w32/m284 t001's static review (`docs/cli-compatibility-checklist.md` § Render SDK, "Known residuals").
- **Goal linkage:** Pillar 5 sandboxes and Render parity (ADR008): moving SDK code to bex should need only a base-URL change.
- **Expected outcome:** SDK scripts using relative paths, or touching a suspended sandbox, work unchanged.
- **Why now:** These are the only known failures the m284 live walk would hit. Fixing them first lets that walk grade every method green once the QA sign-in is back; this milestone doesn't need that sign-in.
- **Render parity:** Included: sandbox file and exec behavior on REST changes, and GraphQL/MCP must stay aligned. Dashboard N/A.
