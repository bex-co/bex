# CLI sweep 988 — duplicate service name conflict (live, 2026-10-20)

One disposable free web service created and deleted (204). No new bug filed.

- Creating a second service with the same name in the same workspace → coded 409 `CONFLICT` naming the colliding name (`name "qa988-dup" is already in use`). The conflict is detected before any second resource exists; the follow-up cleanup needed only the first service.
