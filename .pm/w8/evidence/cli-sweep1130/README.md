# CLI sweep 1130 — GraphQL errors ride HTTP 200 with structured errors (live, 2026-10-20)

One disposable free web service created and deleted (204, verified 404). No new bug filed.

- Unknown field → 200 with `data: null` and a validation error naming the field and type (`Cannot query field "nopeField" on type "Service"`) with line/column.
- Malformed query → 200 with a syntax error carrying the offending position and source excerpt.
- Unknown service id → `data.service: null` plus a `NOT_FOUND` extension code (same shape as the after-delete read in sweep 1129).
- `{ __typename }` → `Query`: basic introspection responds.

GraphQL failures never leak into HTTP status codes; REST and GraphQL keep their own error idioms over the same store.
