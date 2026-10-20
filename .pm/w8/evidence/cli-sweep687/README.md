# CLI sweep 687 — datastore PATCH validation (live, 2026-10-19)

Refusal-only probes against the baseline Postgres (never mutated) and a synthetic Key Value.

- `PATCH /postgres/{id} {"plan":"bogus"}` → 400 `invalid request body at /plan`.
- `{"bogusField":1}` → 400 `request body contains unknown field "bogusField"`.
- `{"name":""}` → 400 `RESOURCE_NAME_INVALID: name must use lowercase letters, digits, and hyphens, be at most 30 characters, and not start or end with a hyphen`.
- `PATCH /key-value/{synthetic}` → 404 `NOT_FOUND`.

No bug filed.
