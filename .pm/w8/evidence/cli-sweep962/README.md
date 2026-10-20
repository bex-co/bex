# CLI sweep 962 — GraphQL service query shapes (live, 2026-10-20)

Read-only probes on the baseline `hello-go` service. No fixture created. No new bug filed.

- `service(id:)` returns the service with REST-consistent fields (`plan: free`, `suspended: not_suspended`, `type: web_service`).
- Unknown and malformed ids both answer `service: null` plus a `NOT_FOUND`-coded GraphQL error — no distinction between "bad format" and "absent", and no data leak either way.
- `services` is a plain list field (Render dashboard shape), not a Relay connection: `first:`/`edges`/`pageInfo` are rejected with precise `Unknown argument`/`Cannot query field` errors, and the plain `{ services { id name } }` form returns the workspace list.
