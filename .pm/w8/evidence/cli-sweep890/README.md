# CLI sweep 890 — unknown query parameters refuse by operation (live, 2026-10-20)

Disposable free web service created, probed, deleted (204, then GET 404). No new bug filed.

- `GET /services/{id}?bogus890=1` → 400 `unsupported query parameter "bogus890" for retrieve-service`; the same parameter on the collection → `…for list-services`. The refusal names both the parameter and the operation it was sent to (the strict query discipline also seen on log-values in sweep 828), so a misspelled filter can never silently widen or narrow a result set.
