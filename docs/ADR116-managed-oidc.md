# ADR116 — Managed OIDC: one issuer per workspace, Render's claims, an ES256 key set and its rotation

**Status:** accepted (2026-11-18, w2/m476): the issuer, the signer, the key store, rotation and the public documents. Runtime delivery and AWS (D9) accepted 2026-11-18 with w2/m477. Anthropic and OpenAI (w2/m478), GCP (w2/m479) and build time (w2/m480) reuse D9.

**Basis:** Render parity first ([ADR008](ADR008-vision.md#render-parity-first), user decisions 2026-10-17 and 2026-10-20). The user filed w2/m476–m480 with `/pm them all to w2` on 2026-11-17, reopening the 2026-08-23 deferral in `.pm/FUTURE-MAYBE.md`. The choices below that the brainstorm left open (D2 host, D5 custody) are the worker's, with the basis given; none is a user decision.

## Context

Render's Managed OIDC lets a service on a Pro workspace or higher authenticate to AWS, GCP, Anthropic and OpenAI with a short-lived token instead of a stored key ([render.com/docs/oidc](https://render.com/docs/oidc), read 2026-11-17; [How we built Render OIDC](https://render.com/blog/how-we-built-render-oidc)). Each workspace is its own identity provider, `https://oidc.render.com/<tea-…>`, so a provider trusting one workspace rejects another's tokens even when its trust policy skips the subject check. Tokens expire 60 minutes after issuance, and a daemon outside the container keeps a fresh one in a read-only, memory-backed directory.

Render's issuer, probed 2026-11-17 and again 2026-11-18:

- `GET /tea-c0000000000000000000/.well-known/openid-configuration` answers 200, `Content-Type: application/json`, `Cache-Control: public, max-age=300`, with exactly five fields in this order and a trailing newline: `issuer`, `jwks_uri` (`<issuer>/.well-known/jwks.json`), `response_types_supported: ["id_token"]`, `subject_types_supported: ["public"]`, `id_token_signing_alg_values_supported: ["ES256"]`.
- `GET /tea-c0000000000000000000/.well-known/jwks.json` answers 200 with the same headers and `{"keys":[{"use":"sig","kty":"EC","kid":"_VSjJvQvo0AvEDG68QvXH01Ps2ZPj4Wl_ZIQZNo6PvU","crv":"P-256","alg":"ES256","x":"…","y":"…"}]}`, no trailing newline. The `kid` is 43 base64url characters, an RFC 7638 SHA-256 thumbprint's length, and it is the same for every workspace probed.
- Any `tea-` id whose 20 characters decode as an xid answers; there is no existence check. `tea-abc`, `tea-d0000000000000000001` (its last character carries bits an xid cannot have), `TEA-…`, `srv-…`, `/`, `/<tea>`, `/<tea>/` and a trailing slash after a document answer Go's `404 page not found` (`text/plain; charset=utf-8`, `X-Content-Type-Options: nosniff`).
- `POST` and `HEAD` on a document's path answer `405` with `Allow: GET` and an empty body, even for a malformed workspace id (`POST /tea-abc/.well-known/jwks.json` is a 405); `POST /` is a 404.

Render has no REST, GraphQL, MCP, CLI or Blueprint surface for OIDC. Its settings are ordinary environment variables (`AWS_ROLE_ARN`, `GCP_WORKLOAD_IDENTITY_PROVIDER`, `ANTHROPIC_FEDERATION_RULE_ID`, `OPENAI_IDENTITY_PROVIDER_ID` and their companions).

## Decisions

### D1 — The issuer and its two documents

- The issuer of workspace `<tea>` is `<BEX_OIDC_ISSUER_URL>/<tea>`, `https://oidc.bex.co/<tea>` in production. The two documents are served at `/<tea>/.well-known/openid-configuration` and `/<tea>/.well-known/jwks.json` with Render's field set, field order, headers and trailing bytes, as listed under Context.
- A workspace id is accepted when it is `tea-` followed by 20 characters that decode as an xid (`id.Is(id.Workspace, …)`, the same strict decoding Render's answers show). Nothing checks that the workspace exists: an unknown well-formed id gets the same answer as a real one, so the issuer reveals nothing about which workspaces exist. Every other path on the issuer host is a `404 page not found`. A document's path asked for with any method but `GET` is a `405` with `Allow: GET` and no body, before the id is checked, as on Render. One known difference: Go's router redirects an uncleaned path (`//tea-…/…`) where Render answers 404; no client sends one.
- Discovery does not depend on the key set. The JWKS answers `503` until this replica has loaded the key set once.
- Both documents carry `Cache-Control: public, max-age=300`, Render's value. Rotation (D6) is sized so a verifier holding a JWKS for that long, or much longer, never meets a token signed with a key it has not seen.
- The routes need no credential and sit behind bex-api's shared rate limiter, keyed by client address. They are in the always-public inventory (`internal/api/alwayspublic_guard_test.go`).

### D2 — The issuer host

- The host is `oidc.<root domain>`: `oidc.bex.co` in production, set by `BEX_OIDC_ISSUER_URL` (an `https://` or `http://` origin with no path, query or user info, whose host is not the API's). Unset, the issuer is off: no route is mounted and nothing is signed. A self-hosted installation sets its own.
- Production DNS needs no change: the proxied `*.bex.co` wildcard already sends `oidc.bex.co` through Cloudflare to Traefik (verified 2026-11-18: it answered Traefik's `404 page not found`), as `api.bex.co` is sent. A dedicated Ingress (`lego/operator/config/api/ingress-oidc.yaml`) gives the host a Let's Encrypt certificate (`bex-oidc-tls`) and routes it to bex-api's public port.
- bex-api routes by host: a request whose `Host` is the issuer host reaches only the issuer, and the issuer does not answer on `api.bex.co`. The issuer is not mounted on the `:8091` control-plane listener.

### D3 — Claims

The JOSE header is `{"alg":"ES256","kid":"<kid>","typ":"JWT"}`. The payload has five claims:

| Claim | Value |
| --- | --- |
| `iss` | `<BEX_OIDC_ISSUER_URL>/<tea>` |
| `sub` | `workspace:<tea>:environment:<evm-…>:service:<srv-…>`, with `default` in place of the environment id for a service outside every environment (Render's template, [render.com/docs/oidc](https://render.com/docs/oidc) and [Keycard's Render guide](https://docs.keycard.ai/guides/deploy-to-render-without-secrets/)). An environment is written with its public `evm-` id, never the `env-` storage spelling. |
| `aud` | one string per provider: `sts.amazonaws.com` (AWS), `api.anthropic.com` (Anthropic), `api.openai.com` (OpenAI), and for GCP the workload identity provider's default audience, whose exact spelling w2/m479 fixes |
| `iat` | the signing time, in whole seconds |
| `exp` | `iat` + 3600 (Render: "Each token Render signs expires 60 minutes after issuance") |

There is no `nbf` and no `jti`: Render documents neither, and none of the four providers requires them. Render publishes no sample token, so whether its tokens carry further claims is **unverified**. A provider rule that matches on a claim bex does not send would refuse bex's token, and that would be filed as drift when found.

The signer refuses a subject whose workspace, environment or service id is not well formed, and an empty audience.

### D4 — Algorithm, key and `kid`

- ES256 on P-256, the one algorithm discovery advertises.
- The `kid` is the RFC 7638 thumbprint of the public key: base64url (no padding) of the SHA-256 of `{"crv":"P-256","kty":"EC","x":"…","y":"…"}`.
- One key set signs for every workspace, as Render's probes show. Isolation between workspaces comes from `iss`, which each provider's trust binds to one workspace's issuer URL, not from separate keys.
- Signing uses `github.com/golang-jwt/jwt/v5`, already a backend dependency (the GitHub App and Web Push signers use it). The JWKS is encoded by hand with Render's field order. Tests and `scripts/oidc-issuer-verify.sh` verify with `github.com/coreos/go-oidc/v3` (with `go-jose`), already pinned in the workspace by the CLI module, so the verifier shares no code with the signer.

### D5 — Key custody: the control-plane database

- The key set lives in control-plane Postgres, table `oidc_signing_keys (kid, private_key, created_at, activates_at)`, the private key as PKCS#8 DER. Every bex-api replica reads it every minute.
- **Why not a `bex-system` Secret, as the brainstorm proposed.** bex-api has no access to any `bex-system` Secret by design ([ADR028](ADR028-security-review.md), guarded by `scripts/gitops-validate.sh`): a compromised bex-api cannot read the platform's credentials. A Secret here would mean a create grant (RBAC cannot name a Secret that does not exist yet) or a manifest Argo CD would fight bex-api over. The database is already inside bex-api's trust boundary, holds webhook signing secrets the same way, and gives the transaction and advisory lock that make bootstrap single-writer across replicas.
- **Why not OpenBao transit.** OpenBao is unsealed by hand after a restart; token refresh would stop with it.
- **Losing the key set is recoverable without tenants.** Providers trust an issuer URL and fetch its JWKS; none pins a key. bex-api bootstraps a fresh key when the table is empty, and services get tokens signed with it at their next refresh.
- **Residual.** A copy of the control-plane database (a backup in Wasabi, a dump) holds the private key, and with it a holder can mint tokens for any workspace until the key is rotated out. That is the exposure class of `webhooks.secret`. The stricter variants, envelope encryption under a key the database never holds, or a KMS/HSM signer, are follow-ups in `.pm/FUTURE-MAYBE.md`, not part of parity.

### D6 — Bootstrap and rotation

A key's state is a function of the stored rows and the time, so every replica agrees without a coordinator:

| State | When | Published in the JWKS | Signs |
| --- | --- | --- | --- |
| next | `activates_at` is in the future | yes | no |
| current | the latest key whose `activates_at` has passed | yes | yes |
| previous | superseded, and its successor started signing less than 2 hours ago | yes | no |
| removed | superseded, and its successor started signing 2 or more hours ago | no (the row is deleted at the next reload) | no |

The numbers:

- **Publish lead: 1 hour.** `oidc-rotate-key --apply` inserts a key that activates 1 hour later. It is in every replica's JWKS within the 1-minute reload, so a verifier caching the JWKS for the advertised 5 minutes, or for anything under 59 minutes, has the key before the first token it signs.
- **Retention after supersession: 2 hours.** A token lives 1 hour; the second hour covers clock skew and a replica's last token signed just before its reload. Nothing still valid was signed with a removed key.
- **Staleness guard: 1 hour.** A replica that has not reloaded the key set for an hour (the publish lead) refuses to sign, because it could have missed a rotation and would sign with a key its peers are about to remove. It keeps serving the JWKS it has.
- **Bootstrap.** When the table is empty, a replica inserts one key that signs at once, inside a transaction holding an advisory lock, so replicas starting together create exactly one key.
- **One rotation at a time.** `oidc-rotate-key` refuses while a next key is pending.
- **Emergency rotation.** `oidc-rotate-key --emergency --apply` inserts a key that signs at once and deletes every other key. Tokens signed with the old key stop verifying immediately, which is the point when the key may be compromised; services recover at their next token refresh.

The operator-run commands, `kubectl -n bex-system exec deploy/bex-api -- /api oidc-rotate-key [--emergency] [--apply]` and `/api oidc-test-token <tea> <srv>`, are in [runbooks/oidc-key-rotation.md](runbooks/oidc-key-rotation.md). Like every operator-run verb they never mount on `:8090`. `oidc-test-token` mints for the fixed audience `bex-oidc-verify`, which no provider accepts, so the verification tool cannot become a way to act as a tenant at AWS or a model provider.

### D7 — Delivery

Who asks for a token, how it reaches the container, its refresh and the plan gate (the brainstorm's D3 and D4) are D9, w2/m477's addendum. The signer's interface is `oidcissuer.Issuer.Mint(Subject, audience)`.

### D8 — No new surface

bex adds no REST, GraphQL, MCP, dashboard, CLI or Blueprint field for OIDC, as Render has none. The public documents in D1 are the only tenant-facing surface this ADR adds.

### D9 — Runtime delivery (w2/m477)

Render, [render.com/docs/oidc](https://render.com/docs/oidc) (read 2026-11-18): add `AWS_ROLE_ARN` and redeploy; Render "automatically sets an additional environment variable named `AWS_WEB_IDENTITY_TOKEN_FILE`" to the token file's path, and says "Do not manually set" it. Render's [build write-up](https://render.com/blog/how-we-built-render-oidc): a daemon outside the container writes the tokens "before the service ever starts" and keeps them fresh, into a host directory in memory-backed `/dev/shm`, mounted with "a read-only bind mount" at `/var/lib/render/oidc/`. Neither names the file inside the directory, so bex's file name (`aws-token`) is **unverified** against Render's. Render's behavior below Pro is not documented.

**Who decides.** Two halves, each where its input lives:

- **The plan, bex-api.** An App of a workspace whose plan includes Managed OIDC (`core.FeatureManagedOIDC`, Pro and higher; `core.PlanIncludes`, so an unknown plan includes nothing) carries the label `app.bex.co/managed-oidc: enabled`, only while this bex-api runs an issuer. The projector keeps it in step with the plan (`store.ManagedOIDCLabel`, like the kept-builds label), and the create path stamps it on a new App, so a service created with `AWS_ROLE_ARN` mounts its token from its first deploy.
- **The trigger, the operator.** When it projects a release's workloads, the operator reads `AWS_ROLE_ARN` from that release's own environment, as it reads `PORT` and `WEB_CONCURRENCY` (`configuredEnv`): a literal, the service's env Secret or a linked group's, or their per-release snapshots. A blank value is no trigger. The REST, GraphQL, MCP, dashboard, Blueprint and env-group surfaces all write those sources, so no surface needs its own field, as on Render.
- **The decision is per release.** The operator decides when it first projects a release (its pre-deploy step, instances or cron template, whichever comes first; a rollback reads its selected release's environment), records it in `status.managedOIDC` (`generation`, `providers`, `planAllowed`) before any pod of the release exists, and keeps it for the life of the release. A variable or plan change reaches a service at its next deploy and never rolls a running one, and a pass reads no variable to keep the decision. A release already serving when an operator with Managed OIDC first sees it mounts nothing until its next deploy, so the upgrade rolls no pod. A cancel that restores a served release's recorded template records that template's mounts instead.

**Who mints: bex-api** (the `cloneSecret` precedent; the operator holds no key and no database). `internal/managedoidc.Minter` lists the Apps carrying the label and, for each whose `status.managedOIDC` mounts a provider, writes the provider's token (D3's claims, `aud` `sts.amazonaws.com`) to the App's Secret `<app>-oidc`, key `aws-token`. One replica writes at a time (`store.PGStore.RunExclusive`, a session advisory lock). The Secret carries `app.bex.co/protected-from-tenant-mount`, so another service naming it in its env is refused: it would act as this service. The App owns it (not as controller), so deleting the service deletes its tokens.

**Refresh.** A pass runs every minute, and at once when a release starts mounting a token (the App status watch bex-api already runs wakes it for that change alone), and replaces a token once it is 20 minutes old, a third of its life: a refresh failing for half an hour still leaves a valid file, and the kubelet's Secret sync (about a minute) never hands a container a token close to expiry. A service that moves environment gets a token for its new `sub` at the next pass. At most 100 writes per pass; a pass that hits the bound starts the next at once. `bex_managed_oidc_refresh_failures_total` and `bex_managed_oidc_oldest_token_age_seconds` meter it. A downgraded workspace, a release without the trigger and a refused release get no new tokens; the last one expires on its own.

**How it reaches the container.** A release that mounts a provider gets a Secret volume of `<app>-oidc` listing `aws-token`, mounted read-only at `/var/lib/render/oidc` (the kubelet keeps Secret volumes in tmpfs, as Render's `/dev/shm`), and `AWS_WEB_IDENTITY_TOKEN_FILE=/var/lib/render/oidc/aws-token`. The volume is required, so a container does not start before its token exists; the decision is written before the pod and wakes the minter, so the token is usually there first. A value the service set itself for `AWS_WEB_IDENTITY_TOKEN_FILE` gives way to the platform's, as Render warns. A release without the trigger gets none of it: its pod template is the one it had before.

**Which containers.** Every container bex runs for the service: instances (web, private, background worker), every cron run, the pre-deploy step and the initial deploy hook (which copies the instance's template). SSH and dashboard shell sessions exec into an instance and inherit the file and the variable. One-off jobs and ephemeral shells are refused on bex (`.pm/DO_NOT_DO.md`). Build time is w2/m480. OIDC does not authenticate private image pulls, on Render either.

**Below Pro** (the brainstorm's D4; Render's behavior unverified). The variable is accepted and kept on every surface, no token is mounted, and the deploy's log says why: `==> Managed OIDC requires a Pro workspace or higher: AWS_ROLE_ARN is set, but no identity token is mounted` (from `status.managedOIDC`, so only the running release's deploy carries it).

**Residuals.**

- A token of at most 60 minutes sits in etcd and its backups. Memory-only delivery through a CSI driver is the stricter variant, in `.pm/FUTURE-MAYBE.md`.
- A relying party that does not check `iss` accepts any workspace's token, because the key set is shared (D4). AWS, GCP, Anthropic and OpenAI bind a trust to one issuer URL; MinIO's STS does not check `iss` (observed 2026-11-18), so `scripts/oidc-aws-verify.sh` proves the exchange there and leaves workspace isolation to `scripts/oidc-issuer-verify.sh`.
- While a newer release's pre-deploy step holds the rollout, the decision recorded is the newer release's. If it drops the trigger, the served instances get no refresh for the length of the step, and their token stays valid for at least 40 minutes after the last one.

## Consequences

- ADR018's row "Managed OIDC for AI providers" names w2/m476–m480 as owners, and ADR030's open gap points at the same milestones. Both stay ✖ until delivery ships.
- `BEX_OIDC_ISSUER_URL` is documented in `lego/backend/AGENTS.md`.
- `scripts/oidc-issuer-verify.sh` checks production: a token minted for a workspace verifies through that workspace's discovery with go-oidc, fails against another workspace's issuer, and discovery has Render's keys.
- `scripts/oidc-aws-verify.sh` checks a running service (D9): its token file and variable, an `AssumeRoleWithWebIdentity` exchange at an STS trusting its workspace's issuer, a refusal for another audience, and the same across one refresh.
