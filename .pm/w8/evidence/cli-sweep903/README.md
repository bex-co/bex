# CLI sweep 903 — log query enum/limit discipline (live, 2026-10-20)

Disposable free web service created, probed, deleted (204, then GET 404). No new bug filed.

- `direction=sideways903` → named 400 `direction must be one of forward|backward` (a closed enum, unlike sweep 902's open label values); `direction=forward` → 200.
- `limit=0` → the shared `limit must be an integer between 1 and 100` refusal; a `text=Server` content filter → 200.
