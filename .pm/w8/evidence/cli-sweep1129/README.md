# CLI sweep 1129 — GraphQL reads the same store REST verbs mutate (live, 2026-10-20)

One disposable free web service created and deleted (204, verified 404). No new bug filed.

- GraphQL lives at `/graphql`; `/v1/graphql` → 404 (the versioned prefix is REST-only).
- `service { suspended }` reads `not_suspended` → REST `suspend` (202) → GraphQL reads `suspended` → REST `resume` (202) → GraphQL reads `not_suspended`: one store across both surfaces, no projection lag.
- After DELETE, GraphQL returns `data.service: null` with a `NOT_FOUND` error extension — the GraphQL spelling of the REST 404.
