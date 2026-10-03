# w4 · m157 — Preserve Postgres special numeric query values

**Worker:** worker4 **Goal:** SQL results preserve NaN and signed infinities across query surfaces **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Audit SQL numeric conversion and bound the shared contract | 25m | — |
| t002 | Normalize special numeric results before JSON budgeting | 45m | t001 |
| t003 | Render parity and live result verification | 25m | t002 |
| t004 | Simplify | 15m | t003 |
| t005 | Test coverage | 40m | t003 |
| t006 | Closeout | 15m | t004, t005 |

## Definition of done

- SQL real/double NaN and both infinities return successful results on REST, GraphQL/dashboard and MCP, as canonical NaN/Infinity/-Infinity string values; numeric infinities retain their sign instead of becoming zero.
- Scalar GraphQL cells have no extra token quotes. Arrays containing special and finite values preserve each value under the existing array presentation contract.
- Finite float, exact Numeric/bigint, null, text and ordinary array controls retain their existing values/representations; bytea/time semantics do not change.
- Shared normalization runs before the encoded result budget; raw/encoded caps, row truncation, transaction mode, writable confirmation, auth, timeout and safe SQL errors remain enforced. Local write-RETURNING coverage verifies the same conversion path without production table writes.
- Meaningful decoder/executor and adapter tests pass, and live Free Postgres probes match the captured expected behavior. Fixtures are deleted and the session revoked.

## Source + Goal linkage

- **Source:** user-requested continuous QA sweep 37 with muse.env, filing to w4; [full evidence and researched fix](finding.md).
- **Goal linkage:** ADR009 managed databases and ADR006 shared API/MCP core; reliable data reads for humans and agents.
- **Expected outcome:** supported PostgreSQL numeric values remain truthful and queryable through the product.
- **Why now:** live probes show both an outright valid-query failure and silent successful-result corruption through shared code. The two mechanisms need one explicit conversion contract before independent adapter fixes drift.
- **Render parity:** included for REST/GraphQL/MCP/UI consistency; vendor special-number encoding remains unverified and is not invented.
- **Estimate:** 165m across six tasks, including consumer audit and required closing tasks.
