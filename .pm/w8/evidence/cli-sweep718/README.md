# CLI sweep 718 — static-only verbs re-check (live, 2026-10-19)

Disposable free web service `qa718-web` (`srv-dbbc1r5fsg2s73akoa20`). Deleted and verified absent (REST 404).

- `PUT /services/{id}/headers` and `/routes` on a web service → 400 `service "…" is not a static_site` (sweep 672 gate, unchanged).
- `bex services update <id>` with no update flag → client refusal `at least one update flag must be provided` before any request.

No bug filed.
