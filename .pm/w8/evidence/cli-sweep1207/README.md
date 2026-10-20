# CLI sweep 1207 — GraphQL create/delete full cycle (live, 2026-10-20)

One GraphQL-created free web service deleted and verified absent (REST 404, name list empty). No new bug filed.

- `createService` takes flat arguments (35 of them, introspected): an enum-typed guess for `type` was refused with the expected type named (`Expected type "String"`); the string form plus `image` created the service, and REST read it immediately (one store, sweep 1129/1171 parity extended to creation).
- `deleteService` returns `Boolean`: asking for a sub-selection → validation error, and the service survived that failed call — failed mutations change nothing. The correct scalar form returned `true` and the service was gone.
