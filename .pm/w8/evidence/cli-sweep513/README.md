# CLI sweep 513 — Key Value required-field completeness

**Verdict: no bug filed.** Same method as sweep 512 applied to Key Value: diff the live REST payload against every `required` field in Render's vendored schemas. Same isolated session, `bex-canary`, HEAD build `c603c8ec2`, 2026-10-19T09:15Z.

One free Key Value created → `available` (~40 s) → raw `GET /v1/key-value/{id}` diffed → deleted by ID (named not-found verified).

| check | observed result |
| --- | --- |
| Required fields vs `keyValue` / `redis` / `keyValueDetail` / `redisDetail` | None missing, none `null` — all 14 payload keys present including `updatedAt`, `region`, `owner`, `version`, `ipAllowList`. |
| `suspenders` | Correctly absent: Render's Key Value schemas do not define it (the sweep 512 gap is Postgres-specific). |
| Contrast with the conformance fixture | The live payload carries fields the `retrieve-redis` allowlist records as missing (`updatedAt`/`region`/`owner`) — confirming those allowlist rows describe the minimal fixture view, not production. The Postgres `role`/`suspenders` omissions filed in w8/129 were diffed against the **live** payload, not the fixture, so they stand. |

## Safety and unrun checks

- Cleanup: exact-owned `red-dbatvbnbcrps73dmoal0` deleted by ID; re-`get` answers the named not-found. No connection credentials printed or recorded.
- Unrun: suspended-state Key Value payload (no `suspenders` in schema, so only the `suspended` enum flips — covered by sweep 454's suspend/resume leg on Postgres and KV reads elsewhere), list-envelope field completeness per item.
- Deploy lag means deployed revisions can precede local `main`.
