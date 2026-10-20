# CLI sweep 744 — env-group PATCH and per-key routes (live, 2026-10-19)

Fixture `qa744-group` deleted (204) and list-verified absent.

- `PATCH /env-groups/{id} {"name":…}` → 200 rename; repeating the same name → 200 (no self-conflict). An unknown-field body → 400 `invalid request body at /name` (the PATCH schema carries only `name`).
- Per-key routes work: `PUT /env-groups/{id}/env-vars/QA744_K {"value":"v1"}` → 200 `{"key":"QA744_K"}` (value write-only on write); `GET …/env-vars/QA744_K` → 200 with the value for the authorized owner; `DELETE` → 204.

No bug filed.
