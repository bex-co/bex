# CLI sweep 1226 — rapid deploy triggers supersede (live, 2026-10-20)

One disposable free web service created and deleted (204, verified 404). No new bug filed.

- Three deploy triggers in quick succession → 201 each. The deploy list then shows the newest as `queued` and the superseded ones as `canceled` (plus the initial `created` deploy): a newer trigger cancels its still-pending predecessors instead of queueing a pile-up or refusing with a conflict.
