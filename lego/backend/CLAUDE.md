# backend/CLAUDE.md

The **backend** module (`github.com/bex-co/bex/lego/backend`) is the **business-logic** layer. Today it builds one binary — **bex-api** (`cmd/api`, the `/api` entrypoint) — the Render-compatible REST/GraphQL/MCP surface plus OpenFGA authz, API-key auth, and the metrics API. It imports the `types/` module (the CRD contract) and **never** the `operator/` module: it writes _intent_ (App CR spec patches), the operator converges it. Build/test from here (`go build ./... && go test ./...`); the shared `make` targets, `config/`, and codegen live in `../operator`.

## Layout

Organized **by feature** (one package per feature), not by technical layer. A feature = one Go package holding its **service** (business logic) + **models** + the **REST/GraphQL/MCP registration fragments** as files inside it.

- `cmd/api/` — the bex-api entrypoint. `api mcp-stdio` (or `BEX_MCP_STDIO=1`) serves only the MCP adapter over stdio for a local agent.
- `internal/core/` — the **leaf kernel** every feature imports: `Base` (client + namespace + clock + the `Authorize` gate + `GetApp`/`AppPods`), the caller `Identity`, the error sentinels, and the shared HTTP/cache helpers (`WriteErr`, `DoJSON`, `TTLCache`, …). Imports the CRD types, nothing else in bex.
- `internal/<feature>/` — one package per feature: `apps` (lifecycle + plans), `logs`, `metrics`, `apikeys`, `postgres` (managed Databases), `secrets` (OpenBao-backed env vars). Each has `service.go` (verbs, each starting with `s.Authorize`) + `models.go`/render shapes + `rest.go`/`graphql.go`/`mcp.go` fragments. `authz` is the OpenFGA checker (satisfies `core.Checker`); `gqlutil` is the shared GraphQL resolver helper.
- `internal/store/` — the **control-plane store** (docs/control-plane.md): Postgres migrations (`tenants`/`apps`/`domains`), the rows→App-CR reconciler, and the internal tenant API (`BEX_CP_ADDR`, default :8091). Opt-in: wired by `cmd/api` only when `BEX_CP_DB_URI` is set.
- `internal/api/` — the **composition root**: wires the feature services behind one auth gate (`auth.go`) and assembles the three surfaces as **single artifacts** — one REST router, one GraphQL schema, one MCP registry (`server.go`). It imports the features + `core`; features never import it (no cycle). See [`internal/api/CLAUDE.md`](internal/api/CLAUDE.md).

## Rules

- **Never import `operator/`** (mechanism). Cross-layer contact is only through `types/` and the `App` CR.
- **One service per feature, three adapter fragments, Render-consistent.** A verb has a single implementation in its feature's `service.go`; its REST/GraphQL/MCP fragments are thin presentation that _register into the single shared roots_. A change to one surface fans out to the other two. Never fragment the roots themselves (one schema / one router / one registry). Full rules in `internal/api/CLAUDE.md`.
- **Every verb starts with `s.Authorize(ctx, core.Rel…)`** — enforcement at the service layer keeps the three surfaces authorization-identical (`TestAuthzGuardsEveryVerb` sweeps this).
- The Postgres source of truth (control plane) lives here as `internal/store/`, **opt-in via `BEX_CP_DB_URI`** ([../../docs/control-plane.md](../../docs/control-plane.md)); with it unset the binary is bex-api alone.
