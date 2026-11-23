# w8 · m579 — Share bex-api rate-limit buckets and log-subscription caps across replicas

**Worker:** worker8 **Goal:** every Render-compatible REST operation, Render MCP tool, deploy hook, general/sandbox API bucket, and log-subscription cap (`BEX_MAX_SSE_CONNS`, per-subject, per-workspace) is enforced across all `bex-api` replicas through one shared control-plane Postgres store, so a caller's effective budget matches Render's documented numbers and `RateLimit-Remaining` decreases consistently whichever replica answers. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | Control-plane Postgres schema and atomic store operations for shared token buckets and log-subscription leases | 45m | — |
| [t002](t002.md) | Shared rate-limit backend in `core`, `api`, `sandbox`, `deploys` and `cmd/api` with fail-open fallback | 50m | t001 |
| [t003](t003.md) | Shared log-subscription caps across replicas with lease heartbeats in `internal/logs` | 40m | t001 |
| [t004](t004.md) | Docs: update ADR006, ADR018, ADR104 and rate-limit references | 25m | t002, t003 |
| [t005](t005.md) | Render parity | 25m | t004 |
| [t006](t006.md) | Simplify | 20m | t005 |
| [t007](t007.md) | Test coverage | 40m | t005, t006 |
| [t008](t008.md) | Live check on production in `bex-canary` | 20m | t007 |
| [t009](t009.md) | Closeout | 10m | t008 |

Total: **9 tasks, about 4h35m**.

## Definition of done

1. **Shared token buckets across replicas.** With two `bex-api` limiter instances sharing the same control-plane Postgres store, 400 Render `GET` requests across both within a minute succeed with consistent, monotonically decreasing `RateLimit-Remaining` (399, 398, …), and the 401st is refused with HTTP 429, `Retry-After`, and `RateLimit-Remaining: 0` whichever instance handles it. The same holds for every Render row (including per-service deploy/hook and hourly service-create rows), the general limiter, and the sandbox rate classes.
2. **All-or-nothing multi-bucket charges and clean refunds.** When a request checks more than one bucket (caller/row + workspace) and a later bucket refuses, no earlier bucket loses a token. When an MCP tool or GraphQL sandbox field takes its own row/class, its general token is refunded cleanly in the shared store.
3. **Shared log-subscription caps across replicas.** With two `logs.Service` instances sharing the same Postgres store, global (`BEX_MAX_SSE_CONNS`), per-subject (`BEX_MAX_SSE_CONNS_PER_SUBJECT`), and per-workspace (`BEX_MAX_SSE_CONNS_PER_WORKSPACE`) limits are enforced across both instances. Closing a stream releases its lease immediately; a crashed replica's lease expires automatically after its TTL while active streams renew on a heartbeat.
4. **Fail-open fallback and internal exemptions.** When Postgres is unavailable or unconfigured (unit tests, DB-free local mode, or transient DB error), both the rate limiter and the subscription limiter fall back to their in-process implementations without returning 500s, and the choice is documented in ADR006. Internal callers (`:8091`, `/healthz`, `/readyz`, in-process reconcilers) stay unthrottled, and `BEX_RATE_LIMIT=0` still disables rate limiting.
5. **Docs and parity.** ADR006 § Rate limits / Request caps, ADR018's Rate limits row, ADR104 D7 (where sandbox buckets share the backend), and `docs/render-artifacts/rate-limit-headers.md` drop the single-replica caveat, record the outage behavior, and cite only actual user decisions as user decisions.
6. **Live check on production.** In `bex-canary` once production's pin carries the fix, two `GET /v1/services?ownerId=<bex-canary>&limit=1` requests within 150 ms read `RateLimit-Limit: 400` and `RateLimit-Remaining: 399` then `398`, whichever replica each reaches, without provoking a 429.

## Source + Goal linkage

- **Source:** inbox note [w8/417](../done/417.md), filed 2026-11-23 as a follow-up from [w8/410](../done/410.md) and promoted by `/loopx w8` the same day.
- **Triage evidence (2026-11-23, `1b540b4d96`):**
  - **The premise holds.** Production runs `bex-api` with 2 replicas (`lego/operator/config/prod/kustomization.yaml:185`), its Ingress (`lego/operator/config/api/ingress.yaml`) and Service (`service.yaml`) do not pin callers to one replica, every rate limiter (`lego/backend/internal/core/ratelimit.go:56`, `internal/api/ratelimit.go`, `internal/api/render_ratelimit.go`, `internal/sandbox/ratelimit.go`) keeps per-key buckets in process memory, and `logs.Service.acquireSlot` (`lego/backend/internal/logs/rest.go:450`) tracks `MaxSSEConns`, `MaxSSEConnsPerSubject`, and `MaxSSEConnsPerWorkspace` in process memory.
  - **Store choice:** there is no platform Valkey/Redis in `deploy/gitops/base/` (only tenant-managed KeyValue instances; the SSH gateway's `NonceGuard` uses control-plane Postgres `shell_ticket_nonces` at `lego/backend/internal/store/sshsessions.go:96`). Control-plane Postgres (`store.PGStore`) is the shared store `bex-api` already depends on in production (`deploy/gitops/base/bex-postgres.yaml`).
  - **Why not consistent-hash routing or dividing by replicas:** consistent hashing at Traefik cannot key by resolved API-key creator (`core.RateLimitUser`, shared across a user's keys), by `{serviceId}` (deploy row + hooks), or by workspace, nor can it enforce global/workspace log-subscription caps across replicas. Dividing each budget by replica count changes `RateLimit-Limit` (e.g. 200 instead of 400) and still leaves `RateLimit-Remaining` jumping between replicas.
  - **Sibling note:** `.pm/w22/021.md` tracks the same replica split for sandbox rate limits (`internal/sandbox/ratelimit.go`); sharing the token-bucket backend at `core` covers sandbox classes alongside the REST/Render rows (this milestone does not mutate `w22/021.md`).
  - **Sizing:** the fix spans a Postgres migration and atomic store methods, the shared limiter backend across `core`/`api`/`sandbox`/`deploys`/`cmd/api`, lease-backed log-subscription caps with heartbeats in `internal/logs`, ADR updates, Render parity review, multi-instance real-Postgres tests, and a live `bex-canary` check (~4h35m across 9 tasks).
- **Goal linkage:** ADR008 § Render parity first ([ADR006](../../../docs/ADR006-bex-api.md) § Rate limits, [ADR018](../../../docs/ADR018-render-parity.md) Rate limits row). Render's rate limits apply per user (and per service for deploys), not per backend replica.
- **Expected outcome:** a script or client pacing itself from `RateLimit-Remaining` sees consistent counts across consecutive requests, and a caller's effective rate budget and concurrent log-tail caps match the documented limits regardless of replica count.
- **Why now:** w8/410 (`696783e94e`) just adopted Render's per-endpoint REST/MCP rate limits on production, and its live read showed consecutive GETs reading 399 twice across the two replicas; ADR018 records no user sign-off for a per-replica divergence, so under AGENTS.md it is open parity work.
- **Render parity included:** yes (`t005`) — the rate-limit headers, refusal bodies, and log-subscription caps are tenant-facing across REST, GraphQL, MCP, deploy hooks, and SSE/WS log tails.
