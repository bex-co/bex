# internal/api/CLAUDE.md — bex-api

bex-api is **one `Core` (`core.go`) with three thin adapters** — `rest.go`, `graphql.go`, `mcp.go` — that are pure presentation over identical Core methods and hold no logic. This is the design guarantee that the three surfaces can't drift ([docs/bex-api.md](../../../../docs/bex-api.md)). Two rules keep it that way:

- **New verbs go in `core.go` only.** An adapter never gets a second implementation of a verb; it maps the shared Core method to its wire format. **Every new Core verb starts with `c.authorize(ctx, rel…)`** (authz.go — mapped to the Render-shaped relations in authz.go (relCanView/relCanViewLogs/relCanOperate/relCanCreate/relCanViewSensitive/relCanManageKeys)); enforcement at Core level is what keeps the three surfaces authorization-identical. Pod logs are the one dependency Core reaches past the generic client for — via the injected `PodLogSource` (`podlogs.go`), kept out of `core.go` so the domain layer stays clientset-free and testable.
- **Three-adapter parity — a change to one adapter must consider the other two.** When you add/rename/reshape a verb, argument, or field in REST, GraphQL, _or_ MCP, apply the equivalent change to the other two (or write down why one legitimately differs). A verb reachable over REST should be reachable over GraphQL and MCP; the same `Core` change fans out to all three. Don't leave one surface behind.

**Render.com consistency is mandatory, per surface.** Each adapter mirrors its Render counterpart and is verified against the real Render artifact — don't "fix" a shape to look more conventional:

- REST → Render's public OpenAPI spec (`render-public-api-1.json`): e.g. `suspended` is the string enum `"suspended"`/`"not_suspended"`, not a boolean; list is the `{service, cursor}` envelope; managed Postgres is the `/v1/postgres` noun.
- GraphQL → operation names captured from Render's dashboard (`services`, `server(id)`, `suspendService`, `database(id)`, …).
- MCP → Render's official MCP server (`render-oss/render-mcp-server`): tool names (`list_services`/`get_service`/`list_logs`) and argument names (`serviceId`, `list_logs`'s `resource` array) match 1:1. bex-only extensions (e.g. `restart_service`/`suspend_service`/`resume_service`, which Render's MCP omits) still follow Render's naming convention.

Omit Render fields/filters bex can't honor rather than faking them — the returned object stays a safe superset (Render clients ignore unknown keys; bex clients get the extras like `phase`/`replicas`/`revision`).

**Transports & auth.** Every HTTP route (REST, GraphQL, MCP streamable-HTTP at `/mcp`) sits behind the same auth gate ([docs/bex-api.md#auth](../../../../docs/bex-api.md)): OAuth2 API-key tokens via Hydra introspection, Kratos sessions for humans, no shared static token. Missing `BEX_HYDRA_ADMIN_URL` refuses to start; Ory outages fail closed. MCP's stdio transport (`api mcp-stdio`) is the exception — its trust boundary is the subprocess itself, so no auth gate applies.
