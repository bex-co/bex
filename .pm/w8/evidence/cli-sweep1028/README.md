# CLI sweep 1028 — strict query-parameter discipline (live, 2026-10-20)

One disposable free web service created and deleted (204). No new bug filed.

- An unknown query parameter on a service GET → coded 400 `unsupported query parameter "qaBogusParam" for retrieve-service` — the refusal names both the parameter and the operation. Even a plausible-looking `limit` on the single-object GET is refused the same way: query vocabularies are per-operation and enforced, not silently ignored.
