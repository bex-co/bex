# CLI sweep 1148 — server scoping is by ownerId/membership, not the workspace header (live, 2026-10-20)

Read-only sweep (baseline `hello-go` read only, never mutated). No new bug filed.

- `GET /services?limit=100` without `ownerId` returns the caller's default-workspace services (7 items, all owned by `tea-d982…`) regardless of the `X-Bex-Workspace` header value — including a bogus header and no header. The header is a client-side selection the CLI translates into an `ownerId` query parameter; the server scopes lists by that parameter, not by the header.
- A direct read of the `bex-canary` baseline service succeeds under the canary header, the default-workspace header, and a bogus header alike: direct-id access is governed by workspace membership (the QA user is a member of both real workspaces), consistent with the sweep 1024 finding. The bogus header neither grants nor denies anything on its own.
