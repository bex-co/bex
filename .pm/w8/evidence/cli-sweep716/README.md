# CLI sweep 716 — env-var key length boundary (live, 2026-10-19)

Disposable free web service `qa716-web` (`srv-dbbc13dfsg2s73ako9qg`). Deleted and verified absent (REST 404).

- Key of exactly 253 characters → 200.
- Key of 254 characters → 400 `ENVIRONMENT_VARIABLE_INVALID: environment variable name is longer than 253 characters` (with `params.field: "key"`).

The boundary matches the message sweep 647 recorded. No bug filed.
