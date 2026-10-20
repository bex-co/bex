# CLI sweep 776 — Key Value persistence-mode cycle (live, 2026-10-19)

Disposable free Key Value `qa776-kv` (`red-dbbcq9d4c21c738t03t0`), created with `--persistence-mode off`. Deleted and verified absent (zero `qa776` in list).

- `PATCH {"persistenceMode":"snapshot"}` → 200, read back `snapshot` once `available`.
- `PATCH {"persistenceMode":"bogus"}` → 400 `invalid request body at /persistenceMode`.

No bug filed.
