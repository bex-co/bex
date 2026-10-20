# CLI sweep 855 — name availability is a GraphQL bex extension (live, 2026-10-20)

Read-only probes. No new bug filed.

- There is no REST `/services/name-available` route (the path falls through to `retrieve-service`, which refuses the `name` query parameter by name). Availability is the GraphQL `serviceNameAvailable` query — a documented bex extension (Render has no public availability API; `lego/backend/internal/apps/graphql.go:642`, `nameavailable.go`).
- Scoping is the caller's workspace: `hello-go` (a canary service) reads `available:true` from the default workspace, while `agentmarketcap-1` (taken in the default workspace) reads `available:false` with `suggestion: "agentmarketcap-1-1"`.
