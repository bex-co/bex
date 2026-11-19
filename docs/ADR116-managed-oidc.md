# ADR116 — Managed OIDC: one issuer per workspace, Render's claims, an ES256 key set and its rotation

**Status:** accepted (2026-11-18, w2/m476): the issuer, the signer, the key store, rotation and the public documents. Runtime delivery and AWS (D9) accepted 2026-11-18 with w2/m477. Anthropic and OpenAI (D10) accepted 2026-11-18 with w2/m478. GCP (D11) accepted 2026-11-18 with w2/m479. Build time (D12) accepted 2026-11-18 with w2/m480.

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
- A workspace id is accepted when it is `tea-` followed by 20 characters that decode as an xid (`id.Is(id.Workspace, …)`, the same strict decoding Render's answers show). Nothing checks that the workspace exists: an unknown well-formed id gets the same answer as a real one, so the issuer reveals nothing about which workspaces exist. Every other path on the issuer host is a `404 page not found`. A document's path asked for with any method but `GET` is a `405` with `Allow: GET` and no body, before the id is checked, as on Render. One known difference: an uncleaned path (`//tea-…/…`) does not get Render's 404. Go's router in bex-api answers it with a 307 to the cleaned path. On production the edge cleans the path before bex-api sees it, as it does for every `*.bex.co` host, so the client gets the document (checked 2026-11-18, w2/m476 t007). No client sends one.
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
| `aud` | one string per provider: `sts.amazonaws.com` (AWS), `api.anthropic.com` (Anthropic), `api.openai.com` (OpenAI), and for GCP the workload identity provider's default audience, `https://iam.googleapis.com/projects/<project number>/locations/<location>/workloadIdentityPools/<pool>/providers/<provider>` (D11) |
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

**Which containers.** Every container bex runs for the service: instances (web, private, background worker), every cron run, the pre-deploy step and the initial deploy hook (which copies the instance's template). SSH and dashboard shell sessions exec into an instance and inherit the file and the variable. One-off jobs and ephemeral shells are refused on bex (`.pm/DO_NOT_DO.md`). Build time is D12. OIDC does not authenticate private image pulls, on Render either.

**Below Pro** (the brainstorm's D4; Render's behavior unverified). The variable is accepted and kept on every surface, no token is mounted, and the deploy's log says why: `==> Managed OIDC requires a Pro workspace or higher: AWS_ROLE_ARN is set, but no identity token is mounted` (from `status.managedOIDC`, so only the running release's deploy carries it).

**Residuals.**

- A token of at most 60 minutes sits in etcd and its backups. Memory-only delivery through a CSI driver is the stricter variant, in `.pm/FUTURE-MAYBE.md`.
- A relying party that does not check `iss` accepts any workspace's token, because the key set is shared (D4). AWS, GCP, Anthropic and OpenAI bind a trust to one issuer URL; MinIO's STS does not check `iss` (observed 2026-11-18), so `scripts/oidc-aws-verify.sh` proves the exchange there and leaves workspace isolation to `scripts/oidc-issuer-verify.sh`.
- While a newer release's pre-deploy step holds the rollout, the decision recorded is the newer release's. If it drops the trigger, the served instances get no refresh for the length of the step, and their token stays valid for at least 40 minutes after the last one.

### D10 — Anthropic and OpenAI (w2/m478)

Render, [render.com/docs/oidc](https://render.com/docs/oidc) (read 2026-11-18):

- **Anthropic.** In the Claude Console: a service account (Developer role), a Custom OIDC issuer `https://oidc.render.com/<tea>` with JWKS source "OIDC Discovery", and a federation rule (subject pattern, expected audience `api.anthropic.com`, token lifetime "At least 30 minutes"). On the service: `ANTHROPIC_FEDERATION_RULE_ID`, `ANTHROPIC_ORGANIZATION_ID`, `ANTHROPIC_SERVICE_ACCOUNT_ID` and `ANTHROPIC_WORKSPACE_ID`, then redeploy. "Render detects the `ANTHROPIC_FEDERATION_RULE_ID` environment variable and automatically sets an additional environment variable named `ANTHROPIC_IDENTITY_TOKEN_FILE`". "By reading the environment variables above, the Anthropic SDK can authenticate automatically". To test, SSH into the service with `JWT=$(cat $ANTHROPIC_IDENTITY_TOKEN_FILE)`.
- **OpenAI.** In the organization's Security settings: a Workload Identity Provider with the issuer and audience `api.openai.com`, and a mapping on `sub` to a service account. On the service: `OPENAI_IDENTITY_PROVIDER_ID` and `OPENAI_SERVICE_ACCOUNT_ID`. "Render detects the `OPENAI_IDENTITY_PROVIDER_ID` environment variable and automatically sets an additional environment variable named `OPENAI_IDENTITY_TOKEN_FILE`". "Your code should read the token from this file path at runtime and use it to initialize the OpenAI client".

**Triggers.** Each provider's trigger is the one variable Render says it detects, `ANTHROPIC_FEDERATION_RULE_ID` and `OPENAI_IDENTITY_PROVIDER_ID`, with the same rule as `AWS_ROLE_ARN` (D9: a non-blank value in the release's own environment, from any source). The companions are read by the SDK, not by the platform: a service that sets a trigger without them still gets its token, and its SDK reports what is missing. The audiences are Render's, `api.anthropic.com` and `api.openai.com`.

**Delivery** is D9's, unchanged. The providers sit in one table, `ManagedOIDCProviders` (`lego/types/v1alpha1/managed_oidc.go`; `scripts/oidc-audience-verify.sh` carries a copy its self-test keeps equal). A release mounts each provider its environment asks for: `anthropic-token` with `ANTHROPIC_IDENTITY_TOKEN_FILE` and `openai-token` with `OPENAI_IDENTITY_TOKEN_FILE`, beside `aws-token`, in the same read-only directory. Render names no file, so these names are **unverified**. The minter writes one token per provider into `<app>-oidc`, each signed for its own audience and never reused across audiences, and refreshes them together. A provider added at a deploy is minted at once. One dropped at a deploy loses its key, and the others keep theirs. Below Pro, the deploy log names every trigger the release set (`AWS_ROLE_ARN, ANTHROPIC_FEDERATION_RULE_ID and OPENAI_IDENTITY_PROVIDER_ID are set, but no identity token is mounted`).

**What the official SDKs read** (sources read 2026-11-18):

| SDK | Reads | First release |
| --- | --- | --- |
| Anthropic [Python](https://github.com/anthropics/anthropic-sdk-python/blob/main/src/anthropic/lib/credentials/_chain.py), [TypeScript](https://github.com/anthropics/anthropic-sdk-typescript/blob/main/src/lib/credentials/credential-chain.ts), [Go](https://github.com/anthropics/anthropic-sdk-go/blob/main/internal/auth/constants.go) | The zero-argument client uses federation when `ANTHROPIC_FEDERATION_RULE_ID`, `ANTHROPIC_ORGANIZATION_ID` and `ANTHROPIC_IDENTITY_TOKEN_FILE` are set, and sends `ANTHROPIC_SERVICE_ACCOUNT_ID` and `ANTHROPIC_WORKSPACE_ID` when they are. It exchanges the token at `<base URL>/v1/oauth/token` (jwt-bearer grant) and rereads the file at every exchange ("Kubernetes projected service-account tokens … MUST be re-read on every invocation"). `ANTHROPIC_API_KEY`, `ANTHROPIC_AUTH_TOKEN` and `ANTHROPIC_PROFILE` take precedence over federation. | Python `anthropic` 0.98.0, TypeScript `@anthropic-ai/sdk` 0.93.0, Go v1.39.0 (all 2026-05-04). `ANTHROPIC_WORKSPACE_ID` targeting: 0.99.0, 0.94.0, v1.40.0 (2026-05-05) |
| OpenAI [Python](https://github.com/openai/openai-python/blob/main/src/openai/auth/_workload.py), [Node](https://github.com/openai/openai-node/tree/master/src/auth) | No variable. The code passes `workload_identity={identity_provider_id, service_account_id, provider: {token_type: "jwt", get_token}}` (Node: `workloadIdentity`), and the SDK calls `get_token` at each exchange (RFC 8693 token exchange at `https://auth.openai.com/oauth/token`), 20 minutes before its access token expires. | Python `openai` 2.31.0, Node `openai` 6.34.0 (2026-04-08). Render's examples pass no client id, which needs Python 2.37.0 or Node 6.38.0 (2026-05-13) |

Render's Python example passes `"get_token": Path(TOKEN_PATH).read_text().strip`, a method bound to the string read when the client is built. Its first exchange after the token's hour fails. `examples/oidc-ai` passes a function that reads the file at each call, and its README says why. This is a defect in Render's example, not a platform difference.

**Proof.** `examples/oidc-ai` calls the Messages API with `anthropic.Anthropic()` and the Responses API with `openai.OpenAI(workload_identity=…)`, and refuses to call while a stored key is set. `scripts/oidc-audience-verify.sh` checks a running service that sets two or more triggers. Each token carries its own audience and the same `sub`. A MinIO STS trusting the workspace's issuer for each audience issues credentials for its own provider's token and refuses every other provider's. On dev-2 that held for a service with all three triggers (w2/m478 t003). Production acceptance against the real Claude Console and OpenAI organization is w2/m478 t004.

**Residual.** When a deploy drops one of several providers, the Secret loses that key while the old release's pods still list it. The kubelet then cannot update those pods' token volume for the rest of the rollout. Their other tokens go unrefreshed until the pods are replaced, and they stay valid for at least 40 minutes after their last refresh, as in D9's pre-deploy residual.

### D11 — Google Cloud (w2/m479)

Render, [render.com/docs/oidc](https://render.com/docs/oidc) (read 2026-11-18): a workload identity pool provider of type OIDC with issuer `https://oidc.render.com/<tea>`, audience "Default audience" and `google.subject = assertion.sub`. On the service: `GCP_WORKLOAD_IDENTITY_PROVIDER`, "Required. The resource name of the provider you created (starts with `projects/`). You can also paste the provider's full URL (starting with `https://iam.googleapis.com/`)", and `GCP_SERVICE_ACCOUNT_EMAIL`, "Optional … the email address of the service account to impersonate". "During the deploy, Render detects `GCP_WORKLOAD_IDENTITY_PROVIDER` and writes a credential configuration file for your service's OIDC token. Render then sets an additional environment variable, `GOOGLE_APPLICATION_CREDENTIALS`, to the path of the new file." Its troubleshooting: an invalid audience means the variable is not the full resource name, "Use your project number, not your project ID"; "Scope(s) must be provided" means the library must ask for `cloud-platform`. Render publishes no sample of the file.

**Trigger and inputs.** The trigger is `GCP_WORKLOAD_IDENTITY_PROVIDER`, with D9's rule. When the operator decides a release, it also reads `GCP_SERVICE_ACCOUNT_EMAIL` from the same environment and records both in `status.managedOIDC.gcp` (`appv1alpha1.NewManagedOIDCGCP`):

- `projects/<n>/locations/<l>/workloadIdentityPools/<p>/providers/<x>`, the same behind `https://iam.googleapis.com/` (Render's two spellings) or `//iam.googleapis.com/` (the audience a gcloud-written file spells), surrounding whitespace ignored, all give the same resource name.
- A project segment that is a project id (Google's grammar: a lowercase letter, then 4–28 lowercase letters, digits or hyphens, ending in a letter or digit) is refused by name. Anything else that is not that shape is refused too, and so is a service account email whose characters could not name an account in the impersonation URL. A refused release mounts no GCP file, so nothing is minted for an audience Google would refuse, and the deploy log says why: `==> GCP_WORKLOAD_IDENTITY_PROVIDER names project "my-project" by its ID: Google Cloud expects the project number (IAM & Admin > Settings) in the provider's audience, so no Google Cloud credential configuration is mounted`. The release's other providers still mount. Render is not known to check the value: its page documents the failure only as Google's "invalid audience" at run time. A deploy that fails on it would be stricter than Render, so bex names it and deploys.

**Delivery** is D9's, with two files. The token is `gcp-token`, for the default audience `https://iam.googleapis.com/<resource name>` (Google: "Tokens contain an `aud` claim with the following value: `https://iam.googleapis.com/projects/PROJECT_NUMBER/…`"). Beside it in `<app>-oidc` is `gcp-credentials.json`, and `GOOGLE_APPLICATION_CREDENTIALS=/var/lib/render/oidc/gcp-credentials.json` names it. Both file names are bex's and **unverified** against Render's. A value the service set itself for `GOOGLE_APPLICATION_CREDENTIALS` gives way when GCP mounts, as Render warns, and stays the service's own when GCP is refused. The minter writes the configuration with the token and rewrites both when a deploy changes the provider or the service account. A pod template that mounts GCP carries the inputs in the annotation `app.bex.co/managed-oidc-gcp`, so a cancel that restores a served release's template restores its configuration too.

**The file** is byte for byte what `gcloud iam workload-identity-pools create-cred-config <resource name> [--service-account=<email>] --credential-source-file=/var/lib/render/oidc/gcp-token --credential-source-type=text` writes (gcloud 593.0.0, pinned as `lego/backend/internal/managedoidc/testdata/gcloud-*.json`): `type` `external_account`, `audience` `//iam.googleapis.com/<resource name>`, the JWT subject token type, `https://sts.googleapis.com/v1/token`, and `credential_source` naming the token file as text. With an email it also carries `service_account_impersonation_url` (`https://iamcredentials.googleapis.com/v1/projects/-/serviceAccounts/<email>:generateAccessToken`); without one it carries gcloud's `token_info_url` (`https://sts.googleapis.com/v1/introspect`). `golang.org/x/oauth2/google`, the library under Google's Go client libraries' Application Default Credentials, builds credentials from the file, asks Google's STS for the `cloud-platform` scope with the file's token and the provider's audience, rereads the file at each exchange, and impersonates the account when one is named (`gcp_test.go`, against a stand-in for Google's endpoints). Production against a real pool is w2/m479 t003.

**Residual.** The configuration sits in the same Secret as the tokens, so D10's residual applies: a deploy that changes the provider rewrites the file the old release's pods read for the rest of the rollout.

### D12 — Build time (w2/m480)

Render, [render.com/docs/oidc](https://render.com/docs/oidc) § Limitations (read 2026-11-18): "OIDC _is_ available at build time for services on any native runtime." "Currently, OIDC auth is not available at build time for services that build from a Dockerfile." The page says nothing of how a build reads the token, so bex gives the build what an instance gets.

**Which builds.**

| Builder | Build-time token | Basis |
| --- | --- | --- |
| Native (`runtime` `node`, `python`, `ruby`, `go`, `rust` or `elixir`; a static site's build command, which runs in the native Node environment) | yes | Render's first sentence. A static site is a Render service whose build runs natively; Render does not single it out, so it is treated as native. |
| Build Source (`build_source`, `bds-…`) | no | Render's `sub` claim names a single `srv-…` service and documents no Build Source subject. Build source carriers never carry `app.bex.co/managed-oidc` and their native builds do not decide OIDC, so a carrier carrying a trigger variable builds normally instead of waiting on an unmintable token. |
| Dockerfile (`runtime: docker`, `builder: dockerfile`, and `auto`, which builds the repository's Dockerfile) | no | Render's second sentence. The service's own variables still reach it as build args (w1/m178), `AWS_ROLE_ARN` included, but no token, no file and no platform-set file variable. |
| `builder: buildpack` (kpack, a bex-only builder no Render surface selects) | no | kpack refuses `secretKeyRef` in build env (`.pm/w1/done/121.md`), so a token could only travel as a plaintext field of the Image CR. Its build already gets only `BP_*`/`BPE_*` variables. |

**The decision.** At a native build's dispatch the operator decides with D9's rules (a non-blank trigger, D11's GCP input check, the plan label) from the in-flight release's environment, which is the one the build command sees. It records the decision in `status.buildManagedOIDC`, whose generation is the release being built, before the build Job exists, and keeps it for the life of the build. A variable saved after dispatch reaches the next deploy, as it does at runtime. Below Pro the build gets nothing; the release's deploy log already says why (D9).

**Minting.** bex-api's minter writes the build's tokens, with the same subject, audiences and GCP configuration a release gets, to `<app>-oidc-build`. The Secret carries `app.bex.co/oidc-generation`, the build's release, so a new build never takes an earlier build's tokens, however young. It is refreshed like a release's while the App is `building`, and no longer once it is not, so a decision a canceled build leaves behind is not minted. The status watch wakes the minter when a build starts needing tokens. The served release's `<app>-oidc` is never touched: a build cannot change the files running instances read, nor drop a key their volume lists.

**Delivery.** The operator holds no key. It copies the minted files next to the build Job as `bld-<app>-oidc`, only once they carry the build's generation, and rewrites the copy on every pass while the build waits, so a queued build's Pod starts with a recent token. Until then it removes a copy an earlier build left. The Job lists every file in a required Secret volume, so the build does not start before its tokens exist (Render writes runtime tokens "before the service ever starts"). The files then take the secret-file path (w4/m163):

- Only the preparer mounts the Secret. It copies the files into the build's scratch directory, one snapshot per build.
- buildctl hands each file to BuildKit as a secret (`bex-oidc-<file>`).
- The authored build `RUN` alone mounts each file at its runtime path, `/var/lib/render/oidc/<file>`, `required=true`.
- The env loader exports each provider's file variable (`AWS_WEB_IDENTITY_TOKEN_FILE`, `GOOGLE_APPLICATION_CREDENTIALS`, `ANTHROPIC_IDENTITY_TOKEN_FILE`, `OPENAI_IDENTITY_TOKEN_FILE`) after the service's own values, so a value the service set itself gives way, as at runtime.

Nothing travels as a build arg, an `ENV`, a `COPY` or a file in the build context. No layer, image config or build log holds a token, and the image's environment names no token file, so an instance of a release without the trigger finds no dangling variable. `TestNativeManagedOIDCPinnedBuildKit` (`BEX_BUILDKIT_INTEGRATION=1`) checks this against the pinned BuildKit: the build command reads and checks every token, a Dockerfile build of the same options sees neither variable nor file, and no blob of the image carries a token. The same test fails when a token is passed as a build arg or copied into the image. BuildKit's cache key ignores secret contents. The provider set is in the loader, so a change to it reruns the build command; a new token alone does not.

**End.** When the operator observes the build succeed or fail, it clears the decision and deletes both Secrets. A copy in the build namespace is in the finalizer's inventory; the App owns `<app>-oidc-build`.

**Residuals.**

- A build reads a snapshot. A token is signed at most 20 minutes before the build Pod starts and lives 60 minutes, so a build command that runs for more than about 40 minutes may read an expired token. A build cannot refresh it. Render documents neither its build-time token's lifetime nor a refresh.
- A minter pass in flight when the build ends can write `<app>-oidc-build` once more. That token is no longer refreshed and expires within the hour, and the next build's end or the App's deletion removes it, as D9's "the last one expires on its own".
- A build waits for bex-api: with the minter down, a native build that sets a trigger sits with its Pod waiting on the copy until the build times out.

## Consequences

- ADR018's row "Managed OIDC for AI providers" names w2/m476–m480 as owners, and ADR030's open gap points at the same milestones. Both stay ✖ until delivery ships.
- `BEX_OIDC_ISSUER_URL` is documented in `lego/backend/AGENTS.md`.
- `scripts/oidc-issuer-verify.sh` checks production: a token minted for a workspace verifies through that workspace's discovery with go-oidc, fails against another workspace's issuer, and discovery has Render's keys.
- `scripts/oidc-aws-verify.sh` checks a running service (D9): its token file and variable, an `AssumeRoleWithWebIdentity` exchange at an STS trusting its workspace's issuer, a refusal for another audience, and the same across one refresh.
- A native build with a trigger reads its tokens (D12); `TestNativeManagedOIDCPinnedBuildKit` checks the transport against the pinned BuildKit.
- `scripts/oidc-audience-verify.sh` checks a running service with several providers (D10): each token is accepted only where its own audience is expected. For GCP it also checks that `GOOGLE_APPLICATION_CREDENTIALS` is an `external_account` file naming the token, and takes the audience from it (D11). Both scripts share `scripts/lib/oidc-sts.sh`.
