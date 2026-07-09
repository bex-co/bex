# w2 · m1 — MCP server over bex-api verbs

**Worker:** worker2 **Goal:** Expose the bex-api lifecycle verbs over MCP as "just another thin adapter over the same `Core`" — so an agent operates bex natively (list/get/restart/suspend/resume/logs) instead of screen-scraping a dashboard. Tool names and shapes track Render's official MCP server (`render-oss/render-mcp-server`) — a third Render-consistent adapter alongside the REST (public-API) and GraphQL (dashboard) ones. Delivers pillar 3. **Status:** done

## Render MCP consistency

The MCP surface mirrors Render's official server the way `rest.go` mirrors Render's OpenAPI spec and `graphql.go` mirrors the dashboard operations:

- **Snake_case tool names, Render's nouns.** `list_services`, `get_service`, `list_logs` map 1:1 onto Render's tools; each takes/returns the same `service` object bex already emits (`id`, `name`, `type: "web_service"`, string `suspended` enum, `dashboardUrl`, `serviceDetails.url`) plus bex extras (`phase`, `replicas`, `revision`) — a superset Render-trained agents safely ignore.
- **`id` is the App name**, exactly as in REST/GraphQL — opaque, round-tripped from `list_services`.
- **Lifecycle verbs are bex extensions.** Render's official MCP is read-heavy and does _not_ expose restart/suspend/resume; bex adds `restart_service` / `suspend_service` / `resume_service`, named after Render's REST verbs (`POST /v1/services/{id}/{verb}`) so they read as native to a Render-shaped agent. Each returns the updated `service` object; the operator converges asynchronously (poll `get_service` for `suspended`/`phase`).
- **Out of scope, matching bex-api today.** No `create_*`, `get_metrics`, or Postgres/Key-Value tools — bex has no metrics or datastore surface yet. Add them when `Core` grows those verbs, keeping Render's names. (`list_deploys`/`get_deploy` shipped in w2/m5, once deploy history existed to serve them.)

## Tasks (in order)

| id | title | est | depends_on | status |
| --- | --- | --- | --- | --- |
| t001 | MCP adapter over `Core` — `list_services`/`get_service`/`{restart,suspend,resume}_service` | 30m | — | done |
| t002 | Add a `Logs` verb to `Core` + expose as `list_logs` (Render log-label shape) | 30m | t001 | done |
| t003 | Auth + transport: reuse `bex-api-token` bearer, stdio + streamable-http | 25m | t001 | done |
| t004 | Manifests + deploy + end-to-end acceptance | 30m | t001,t003 | done |

## Definition of done

An MCP client (e.g. Claude) can `list_services`, `get_service`, `restart_service`/`suspend_service`/`resume_service`, and `list_logs` — every tool delegating to the same `Core` (`operator/internal/api/core.go`) as REST/GraphQL, authed by the `bex-api-token` Secret. Tool names, arguments, and the returned `service` object are consistent with Render's official MCP server, so an agent that knows Render operates bex with no relearning. No verb has a second implementation.

## Implementation

Built as a third adapter in `operator/internal/api/`, no new domain logic outside `Core`:

- **`mcp.go`** — the MCP adapter (uses `github.com/modelcontextprotocol/go-sdk`). Registers the six Render-consistent tools; each delegates to the same `Core` method REST/GraphQL call. `serviceTool` reuses one mapping for `get_service`/`restart_service`/`suspend_service`/`resume_service` (the REST verb pattern). Returns the `renderService` shape (`render.go`).
- **`core.go`** — new `Logs(ctx, name, tail)` verb: lists an App's pods by the controller's `app.bex.co/app` label, tails each via an injected `PodLogSource`, parses the timestamp prefix, and returns timestamp-sorted `LogEntry`s in Render's log-label shape (`service`/`instance`/`container`). `ErrLogsUnavailable` when unwired, `ErrNotFound` for unknown Apps.
- **`podlogs.go`** — production `PodLogSource` over a client-go clientset (the one read controller-runtime's client can't serve); kept out of `Core` so the domain layer stays clientset-free and testable.
- **Transports (`mcp.go` + `server.go`)** — streamable-HTTP mounted at `/mcp` behind the same `bearerAuth(bex-api-token)` gate as REST/GraphQL; stdio via `Server.RunStdio`, selected in `cmd/api` by `api mcp-stdio` / `BEX_MCP_STDIO=1` (stdio's trust boundary is the subprocess, so no bearer there).
- **Manifests (`config/api/rbac.yaml`)** — least-privilege bump: read-only `pods` + `pods/log` for the logs verb; existing `/`-prefix Ingress and `:8090` Deployment already cover `/mcp`, no change.

**Acceptance:** `mcp_test.go` drives a real in-memory MCP client↔server round-trip — asserts the six Render-consistent tool names are advertised, that `suspend_service` travels the identical `Core` write path as REST (mutating the App), that `list_logs` returns the Render log-label shape, and that an unknown id surfaces as a tool error. `Core.Logs` aggregation/sorting and its error modes are unit-tested. `make test` green; `make lint` net-neutral vs. the pre-existing baseline.

## Source

`docs/vision.md` pillar 3 (MCP server); `docs/bex-api.md` "one Core, thin adapters"; Render's official MCP server `render-oss/render-mcp-server` (tool names: `list_services`, `get_service`, `list_logs`, `list_deploys`, `get_metrics`, …).
