# CLI sweep 1044 — HTTP method discipline on the service resource (live, 2026-10-20)

One disposable free web service created and deleted (204). No new bug filed.

- `HEAD` on a service → 200 (headers-only read), `OPTIONS` → 204, and `PUT` (not in the service contract — PATCH is the update verb) → 405 method-not-allowed. Verbs outside the registered set fail closed.
