# CLI sweep 891 — GraphQL error discipline on synthetic ids (live, 2026-10-20)

Read-only GraphQL probes plus one refused mutation on a synthetic id (nothing mutated; baseline read only). No new bug filed.

- `service(id: srv-000…)` → HTTP 200 with `data.service: null` and an `errors[]` entry carrying `extensions.code: "NOT_FOUND"` and a path — GraphQL-conventional surfacing of the same coded not-found REST returns as 404.
- `suspendService(id: srv-000…)` → the same `NOT_FOUND` extension, no state touched.
- The baseline reads `phase: "Hibernated"` with `suspended: "not_suspended"` — asleep-but-not-suspended is a first-class distinct state, as the suspend/sleep separation requires.
