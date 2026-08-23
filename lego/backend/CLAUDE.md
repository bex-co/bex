# backend/CLAUDE.md

The **backend** module (`github.com/bex-co/bex/lego/backend`) is the **business-logic** layer. It builds **bex-api** (`cmd/api`) — Render-compatible REST/GraphQL/MCP + OpenFGA + API keys + metrics — and the isolated **SSH gateway** (`cmd/ssh-gateway`). It imports `types/` and **never** `operator/` — it patches App CR _intent_, the operator converges it. Build/test from here (`go build/test ./...`); `make` targets + `config/` live in `../operator`.

## Layout — by feature, not layer

One package per feature: service (business logic) + models + REST/GraphQL/MCP fragments.

- `cmd/api/` — bex-api entrypoint (`api mcp-stdio` / `BEX_MCP_STDIO=1` → MCP over stdio)
- `cmd/ssh-gateway/` — isolated public-key SSH entrypoint; only Deployment with `pods/exec`
- `internal/core/` — leaf kernel: `Base` + `Identity` + sentinels + `Authorize` gates + `TTLCache` + `Poll` workers. Imports CRD types only.
- `internal/hmacticket/` — one HMAC envelope for `shellticket`/`sandboxexec`/`agentsessionticket`; each flavor owns only `Claims`. Leaf.
- `internal/id/` — typed ids `<prefix>-<xid>` (`id.New(kind)`, `id.WellFormed`). Leaf. [ADR020](../../docs/ADR020-identifiers.md)
- `internal/<feature>/` — `apps`, `logs`, `metrics`, `apikeys`, `sshkeys`, `postgres`, `secrets`, `deploys`, `events` (read VIEW over `deploys`+`audit_events`). Each `service.go` + `rest.go`/`graphql.go`/`mcp.go`.
  - `sshgateway` — protocol→exec boundary; shared `Executor/SessionLimiter/NonceGuard/TargetResolver` + `nativessh/webshell/sandboxsse/agentattach/agentcred`; one limiter/guard across transports.
  - `authz` (OpenFGA `core.Checker`), `gqlutil` helper.
- `internal/store/` — control-plane Postgres + App-CR projector, opt-in via `BEX_CP_DB_URI` ([ADR003](../../docs/ADR003-control-plane.md))
- `internal/billing/` — Stripe Billing (opt-in `BEX_STRIPE_SECRET_KEY`), sealed `usage_hourly` → Customers/Subscriptions, invoices, webhooks.
- `internal/api/` — composition root: one auth gate (`auth.go`) + one REST router / one GraphQL schema / one MCP registry. See [`internal/api/CLAUDE.md`](internal/api/CLAUDE.md).

## Rules

- **Never import `operator/`**. Cross via `types/` + App CR only.
- **Ids via `internal/id` only** (`id.New(kind)`), hyphen-separated; depguard forbids `xid` elsewhere. [ADR020](../../docs/ADR020-identifiers.md)
- **One service per feature, three thin fragments, Render-consistent.** Verbs live in `service.go`; fragments map wire formats onto single roots (one schema/router/registry). See `internal/api/CLAUDE.md`.
- **Authz seam:** single-resource verbs start `a, err := s.AuthorizeApp(ctx, core.Rel…, name)` (or `Database/KeyValue`) — fetches + authorizes against resource's OWN `LabelTenant`. Bare `s.Authorize` only for `List`/create. Swept by `TestAuthzGuardsEveryVerb` + `TestFetchByNameUsesTheVerbsOwnRelation`. Never split into two gates.
- Target always recorded (`ServiceTarget/DatabaseTarget/KeyValueTarget`) for events feed; write relations always emit, read only on denial. `TestEveryTargetedVerbIsNamedOrExcused` enforces vocabulary.
- **Workspace from request context** (REST `ownerId`, GraphQL `ownerId`, MCP `workspaceId`). Empty → default workspace; non-member → `ErrForbidden`.
- Store opt-in `BEX_CP_DB_URI`; unset → bex-api alone.

## Environment variables — compact reference

Full meanings + defaults + ADR pointers live in the long descriptions below; this table is the quick lookup (operator/SSR vars → sibling `../operator/CLAUDE.md`, `dashboard/CLAUDE.md`). Unset ⇒ feature 503/disabled unless noted.

