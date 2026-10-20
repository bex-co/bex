# CLI sweep 648 — suspend/resume lifecycle (live, 2026-10-19)

Same disposable fixture as sweep 647 (`qa647-web`, `srv-dbbb8h2vfpec73dm0j50`), deleted after this sweep and verified absent (REST 404).

- `POST /services/{id}/suspend` and `/resume` with a `{}` body: `400 request body is not allowed for this operation` (strict no-body contract).
- With no body: suspend → 202, `GET` shows `"suspended":"suspended"`; second suspend → 202 (idempotent). Public URL while suspended → HTTP 503.
- Resume → 202, `"suspended":"not_suspended"`; second resume → 202 (idempotent).
- Synthetic id `srv-00000000000000000000000000` suspend → 404 `NOT_FOUND`.

Lifecycle, idempotency, and not-found handling are consistent. No bug filed.
