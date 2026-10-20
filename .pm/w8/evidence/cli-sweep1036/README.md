# CLI sweep 1036 — new-pin regression spot battery (live, 2026-10-20)

One disposable free web service created and deleted (204; GET 404). No new bug filed.

- On pin `9e9c28e82d0f`, previously-verified behaviors hold: no-body restart → 200, unknown PATCH field → 400, empty env-var key → 400. The pin change (which carried the 137/138/139 fixes) did not regress these neighboring surfaces.
