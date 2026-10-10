# w4 · m157 — Preserve typed Postgres query values

**Worker:** worker4 **Goal:** SQL results preserve special numeric and temporal values, UUIDs, array shape and exact JSON values across query surfaces **Status:** blocked — t001/t002/t004/t005/t007/t008/t009 done 2026-10-03; t010–t013 JSON follow-up and t003 live result verification/t006 closeout remain; earlier completed tasks retain their original scope

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 — **DONE** | Audit SQL value conversion and bound the shared contract | 25m | — |
| t002 — **DONE** | Normalize special numeric results before JSON budgeting | 45m | t001 |
| t007 — **DONE** | Preserve temporal infinity markers without changing integer values | 30m | t001, t002 |
| t008 — **DONE** | Render decoded UUIDs as canonical strings | 25m | t001, t002 |
| t009 — **DONE** | Preserve native array dimensions before value normalization | 45m | t001, t002 |
| t010 | Audit JSON query consumers and bound codec registration | 30m | t001 |
| t011 | Preserve exact JSON values before query result decoding | 45m | t010 |
| t003 | Render parity and live result verification | 25m | t002, t007, t008, t009, t011, w4/m158/t002 |
| t004 — **DONE** | Simplify | 15m | t003 |
| t005 — **DONE** | Test coverage | 40m | t003 |
| t012 | Simplify the JSON value follow-up | 15m | t003 |
| t013 | Test JSON decoding and existing query consumers | 35m | t003 |
| t006 | Closeout | 15m | t004, t005, t012, t013 |

## Definition of done

- Re-run the two fresh-page JSONB SELECTs and the JSON/native-numeric sibling SELECT in [JSON evidence](json-finding.md): object and JSONB-array numbers retain `9007199254740993` and `12345678901234567890.123456789`; the `json` number remains exact. GraphQL/dashboard scalar JSON null is the non-null text `null`, a JSON string keeps its quote characters, and actual SQL NULL remains nullable/`NULL`. Native bigint/Numeric, float `1.5`, false and text-cast controls retain their captured values. JSON object key order and whitespace are not guaranteed. REST/MCP JSON behavior and additional array/format cases remain verification work in t003/t013 until exercised.

- The 2D integer, 3D integer and 2D text/null SELECTs in [array evidence](array-finding.md) retain nested JSON shape on REST/MCP and the equivalent nested JSON cell text in GraphQL/dashboard. Native matrices match PostgreSQL to_json controls; one-dimensional arrays, empty arrays, SQL NULL arrays and bytea controls retain their current representation. No custom-lower-bound metadata contract is asserted.

- Native UUIDs and UUID array elements are canonical lowercase hyphenated strings; NULL, bytea base64 and ordinary integer-array controls remain unchanged. See [UUID evidence](uuid-finding.md).

- Date/timestamp/timestamptz infinity markers return lowercase infinity/-infinity strings, including arrays; finite time and ordinary integer ±1 controls remain unchanged. See [temporal evidence](temporal-finding.md).

- SQL real/double NaN and both infinities return successful results on REST, GraphQL/dashboard and MCP, as canonical NaN/Infinity/-Infinity string values; numeric infinities retain their sign instead of becoming zero.
- Scalar GraphQL cells have no extra token quotes. Arrays containing special and finite values preserve each value under the existing array presentation contract.
- Finite float, exact Numeric/bigint (MCP precision prerequisite: w4/m158/t002), null, text and ordinary array controls retain their existing values/representations; bytea and finite-time semantics do not change.
- Shared normalization runs before the encoded result budget; raw/encoded caps, row truncation, transaction mode, writable confirmation, auth, timeout and safe SQL errors remain enforced. Local write-RETURNING coverage verifies the same conversion path without production table writes.
- Meaningful decoder/executor and adapter tests pass, and live Free Postgres probes match the captured expected behavior. Fixtures are deleted and the session revoked.

## Source + Goal linkage

- **Source:** user-requested continuous QA sweeps 37, 39, 40, 44 and pass 51 (2026-10-10) with muse.env, filing to w4; [full evidence and researched fix](finding.md), plus [lossless JSON follow-up](json-finding.md).
- **Goal linkage:** ADR009 managed databases and ADR006 shared API/MCP core; reliable data reads for humans and agents.
- **Expected outcome:** supported PostgreSQL typed values and native array shapes remain truthful and queryable through the product.
- **Why now:** live probes show both an outright valid-query failure and silent successful-result corruption through shared code. The six independently traced mechanisms need one explicit conversion contract before independent adapter fixes drift.
- **Render parity:** included for REST/GraphQL/MCP/UI consistency; vendor special-number encoding remains unverified and is not invented.
- **Estimate:** 390m across thirteen tasks (125m new JSON work), including consumer audit and required closing tasks.
