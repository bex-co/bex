# CLI sweep 1143 — duplicate service name conflicts (live, 2026-10-20)

One disposable free web service created and deleted (204, verified absent). No new bug filed.

- A second create with the same name in the same workspace → 409 with code `CONFLICT` and message `name "qa1143-dup" is already in use`. Names are unique per workspace and the collision is a coded conflict, not a silent second service or a generic 400.
