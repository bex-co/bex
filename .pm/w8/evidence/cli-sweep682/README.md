# CLI sweep 682 — authenticated-user endpoint (live, 2026-10-19)

Read-only.

- `GET /users` → 200 with the QA account's email/id (empty `name`, same as sweep 662's `whoami`).
- `GET /users?bogusParam=1` → 400 `unsupported query parameter "bogusParam" for get-user` (validator names the operation).
- `POST /users` → 405 `method not allowed`.

No bug filed.
