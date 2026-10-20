# CLI sweep 1168 — rename moves `name` only; empty rename is a no-op (live, 2026-10-20)

Two disposable free web services created and deleted (204, verified 404). No new bug filed.

- PATCH `name: qa1168-renamed` → 200; read-back shows `name` changed while `slug` and `immutableName` keep the creation value (`qa1168-web`): the URL identity is frozen at create, the display identity is mutable.
- Renaming to another service's name (`hello-go`, the baseline) → 409, matching the create-time duplicate rule (sweep 1143).
- PATCH with an empty or whitespace-only name → 200 and the name is unchanged on read-back: blank is treated as unset, not as a value or an error.
