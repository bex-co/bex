# w4 · m3 — API keys replace the static token

**Worker:** worker4 **Goal:** bex-api's shared `BEX_API_TOKEN` is deleted; every machine caller holds an API key = its own revocable OAuth2 client in Hydra, manageable through the API itself, with a deploy-time-seeded bootstrap key. **Status:** done (2026-07-06; /simplify applied — shared Ory transport + drainClose, generic gqlField resolver, bex.co/api-key metadata marker scoping list/revoke to bex-minted keys, session-cookie gating, doc-drift fixes; E2E re-passed after every change)

## Tasks (in order)

| id   | title                                                                              | est | depends_on |
| ---- | ---------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Remove static mode: ory-only auth gate, no `BEX_API_TOKEN`/`BEX_AUTH_MODE` — **DONE** | 30m | — (w4/m2)  |
| t002 | API-key verbs in Core + REST `/v1/api-keys` + GraphQL + MCP parity — **DONE**      | 45m | t001       |
| t003 | Bootstrap client seeding: script + deploy.yml step + `.env`/gh-secrets — **DONE**  | 30m | t001       |
| t004 | Manifests + docs cutover (deployment env, ADR006-bex-api.md, ADR012-auth.md, CLAUDE.md) — **DONE** | 25m | t002, t003 |
| t005 | E2E: bootstrap client mints key → key calls API → revoke kills it — **DONE**       | 30m | t004       |
| t006 | Simplify — run `/simplify` over the code this milestone changed — **DONE**         | 20m | t005       |
| t007 | Test coverage — meaningful tests for the behavior this milestone shipped — **DONE** | 30m | t005       |

## Definition of done

bex-api refuses to start without `BEX_HYDRA_ADMIN_URL` and accepts no shared secret; the seeded `bex-bootstrap` client's token authenticates REST/GraphQL; `POST /v1/api-keys` mints a working key (secret shown once), list omits secrets, revoke prevents new token issuance — proven by `scripts/auth-e2e.sh` exit 0 on the mock cluster; no `BEX_API_TOKEN`/`BEX_AUTH_MODE` references remain in code or manifests; `make test` green.

## E2E invocation (t005)

On the local mock cluster (auth substrate per docs/ADR012-auth.md, App CRD via `cd operator && make install`):

```
KUBECONFIG=$PWD/infra/local/bex.kubeconfig bash scripts/auth-e2e.sh
```

## Source + Goal linkage

- **Source:** user decision 2026-07-06 (session following w4/m2): "remove static and replace it with client credential api keys — do it now".
- **Goal linkage:** pillars 3–4 (per-agent revocable credentials) and roadmap #1 (multi-tenant control plane) — deletes the last shared-secret path.
- **Expected outcome:** zero shared secrets in the API path; keys mintable/revocable via REST/GraphQL/MCP; CI seeds the bootstrap key on every deploy.
- **Why now:** w4/m1+m2 made the substrate and middleware real; the static token's only remaining callers were our own scripts, so cutover cost was at its minimum and only grows as clients accrete.
