# CLI sweep 1135 — deploy cancel is a one-way state transition (live, 2026-10-20)

One disposable free web service created and deleted (204, verified 404). No new bug filed.

- Cancel of the in-flight deploy → 200; read-back shows `status: canceled`.
- Cancelling the same deploy again → 409 (already in a terminal state, not a silent no-op).
- Cancelling an unknown deploy id under a real service → 404.

Cancel follows the same discipline as restart-while-suspended (sweep 1127): meaningful transitions succeed once, repeats against a terminal state conflict, and unknown ids stay 404.
