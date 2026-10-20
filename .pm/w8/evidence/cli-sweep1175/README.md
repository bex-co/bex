# CLI sweep 1175 — response headers stay minimal (live, 2026-10-20)

One disposable free web service created and deleted (204, verified 404). No new bug filed.

- A service GET returns `Content-Type: application/json` and `Vary: Origin` (the CORS discipline from sweep 1086), with no `ETag` and no `Cache-Control` — matching the no-conditional-request behavior recorded in sweep 1110: responses are never cache-validated, so clients always re-read live state.
