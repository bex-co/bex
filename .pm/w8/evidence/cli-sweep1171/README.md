# CLI sweep 1171 — GraphQL mutations mutate the same store (live, 2026-10-20)

One disposable free web service created and deleted (204, verified 404). No new bug filed.

- GraphQL `suspendService(id)` → 200 with `suspended: suspended` in the payload, and the REST read immediately agrees (`suspended`). The mutation surface is not a GraphQL-only projection: both write paths land in one store, extending the read parity from sweep 1129 to writes.
