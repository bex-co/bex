# CLI sweep 888 — action endpoints reject bodies; delete is state-independent (live, 2026-10-20)

Disposable free web service created, probed, deleted (204, then GET 404). No new bug filed.

- `POST …/suspend` **with** a JSON body → named 400 `request body is not allowed for this operation`, and the service stayed `not_suspended` (the refusal happens before the verb, as with the env-group link in sweep 849).
- `DELETE` afterwards → 204 regardless of lifecycle state (the earlier suspend/resume idempotency cycle is sweep 797).