| Component | Variable | Purpose (default → disabled/unset behavior) |
| --- | --- | --- |
| bex-api | `BEX_API_ADDR` `:8090`, `BEX_API_NAMESPACE`, `BEX_API_CORS_ORIGIN`, `BEX_API_PUBLIC_URL` | listen, watched ns, CORS allowlist, public API origin (deploy-hook URLs + agent `streamUrl`; unset → relative/omitted) |
| bex-api | `BEX_REGION` | placement name on Service/DB/KV metadata (e.g. `fsn1`); unset → omitted |
| bex-api | `BEX_SSH_HOST` | public SSH hostname `ssh.bex.co` for `serviceDetails.sshAddress` + `agentSession.sshAddress` |
| bex-api | `BEX_SHELL_TICKET_SECRET`, `BEX_SHELL_WS_URL` | browser Web Shell HMAC key + gateway `wss://…/shell` origin; either unset → shell 503 |
| bex-api | `BEX_AGENT_SESSION_GATEWAY_URL`, `BEX_AGENT_SESSION_IMAGE`, `BEX_SHELL_TICKET_SECRET` | agent sessions: gateway origin `https://api.bex.co/v1/agent-sessions` (edge→gateway) + driver image + HMAC; unset → create/resume/steer/attach-ticket 503 |
| bex-api | `BEX_AGENT_GIT_PROXY_URL` | gateway Git proxy override (default `http://bex-ssh-gateway…:8082`) |
| bex-api | `BEX_AGENT_MODEL_PROXY_URL` | **mandatory** model proxy origin (`http://bex-ssh-gateway…:8084`); unset → agent create/steer/rehydrate disabled (no fallback) |
| bex-api | `BEX_AGENT_SANDBOX_IDLE_TTL` | finished session keep-alive after last turn/editor disconnect (default `30m`) |
| bex-api | `BEX_AGENT_MAX_LIVE_SANDBOXES_PER_WORKSPACE` | live sandbox cap per workspace (default `5`, 409 `AGENT_SESSION_LIVE_LIMIT`) |
| bex-api | `BEX_AGENT_SNAPSHOT_S3_*` (ENDPOINT/BUCKET/REGION/PREFIX/ACCESS_KEY/SECRET_KEY) | hibernation object store; all set → hibernate/rehydrate, any unset → off (bucket needs SSE) |
| bex-api | `BEX_AGENT_SNAPSHOT_RETENTION`, `BEX_AGENT_MAX_PINNED_SANDBOXES_PER_WORKSPACE` | retention `168h` (7d, 2× if dirty git) + pin cap `10` (409 `AGENT_SESSION_PIN_LIMIT`) |
| bex-api | `BEX_SANDBOX_EXEC_SECRET`, `BEX_SANDBOX_EXEC_URL` | `sandbox exec` HMAC + gateway `http://…:8081/sandbox-exec`; unset → exec 503 |
| ssh-gateway | `BEX_SSH_ADDR` `:2222`, `BEX_SSH_METRICS_ADDR` `:9090`, `BEX_SSH_HOST_KEY_PATH` | SSH listen, metrics, required host private key |
| ssh-gateway | `BEX_SHELL_WS_ADDR` `:8080`, `BEX_SHELL_TICKET_SECRET` | Web Shell gateway transport; unset secret → disabled |
| ssh-gateway | `BEX_SANDBOX_EXEC_ADDR` `:8081`, `BEX_SANDBOX_EXEC_SECRET` | sandbox exec gateway transport (internal-only); unset → disabled |
| ssh-gateway | `BEX_AGENT_CREDENTIAL_ADDR` `:8082`, `BEX_AGENT_CREDENTIAL_API_URL` | Git smart-HTTP proxy + bex-api mint URL; disabled if exec secret unset |
| ssh-gateway | `BEX_AGENT_MODEL_PROXY_ADDR` `:8084` + `…_CREDENTIAL_API_URL`, `…_MAX_CONNS`, `…_PER_POD`, `…_READ_TIMEOUT`, `…_MAX_DURATION` | model proxy gateway transport; caps `32/2`, `2m`, `2h`; shares exec secret |
| ssh-gateway | `BEX_AGENT_MODEL_MAX_REQUESTS_PER_SESSION/…_WORKSPACE` | cumulative exchange budgets `1000/5000`, 429 when exhausted |
| ssh-gateway | `BEX_AGENT_ATTACH_ADDR` `:8083`, `BEX_AGENT_SESSION_DRIVER_PORT` `8787`, `BEX_SHELL_TICKET_SECRET` | agent conversation SSE transport; shares shell secret |
| ssh-gateway | `BEX_CP_DB_URI`, `BEX_OPENFGA_URL`, `BEX_OPENFGA_TOKEN`, `BEX_API_NAMESPACE` | gateway store/authz/ns scope |
| ssh-gateway | `BEX_SSH_HANDSHAKE_TIMEOUT` `10s`, `…_SESSION_TIMEOUT` `4h`, `…_MAX_SESSIONS` `100`, `…_PER_IDENTITY` `5` | connection bounds |
| ssh-gateway | `BEX_SSH_MAX_PREAUTH_CONNS` `256`, `…_PER_SOURCE` `32` | pre-auth handshake caps (global + per-source); trusted PROXY CIDRs |
| ssh-gateway | `BEX_SSH_PROXY_PROTOCOL_TRUSTED_CIDRS` | CIDRs allowed to assert PROXY v1/v2 client IP |
| ssh-gateway | `BEX_SSH_MAX_CHANNELS_PER_CONN` `16` | Zed multiplexing cap per conn for `ags-*` |
| ssh-gateway | `BEX_SSH_MAX_CHANNELS` `512`, `…_PER_IDENTITY` `32` | process-wide exec-stream caps |
| ssh-gateway | `BEX_SSH_REVALIDATE_INTERVAL` `1m` | live-stream fresh re-auth tick (SSH/webshell/agent/sandbox exec) |
| ssh-gateway | `BEX_AGENT_GIT/MODEL_*_PREAUTH_CONNS` + `…_MAX_CONNS/PER_POD/READ_TIMEOUT/MAX_DURATION/MAX_REQUESTS_*` | Git/model proxy admission bounds (global 128/16, per-pod 64/4, `10m`, budgets `1000/5000`) |
| bex-api | `BEX_BUILD_NAMESPACE` | build Job ns (must match operator); logs `type=build`. Pre-deploy Jobs are co-located with the App (ADR043 D8), so `type=predeploy` logs read from the App's own namespace |
| bex-api | `BEX_HYDRA_ADMIN_URL` req, `BEX_KRATOS_URL` | OAuth2 introspection + Kratos sessions |
| bex-api | `BEX_KRATOS_ADMIN_URL` | Kratos admin for owners/members email/MFA |
| bex-api | `BEX_OPENFGA_URL`, `BEX_OPENFGA_TOKEN` | OpenFGA authz; unset → allow-all |
| bex-api | `BEX_ALLOW_INSECURE_AUTHZ` | `1` allows startup with `BEX_CP_DB_URI` but no FGA (dev only); else fail-closed |
| bex-api | `BEX_BASE_DOMAIN` | wildcard `onbex.co` for custom-domain DNS targets |
| bex-api | `BEX_PROM_URL` | Prometheus (Traefik/cAdvisor); unset → request 503, metrics fallback |
| bex-api | `BEX_USAGE_RETENTION_MONTHS` `3`, `BEX_AUDIT_RETENTION_DAYS` `90` | usage hot window + audit purge intervals |
| bex-api | `BEX_MAX_BLUEPRINT_GROUPINGS` `1000`, `…_ENV_GROUPS…` `100`, `…_GIT_CONNECTIONS…` `10`, `…_REGISTRY_CREDS…` `50` | per-workspace caps (coded `*_LIMIT` 409) |
| bex-api | `BEX_MAX_CUSTOM_DOMAINS_PER_SERVICE` `100` / `…_WORKSPACE` `500` | custom-domain quotas (409 `CUSTOM_DOMAIN_LIMIT`) |
| bex-api | `BEX_STRIPE_SECRET_KEY` | restricted Stripe key; unset → no Stripe client/emitter/webhook, estimate-only |
| bex-api | `BEX_REQUIRE_PAYMENT_METHOD` `1`/`all` | paid-intent gate (needs Stripe+store); `all` includes free + agent-sessions |
| bex-api | `BEX_STRIPE_API_URL`, `…_SEAL_HOURS` `48`, `…_EPOCH`, `…_WEBHOOK_SECRET`, `…_COMP_COUPON_ID` `bex-comp-100`, `…_PORTAL_CONFIGURATION_ID`, `…_TAX_CODE/_BEHAVIOR` | Stripe overrides, seal horizon, billing epoch, webhook/coupon/portal/tax |
| bex-api | `BEX_DISK_SNAPSHOT_ENDPOINT`/`_BUCKET`/`_PREFIX`/`_REGION`/`_ACCESS_KEY`/`_SECRET_KEY` | read-only view of the operator's disk-snapshot bucket (docs/ADR082-persistent-disks.md D5) for `GET /v1/disks/{diskId}/snapshots`; unset → snapshot list/restore 503, disks otherwise unaffected. bex-api only LISTS: it never writes or decrypts a snapshot, and never holds the age key. The 24h `snapshotKey` is signed with `BEX_SHELL_TICKET_SECRET` (a reference to an object, not its contents) |
| bex-api | `BEX_LOKI_URL` | Loki durable logs; set → history+filters, unset → live pod logs |
| bex-api | `BEX_OPENBAO_URL`, `BEX_OPENBAO_JWT_PATH` | OpenBao env-vars store; JWT path default pod token |
| bex-api | `BEX_CP_DB_URI`, `BEX_CP_APPS_NAMESPACE`, `BEX_CP_ADDR` `:8091`, `BEX_CP_RESYNC`, `BEX_CP_TOKEN`, `BEX_CP_IDENTITY` | store URI (opt-in) + projection ns/addr/resync/token + instance identity (`production` default, per-dev `dev-N`) |
| bex-api | `BEX_OPENSANDBOX_URL`, `BEX_SANDBOX_IMAGE` (default `docker.io/library/alpine:3@sha256:…`) | OpenSandbox lifecycle + `/v1/sandboxes*` surface; the base sandbox template image, digest-pinned like every image bex runs (w7/m85) |
| bex-api | `BEX_AGENT_SETUP_REGISTRIES` | setup egress FQDN allowlist (npm/PyPI/Go/…) override |
| bex-api | `BEX_WEBHOOK_SECRET`, `…_RETENTION_DAYS` `90`, `…_KEEP` `1000`, `…_MAX_DELIVERIES…` `10000`, `…_BACKOFF`, `BEX_GITHUB_APP_*`, `…_WEBHOOK_SECRET`, `BEX_SMTP_*`, `…_REQUIRE_VERIFIED_INVITE_EMAIL` (on unless `0`), `BEX_DASHBOARD_URL`, `BEX_MCP_STDIO` | webhooks/GitHub/SMTP/invite/dashboard/MCP (see ADRs: 006, 026, 024) |
| bex-api | `BEX_OAUTH_ISSUER`, `…_RESOURCE`, `…_REQUIRE_AUDIENCE`, `…_API_SCOPE` (compat, closed vocab `bex.read/write/sensitive`) | OAuth 2.1 discovery + audience/scope rules ([ADR012] §7) |
| bex-api | `BEX_RATE_LIMIT` `500`, `…_BURST`, `…_AUTH_FAILURE_LIMIT` `60`, `…_MAX_INFLIGHT` `64`, `…_DEVICE_LIMIT` `30`, `…_WEBHOOK_LIMIT` `600`, `…_TRUSTED_PROXY_CIDRS` | token-bucket rates + trusted proxy CIDRs |
| bex-api | `BEX_MAX_BODY_BYTES` `2MiB`, `…_QUERY_HOURS` `720`, `…_SSE_CONNS` `100/5/20`, `…_LOG_STREAM_REVALIDATE_INTERVAL` `1m` | body/query/SSE/log-tail bounds |

Detailed semantics, ADRs, and startup-fail-closed conditions: see the original per-variable paragraphs in git history or the referenced `docs/ADR*.md`.
