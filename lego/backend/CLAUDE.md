# backend/CLAUDE.md

The **backend** module (`github.com/bex-co/bex/lego/backend`) is the **business-logic** layer. Today it builds one binary — **bex-api** (`cmd/api`, the `/api` entrypoint) — the Render-compatible REST/GraphQL/MCP surface plus OpenFGA authz, API-key auth, and the metrics API. It imports the `types/` module (the CRD contract) and **never** the `operator/` module: it writes _intent_ (App CR spec patches), the operator converges it. Build/test from here (`go build ./... && go test ./...`); the shared `make` targets, `config/`, and codegen live in `../operator`.

## Layout

- `cmd/api/` — the bex-api entrypoint. `api mcp-stdio` (or `BEX_MCP_STDIO=1`) serves only the MCP adapter over stdio for a local agent.
- `internal/api/` — one `Core` (`core.go`) with three thin adapters (`rest.go`, `graphql.go`, `mcp.go`); plus `auth.go` (API-key/identity), `authz.go` (OpenFGA checker), `apikeys.go`, `metrics.go`. See [`internal/api/CLAUDE.md`](internal/api/CLAUDE.md).

## Rules

- **Never import `operator/`** (mechanism). Cross-layer contact is only through `types/` and the `App` CR.
- **One `Core`, three adapters, Render-consistent.** A change to one adapter (REST/GraphQL/MCP) must fan out to the other two; verbs have a single implementation in `core.go`. Full rules in `internal/api/CLAUDE.md`.
- The Postgres source of truth (control plane) is **planned, not built** here yet ([../../docs/control-plane.md](../../docs/control-plane.md)); when it lands it joins this module.
