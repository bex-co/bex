# CLI sweep 931 — exotic HTTP verbs (live, 2026-10-20)

Disposable free web service created, probed, deleted (204). No new bug filed.

- `OPTIONS` → 204 and `HEAD` → 200 with no body (Go's mux semantics: HEAD rides the GET handler), `TRACE` → 405 coded. Nothing exotic reaches a handler it should not.
