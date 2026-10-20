# CLI sweep 706 — env-var entry fields (live, 2026-10-19)

Disposable free web service `qa706-web` (`srv-dbbbuh5fsg2s73ako9hg`). Deleted and verified absent (REST 404).

- `PUT /env-vars` with an `isSecret` field on an entry → 400 `request body contains unknown field "isSecret"`, and the whole PUT was refused (list stayed empty) — no per-entry secret flag on this surface (secrecy lives in secret files / blueprint `sync:false`).
- `PUT` with `{"key":"QA706_GEN","generateValue":true}` → 200; the server minted a 44-character base64 value (not recorded here) that read back on GET with the same length. Render's `generateValue` contract works.

No bug filed.
