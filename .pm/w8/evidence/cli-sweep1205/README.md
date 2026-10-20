# CLI sweep 1205 — GraphQL argument validation is exact (live, 2026-10-20)

One disposable free web service created and deleted (204, verified 404); the failed GraphQL mutation persisted nothing (name search empty). No new bug filed.

- `createService(input: …)` with a guessed input-object shape → 200 with two precise validation errors: unknown argument `input`, and missing required argument `name` of type `String!` — each with line/column. The mutation's flat-argument contract is enforced by schema validation before any resolver runs.
