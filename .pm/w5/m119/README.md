# w5 · m119 — Env and secret-file writes share one rollback path

**Worker:** worker5 **Goal:** A failed projection or App patch after a single-key env or secret-file write leaves store, Secret and App consistent, without erasing a concurrent committed write. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Reproduce the two failure orders of a single-key env write | 45m | — |
| t002 | Make single-key setters reuse the batch compensation and delete projectOrRestore | 1h | t001 |
| t003 | Name the rejected key when a projection is refused | 20m | t002 |
| t004 | Render parity | 15m | t003 |
| t005 | Simplify | 15m | t004 |
| t006 | Test coverage | 30m | t004, t005 |
| t007 | Closeout | 10m | t006 |

## Definition of done

- A concurrent `SetEnvVar` committed between this call's CAS write and its failed projection survives.
- An App-patch failure after `upsertSecret` rolls back both the store and the Secret, leaving no mounted-but-unlisted value.
- A refused projection's 400 names the key and the rule.
- `projectOrRestore` is deleted, and single-key and batch paths share one compensation path.

## Source + Goal linkage

- **Source:** Last-24h code review, 2026-10-05, of `eb55d1e27` (w4/m168), which added `projectOrRestore` for single-key writes. The reviewer found both failure orders by reading the race and failure paths (medium confidence).
- **Goal linkage:** ADR006 env var API correctness; ADR008 reliability.
- **Expected outcome:** No lost env writes and no hidden mounted values.
- **Why now:** m168 just added this path, and w5/m118 t002 changes key validation in the same code.
- **Render parity included:** env var error shapes change on REST/GraphQL/MCP.
