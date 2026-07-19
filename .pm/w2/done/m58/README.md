# m58 — Multi-replica Streamable HTTP MCP sessions

**Status:** Done — 2026-07-19

## Goal

An initialized MCP client can send subsequent requests through any bex-api replica without losing its session, and its selected workspace remains bound to both the MCP session and authenticated subject.

## Definition of done

- Streamable HTTP requests do not depend on process-local SDK session state.
- Workspace selection is shared through the control-plane Postgres store when configured, with the existing in-memory behavior retained for local/storeless use.
- Selection lookup is scoped to the authenticated subject and fails closed on store errors.
- A deterministic two-replica regression alternates requests across independent server instances.
- MCP/operator/backend tests and relevant documentation pass.

## Tasks

- [x] **t001** — Freeze the multi-replica session and identity contract — **DONE**
- [x] **t002** — Persist MCP workspace selection in the shared Postgres store — **DONE**
- [x] **t003** — Make Streamable HTTP transport replica-independent — **DONE**
- [x] **t004** — Add deterministic alternating-replica regression coverage — **DONE**
- [x] **t005** — Refresh MCP connection and parity documentation — **DONE**
- [x] **t006** — Simplify the implementation after the feature is complete — **DONE**
- [x] **t007** — Run focused and broad validation — **DONE**
- [x] **t008** — Close out evidence and PM state — **DONE**

## Origin

Promoted from `w2/015.md` after production verification on 2026-07-17 showed intermittent `404 session not found` responses whenever follow-up MCP requests reached a replica other than the one that handled `initialize`.

## Outcome

- `/mcp` now uses the Go MCP SDK's stateless Streamable HTTP mode. Follow-up POSTs accept the client session id without consulting a process-local transport map; standalone SSE GET correctly returns 405 because bex exposes tools, not server-initiated MCP requests.
- Migration `0044_mcp_workspace_selections` and `PGStore` persist selected workspace by `(session_id, subject)`, with workspace deletion cascading the row. Storeless and stdio operation keep the small concurrency-safe local fallback. The migration was renumbered during ship after current `main` independently claimed `0043` for service-event facts.
- Every workspace-scoped MCP adapter propagates shared-store errors instead of silently falling back, while an explicit `ownerId` continues to take precedence without a selection lookup.
- `TestMCPStreamableHTTPAlternatesReplicas` constructs two independent server/handler instances behind a deterministic alternating load balancer. Initialize, initialized notification, selection, selection readback, and scoped `list_services` cross replicas without `session not found` or lost selection.
- Documentation now records the stateless POST/405 GET transport contract, subject-scoped shared selection, no-affinity deployment requirement, and the resolved production residual.

## Validation

- `go test ./...` from `lego/backend/` — pass.
- `make lint-backend` from `lego/operator/` — pass, 0 issues.
- Focused `go test ./internal/core ./internal/api ./internal/store` — pass.
- Disposable Postgres 17 integration run with `BEX_TEST_DB_URI`: `TestPGStore`, migration uniqueness, ownership, upsert, subject isolation, and workspace cascade — pass.
- Browser-shell regression: 3 Vitest files / 8 tests, dashboard typecheck, and production build — pass.
