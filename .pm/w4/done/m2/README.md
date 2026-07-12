# w4 · m2 — bex-api auth: Hydra introspection + Kratos sessions

**Worker:** worker4 **Goal:** bex-api validates real credentials — OAuth2 bearer tokens via Hydra introspection and Kratos sessions for the dashboard GraphQL — behind a `BEX_AUTH_MODE` env flag, with the static `BEX_API_TOKEN` kept as fallback. **Status:** done (2026-07-05; E2E green on the local mock — Hydra token → REST + GraphQL 200, garbage 401, static regression intact; `make test` green with `GOTOOLCHAIN=go1.25.7`)

## Tasks (in order)

| id   | title                                                                                | est | depends_on |
| ---- | ------------------------------------------------------------------------------------ | --- | ---------- |
| t001 | Auth middleware: Hydra token introspection + `BEX_AUTH_MODE` flag — **DONE**         | 30m | — (w4/m1)  |
| t002 | Kratos session validation for the GraphQL dashboard surface — **DONE**               | 30m | t001       |
| t003 | Env plumbing + update `docs/ADR006-bex-api.md` and `CLAUDE.md` env table — **DONE**         | 25m | t001       |
| t004 | E2E acceptance: Hydra token → REST + GraphQL succeed; bad token → 401 — **DONE**     | 25m | t002, t003 |
| t005 | Simplify — run `/simplify` over the code this milestone changed — **DONE**           | 20m | t004       |
| t006 | Test coverage — meaningful tests for the behavior this milestone shipped — **DONE**  | 30m | t004       |

## E2E invocation (t004)

On the local mock cluster (`bash scripts/mock-cluster.sh`, auth substrate deployed per docs/ADR012-auth.md, App CRD installed via `cd operator && make install`):

```
KUBECONFIG=$PWD/infra/local/bex.kubeconfig bash scripts/auth-e2e.sh
```

## Definition of done

With `BEX_AUTH_MODE=ory`, a Hydra `client_credentials` token authenticates REST and GraphQL calls and an invalid/expired token gets 401; legacy static-token mode still works when flagged; env vars documented.

## Source + Goal linkage

- **Source:** `/pm-brainstorm` 2026-07-05 — user chose deploy-only m1 with bex-api integration split into this m2.
- **Goal linkage:** pillar 1 (API-first, Render-compatible surface) with real multi-client credentials; prerequisite for the multi-tenant control-plane API (w1/m2 t006).
- **Expected outcome:** per-client, revocable credentials for agents and humans instead of one shared secret.
- **Why now:** immediately unlocked by w4/m1; every w2 agent surface (MCP, deploy-from-chat) needs per-client tokens before it can be multi-tenant.
