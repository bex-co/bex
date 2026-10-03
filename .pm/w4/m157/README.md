# w4 · m157 — Preserve typed Postgres query values

**Worker:** worker4 **Goal:** SQL results preserve special numeric and temporal values, UUIDs and array shape across query surfaces **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Audit SQL value conversion and bound the shared contract | 25m | — |
| t002 | Normalize special numeric results before JSON budgeting | 45m | t001 |
| t007 | Preserve temporal infinity markers without changing integer values | 30m | t001, t002 |
| t008 | Render decoded UUIDs as canonical strings | 25m | t001, t002 |
| t009 | Preserve native array dimensions before value normalization | 45m | t001, t002 |
| t003 | Render parity and live result verification | 25m | t002, t007, t008, t009, w4/m158/t002 |
| t004 | Simplify | 15m | t003 |
| t005 | Test coverage | 40m | t003 |
| t006 | Closeout | 15m | t004, t005 |

## Definition of done

- The 2D integer, 3D integer and 2D text/null SELECTs in [array evidence](array-finding.md) retain nested JSON shape on REST/MCP and the equivalent nested JSON cell text in GraphQL/dashboard. Native matrices match PostgreSQL to_json controls; one-dimensional arrays, empty arrays, SQL NULL arrays and bytea controls retain their current representation. No custom-lower-bound metadata contract is asserted.

- Native UUIDs and UUID array elements are canonical lowercase hyphenated strings; NULL, bytea base64 and ordinary integer-array controls remain unchanged. See [UUID evidence](uuid-finding.md).

- Date/timestamp/timestamptz infinity markers return lowercase infinity/-infinity strings, including arrays; finite time and ordinary integer ±1 controls remain unchanged. See [temporal evidence](temporal-finding.md).

- SQL real/double NaN and both infinities return successful results on REST, GraphQL/dashboard and MCP, as canonical NaN/Infinity/-Infinity string values; numeric infinities retain their sign instead of becoming zero.
- Scalar GraphQL cells have no extra token quotes. Arrays containing special and finite values preserve each value under the existing array presentation contract.
- Finite float, exact Numeric/bigint (MCP precision prerequisite: w4/m158/t002), null, text and ordinary array controls retain their existing values/representations; bytea and finite-time semantics do not change.
- Shared normalization runs before the encoded result budget; raw/encoded caps, row truncation, transaction mode, writable confirmation, auth, timeout and safe SQL errors remain enforced. Local write-RETURNING coverage verifies the same conversion path without production table writes.
- Meaningful decoder/executor and adapter tests pass, and live Free Postgres probes match the captured expected behavior. Fixtures are deleted and the session revoked.

## Source + Goal linkage

- **Source:** user-requested continuous QA sweeps 37, 39, 40 and 44 with muse.env, filing to w4; [full evidence and researched fix](finding.md).
- **Goal linkage:** ADR009 managed databases and ADR006 shared API/MCP core; reliable data reads for humans and agents.
- **Expected outcome:** supported PostgreSQL typed values and native array shapes remain truthful and queryable through the product.
- **Why now:** live probes show both an outright valid-query failure and silent successful-result corruption through shared code. The five independently traced mechanisms need one explicit conversion contract before independent adapter fixes drift.
- **Render parity:** included for REST/GraphQL/MCP/UI consistency; vendor special-number encoding remains unverified and is not invented.
- **Estimate:** 265m across nine tasks, including consumer audit and required closing tasks.
