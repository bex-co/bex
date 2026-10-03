# w4 · m158 — Preserve exact SQL numbers in MCP responses

**Worker:** worker4 **Goal:** an agent receives the same exact SQL numbers as REST and the console **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Bound the SDK conversion and query-only compatibility path | 25m | — |
| t002 | Preserve exact query JSON through MCP serialization | 45m | t001 |
| t003 | Render parity and live result verification | 25m | t002 |
| t004 | Simplify | 15m | t003 |
| t005 | Test coverage | 35m | t003 |
| t006 | Closeout | 10m | t004, t005 |

## Definition of done

- Re-run the captured SELECT through REST, console and MCP: bigint `9007199254740993` and numeric `12345678901234567890.123456789` retain their exact values. MCP text content and structuredContent both carry exact JSON numbers; raw wire bytes, not a float-decoded client object, are the oracle.
- The captured bigint array retains both positive and negative exact values, and numeric fraction `0.12345678901234567890123456789` retains its digits. Small 42, 1.25, NULL, boolean, bytea and text controls keep their existing representations.
- Tool name/arguments and advertised QueryResult output shape remain stable. Typed input validation, workspace/auth guards, read-only execution, budgets, timeout, safe errors and panic isolation remain enforced; other tools keep their registration behavior.
- Meaningful SDK-boundary and composed-adapter tests pass. Live Free fixture probes succeed, then the fixture is deleted and its session revoked.

## Source + Goal linkage

- **Source:** user-requested live QA sweep 40 with muse.env, filing to w4; [complete evidence and dependency research](finding.md).
- **Goal linkage:** ADR006's shared REST/GraphQL/MCP semantics, ADR009 managed data, and ADR008's AI-native product: agents must not receive altered database identifiers or decimals.
- **Expected outcome:** preserve the exact finite numbers already produced by the core, without changing them into strings or rounding the other adapters to match MCP.
- **Why now:** live HTTP 200 responses silently replace integers and decimal digits in both MCP result representations. m157's shared collector normalization cannot fix this later SDK conversion; its live finite-MCP acceptance depends on this fix.
- **Render parity:** included; Render supports the query tool, but its exact numeric wire behavior was not measured. Do not assert an undocumented vendor guarantee.
- **Estimate:** 155m across six tasks. Scope is the query tool; a global SDK migration is not required.
