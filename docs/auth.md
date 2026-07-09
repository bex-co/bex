# ADR: platform auth — Ory Kratos (identity) + Ory Hydra (OAuth2/OIDC)

**Status:** accepted and consumed (substrate deployed as GitOps base components and live in prod; bex-api validates against it — see [bex-api.md#auth](bex-api.md)). The static `BEX_API_TOKEN` is **gone** (w4/m3): every machine caller holds an **API key** = its own OAuth2 client in Hydra, exchanged for short-lived tokens; the first key (`bex-bootstrap`) is seeded by `scripts/auth-bootstrap-client.sh` on every deploy, and further keys are minted via bex-api's `/v1/api-keys`.

## Context

bex-api used to authenticate every caller with **one static bearer token** (`BEX_API_TOKEN`; removed in w4/m3). That was fine for a single operator but is _the_ blocker for multi-tenancy: the vision's control plane needs tenants and accounts (roadmap #1), and the AI-native pillars need **per-client credentials** — an agent that deploys from chat must hold its own revocable token, not the shared admin secret. So the platform needs real identity (who is this?) and real tokens (what may this credential do, until when?) as infrastructure that bex-api and the control plane consume.

## Decision

### 1. Buy Ory, don't build auth into the control plane

Identity + OAuth2 is commodity infrastructure with a brutal correctness bar (password hashing, session fixation, token semantics, OIDC conformance). Building it into bex's Go control plane means owning that bar forever; renting it (Auth0/Clerk) contradicts an open-source, self-hostable platform. **Ory** is the fit:

- **Apache-2.0, CNCF-adjacent, API-first** — same governance test CNPG passed ([postgresql-management.md](postgresql-management.md) §1).
- **Headless** — Kratos/Hydra are pure APIs; bex keeps its own UX and the Render-compatible API surface.
- **Kubernetes-native** — first-party Helm charts, stateless pods, all state in Postgres (which we already operate well via CNPG).

**Keycloak** was the main alternative — one monolith with an admin UI. Rejected: JVM footprint on a small cluster, realm-centric model that fights an API-first control plane, and its own embedded-ish DB story. Ory splits into small Go services that match how bex is built.

### 2. Kratos vs Hydra — the split of responsibilities

Two services, not one, because the jobs are different:

| service | job | consumed by |
| --- | --- | --- |
| **Kratos** | _identity_: accounts, credentials (email + password to start), self-service flows, admin identity CRUD | tenant sign-up/login; bex-api sessions (w4/m2) |
| **Hydra** | _OAuth2/OIDC_: client registration, token issuance (`client_credentials` first — that's the agent story), introspection | agents/CI holding per-client tokens; bex-api introspection middleware (w4/m2) |

Each has a **public** listener (self-service / OAuth2 endpoints — exposed) and an **admin** listener (identity CRUD / client management / introspection — **cluster-internal only**, no ingress).

```mermaid
flowchart LR
  agent@{ shape: tri, label: "agent / CI · client_credentials" }
  user@{ shape: tri, label: "tenant" }
  agent --> edge["Traefik<br/>oauth.bex.co · auth.bex.co"]
  user --> edge
  edge --> hydra["Hydra public :4444<br/>token + discovery"]
  edge --> kratos["Kratos public :4433<br/>self-service"]
  bexapi["bex-api (w4/m2)"] --> hydraadm["Hydra admin :4445<br/>clients + introspect<br/>(cluster-internal)"]
  bexapi --> kratosadm["Kratos admin :4434<br/>identity CRUD<br/>(cluster-internal)"]
  hydra & hydraadm --> hdb[("hydra-db<br/>CNPG")]
  kratos & kratosadm --> kdb[("kratos-db<br/>CNPG")]
```

### 3. Two CNPG clusters, one database each

Ory hard-requires Kratos and Hydra **not to share a database**. We go one step further — a dedicated CNPG `Cluster` per component ([charts/auth-dbs](../deploy/gitops/charts/auth-dbs/), namespace `auth`, one Argo Application for both since they share sizing and lifecycle policy), mirroring `bex-db`:

- **Independent failover/upgrade/backup** — an identity-store migration can't take down token issuance, and vice versa.
- **CNPG-recommended shape** — one application database per cluster; CNPG generates the `<cluster>-app` credential Secret and the `<cluster>-rw` Service that the DSNs point at.
- Auth state (identities, clients, tokens) lives in Postgres, **not in the pods** — pods are stateless and restartable; this is verified explicitly (below).

### 4. GitOps shape

Everything is an Argo Application in `deploy/gitops/base/`, synced in waves: CNPG operator (1) → `auth-dbs` (2) → `kratos`/`hydra` (3). The Ory apps are **multi-source**: the pinned upstream chart (`ory/kratos`, `ory/hydra`, chart 0.62.1 = app v26.2.0) from the Helm repo + the vendored values from this repo ([base/values/kratos.values.yaml](../deploy/gitops/base/values/kratos.values.yaml), [base/values/hydra.values.yaml](../deploy/gitops/base/values/hydra.values.yaml)). The local overlay patches DB sizing (1Gi, `local-path`) and layers `overlays/local/values/` on top of the base values (local hosts over plain HTTP, Hydra dev mode).

Public endpoints ride the standard edge ([custom-domain.md](custom-domain.md)): chart-native `Ingress` objects, `ingressClassName: traefik`, cert-manager `letsencrypt-prod` certs — `auth.bex.co` (Kratos public) and `oauth.bex.co` (Hydra public; equals Hydra's `urls.self.issuer`, which OIDC requires to match). The admin Services have **no** Ingress.

### 5. `dashboard/` is Kratos's "custom UI" (w5)

Kratos ships no UI of its own — it expects a "bring your own UI" consumer, registered via `selfservice.flows.*.ui_url`. That consumer is [`dashboard/`](../dashboard) (`docs/vision.md`'s human-facing surface): its login/registration/recovery/settings pages render [`@ory/elements-react`](https://www.ory.com/docs/elements)'s flow components against the same Kratos instance, not a hand-rolled auth backend (`dashboard/README.md#authentication`). `kratos.values.yaml` wires this up:

- `selfservice.flows.{login,registration,recovery,settings,verification,error}.ui_url` → `dashboard.bex.co/auth/*` (+ `/settings`) — without these, Kratos redirects self-service flows back to its own domain instead of the dashboard.
- `selfservice.allowed_return_urls` — whitelists the dashboard's origin for the `return_to` param the flow-initiation redirect carries.
- `serve.public.cors` — `dashboard.bex.co` is a different origin from `auth.bex.co`; the dashboard's credentialed cross-origin fetches need explicit CORS + `allow_credentials: true`.
- `session.cookie.domain: bex.co` — scopes the session cookie to every `*.bex.co` subdomain so it's sent as same-site (not cross-site) from the dashboard to Kratos, avoiding the weaker `SameSite=None`.
- `selfservice.flows.registration.after.password.hooks: [{hook: session}]` — Kratos does **not** auto-issue a session after registration by default; without this hook the user registers successfully but lands back at the login screen with no session (a real, easy-to-miss gotcha — confirmed by driving an actual browser through the flow against the local mock cluster).
- Recovery initializes but can't deliver mail until `courier.smtp` lands (courier is disabled — no SMTP yet); see Consequences.

**Local dev**: `auth.bex.local`'s Ingress has no DNS/hosts entry and Traefik isn't exposed to the host, so it's unreachable from a laptop-run dashboard dev server. `overlays/local/values/kratos.values.yaml` instead points `base_url`/the flow `ui_url`s at `localhost:4433`/`localhost:5173` — reach Kratos via `kubectl -n auth port-forward service/kratos-public 4433:80` (the same pattern `scripts/auth-verify.sh`/`scripts/auth-e2e.sh` already use), then `VITE_KRATOS_PUBLIC_URL=http://localhost:4433 yarn dev` in `dashboard/`.

**Deployment.** Unlike bex-api, the dashboard doesn't ship in the operator image (it's a separate Node.js/Vite app) — it's its own image (`dashboard/Dockerfile`) built and rolled by `.github/workflows/deploy.yml` the same way the operator is, deployed via its own Argo Application (`deploy/gitops/base/dashboard.yaml`) pointing at `dashboard/deploy` (kustomize: Deployment + Service + Ingress at `dashboard.bex.co`), namespace `dashboard`. Its `VITE_KRATOS_*`/`VITE_*API_URL` values are Vite **build-time** values (Dockerfile `ARG`s) baked into the JS bundle — the SSR ones (`VITE_KRATOS_SSR_URL`, `VITE_SSR_API_URL`) point at in-cluster Service DNS (`kratos-public.auth.svc:80`, `bex-api.bex-system.svc:8090`) since the dashboard's own pod reaches them directly rather than back out through the public ingress; changing them means rebuilding the image, not just setting a container env var. Verified end-to-end against the local mock cluster: built the image, `kind load docker-image`d it in, applied `dashboard/deploy` directly (Argo isn't installed on the mock cluster — same as how kratos/hydra config changes were applied directly via `helm upgrade` above), then drove a real browser through registration/login/logout against the running pod (port-forwarded to the laptop, `kubectl -n dashboard port-forward service/dashboard 5173:80`, reusing the same `localhost:5173` the local Kratos `ui_url`s already point at).

One real deploy-specific bug this surfaced: the Dockerfile's runtime stage never dropped to a non-root user, but the Deployment requires `runAsNonRoot: true` — Kubernetes refused to start the container (`CreateContainerConfigError`). Fixed with `USER node` (Dockerfile) + explicit `runAsUser: 1000`/`runAsGroup: 1000` (Kubernetes can't resolve a _named_ user for the non-root check, only a numeric UID).

### 6. Secrets: out-of-band, never in git

The charts' built-in Secret rendering is **disabled** (`secret.enabled: false`) so no secret material can appear in git or Argo state. [scripts/auth-secrets.sh](../scripts/auth-secrets.sh) creates the two Secrets (`auth/kratos`, `auth/hydra`) out-of-band:

- **DSNs are composed, not stored** — the script reads the CNPG-generated `kratos-db-app`/`hydra-db-app` credentials and builds `postgres://…@<cluster>-rw.auth.svc:5432/…?sslmode=require`. DB passwords never live in `.env`.
- **Ory secrets come from `.env`** (gitignored, same rule as `bex.kubeconfig`): `KRATOS_SECRETS_{DEFAULT,COOKIE,CIPHER}`, `HYDRA_SECRETS_{SYSTEM,COOKIE}`, `HYDRA_OIDC_PAIRWISE_SALT`. The script validates presence/length and never echoes values; `DRY_RUN=1` prints resource names only.
- **Prod path**: `scripts/gh-secrets.sh` pushes the six keys from `.env` into GitHub Actions secrets, and `deploy.yml`'s "apply auth secrets" step runs `auth-secrets.sh` against the Hetzner app cluster on every deploy (it waits for the CNPG-generated credentials first; the kratos/hydra Applications carry a sync `retry` so the first sync self-heals once the Secrets exist).
- Production-grade committed secrets (SOPS/sealed-secrets) are w1/m7 t003; when that lands, these Secrets become sealed and the script retires.

### 7. OAuth 2.1 provider for agents: one dashboard login, first-party sessions + third-party clients (w4/m9)

The dashboard itself never uses OAuth — it authenticates with its Kratos session cookie, full stop (`.pm/DO_NOT_DO.md`: browser-held tokens are banned; IETF `draft-ietf-oauth-browser-based-apps` ranks them last, and Ory's guidance is "first-party = sessions, Hydra = third-party/machine"). What w4/m9 adds is the **provider side**: a third-party OAuth 2.1 client — an MCP agent like Claude Code — self-registers, sends its user through **the same dashboard login page**, and gets a user-consented access token for bex-api. Four pieces, none of them a custom login provider:

- **Kratos's native bridge does the login.** `oauth2_provider.url: http://hydra-admin.auth.svc:4445` (+ `override_return_to`) in [kratos.values.yaml](../deploy/gitops/base/values/kratos.values.yaml) makes **Kratos itself** resolve and accept Hydra's login challenges. Hydra's `urls.login` points at the dashboard login page, which only **passes `login_challenge` through** when creating the Kratos flow (`use-ory-flow.ts`; OAuth-linked flows are minted fresh, never resumed from storage, and never persisted). An existing session short-circuits with no UI (Kratos answers the AJAX call with `browser_location_change_required` → the hook follows `redirect_browser_to`); a stale challenge degrades to the ordinary login page — the challenge is advisory, never load-bearing.
- **Consent is a headless dashboard route.** Hydra never skips the consent _redirect_ (by design); the canonical pattern is to accept instantly for trusted clients. `urls.consent` points at `/auth/consent` — a server-only TanStack Start handler ([hydra-consent.ts](../dashboard/src/common/server-fn/hydra-consent.ts)) that auto-accepts when Hydra marks the request skippable (`skip`, incl. `skip_consent` clients) or the client is in `OAUTH_TRUSTED_CLIENTS`, and **denies unknown clients** (a real consent UI is future work). Its `HYDRA_ADMIN_URL` env is deliberately not `VITE_`-prefixed — server-only, never in the client bundle.
- **Agents self-register and self-discover.** Hydra's OIDC **Dynamic Client Registration** is enabled (`oidc.dynamic_client_registration.enabled`, public `POST /oauth2/register` on Hydra 2.x — verified live); registered clients are _not_ auto-trusted. bex-api serves **RFC 9728 protected-resource metadata** (`GET /.well-known/oauth-protected-resource`, open by design) and enriches its 401s with `WWW-Authenticate: Bearer resource_metadata="…"`, so an agent that hits `/mcp` unauthenticated can find the authorization server per the MCP authorization spec. Both are gated on `BEX_OAUTH_ISSUER` + `BEX_OAUTH_RESOURCE` (unset ⇒ byte-identical prior behavior).
- **Audience discipline (RFC 8707, the honest subset).** Hydra doesn't implement RFC 8707's `resource` parameter — it has its own `audience` request param — so bex-api enforces the MCP spec's audience check pragmatically: a token whose introspected `aud` is non-empty must include `BEX_OAUTH_RESOURCE` or it's rejected; **empty-`aud` tokens stay accepted** because client_credentials API keys carry no audience and must keep working. Agents that request `audience=<resource>` at authorize time get the full check.

**Verified end-to-end** by [scripts/auth-oauth21-e2e.sh](../scripts/auth-oauth21-e2e.sh) — throwaway dockerized Hydra + Kratos (in-memory), the real dashboard consent route, the real bex-api: RFC 9728 discovery → DCR → operator blesses the client (`skip_consent` PATCH — how a real trusted agent is onboarded) → authorize (PKCE S256) → Kratos-native login-challenge accept → headless consent → code → token (access + refresh) → `Authorization: Bearer` passing bex-api introspection + audience check. Run it locally with Docker; on the mock cluster the same config ships via the kratos/hydra value overlays.

### 8. API-key hygiene: access-token TTL + key metadata (w4/m13)

The API-key surface is [ahead of Render](render-parity.md#bex-ahead-of-render) (Render mints keys dashboard-only; bex makes them first-class, revocable OAuth2 clients over REST/GraphQL/MCP), so it carries its own basic hygiene rather than inheriting Render's.

- **Access-token TTL is deliberate, not the default.** `hydra.config.ttl.access_token: 15m` ([hydra.values.yaml](../deploy/gitops/base/values/hydra.values.yaml)) tunes token lifetime down from Hydra's 1-hour default. API keys use `client_credentials`, which issues **no refresh token**, and bex lets any caller re-mint a token freely — so a short window bounds a leaked or stale access token's blast radius while costing callers only a cheap re-mint. 15m is the balance point against introspection load: bex-api caches positive introspections ≤30s ([`core.PositiveTTL`](../lego/backend/internal/core/http.go)), so the extra re-issuance traffic is negligible. The knob is global, so OAuth 2.1 user access tokens shorten too — they carry refresh tokens, so it's transparent to those callers. **Distinct from revocation latency:** deleting a key stops _new_ tokens immediately, but an already-issued token stays `active` until its own `exp` (bounded by this TTL) minus bex-api's ≤30s introspection cache.
- **Keys carry `created-by` + `last-used`, recorded off the hot path.** Each bex-minted Hydra client stores `bex.co/created-by` (the minting caller's `IdentityFrom` subject — a Kratos identity id or client_id) in its `metadata` at create time, and `bex.co/last-used` (an RFC 3339 timestamp) written from the introspection path. The last-used write is **never a per-request store write**: the auth gate calls a fire-and-forget `TouchAPIKey` after a successful API-key introspection, throttled in-memory to **at most one Hydra write per key per minute** and executed on a background goroutine, so a chatty agent adds nothing synchronous to its request path (and a non-API-key client — a platform or OAuth 2.1 agent client — is filtered out by the marker check before any write). Both fields surface on every list surface (REST/GraphQL/MCP) and the Settings → API Keys page; the create response omits `last-used` (a fresh key has never been used). This provenance + last-used pair is the prerequisite for any future stale-key or rotation policy.

## Authorization (OpenFGA)

Authentication says who is calling; **authorization** says what they may touch. bex uses **OpenFGA** (Zanzibar-style ReBAC, Apache-2.0/CNCF — the same governance test CNPG and Ory passed) as a fourth base component: [base/openfga.yaml](../deploy/gitops/base/openfga.yaml) (pinned chart 0.3.10, cluster-internal only — no ingress, playground off, preshared-key API) against its own CNPG cluster `openfga-db` ([charts/auth-dbs](../deploy/gitops/charts/auth-dbs/)).

**Why OpenFGA and not…** _roles in the control-plane Postgres_ — role checks ossify into `if admin` conditionals; relations ("member of the tenant that owns this app") are the actual product semantics and stay queryable both ways (list what X can see). _Ory Keto_ — would keep the Ory family, but OpenFGA has the healthier ecosystem, a first-class model DSL + CLI, and a CNCF track; Keto's Zanzibar implementation has lagged. The model is small enough that switching later is a rewrite of one file and one seam implementation.

**The model** ([deploy/gitops/authz/model.fga](../deploy/gitops/authz/model.fga), applied as `model.json`) **mirrors Render's permissions management** (render.com/docs/team-members, fetched 2026-07-06) so bex stays Render-consistent in authorization exactly as it is in API shape: an `org` layer (Render Organizations — org admins administer every member workspace) above `workspace` (Render's workspace/team) with Render's five member roles and their documented capability split:

| role · permission | can_view | can_view_logs | can_operate | can_create | can_view_sensitive | can_manage_keys | can_manage | can_manage_billing |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| **admin** | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |
| **developer** | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |  |  |
| **contributor** | ✓ | ✓ | ✓ |  |  |  |  |  |
| **viewer** | ✓ |  |  |  |  |  |  |  |
| **billing** | ✓ |  |  |  |  |  |  | ✓ |

(Render verbatim: viewers are read-only and "can't view service logs or sensitive fields"; contributors "can't view sensitive fields (billing info, connection strings, environment variables)" and can't create/delete most resources; developers get full resource access but no org settings; billing gets billing only.) `service`/`postgres`/`api_key` objects are workspace-ownable for per-object tuples later. Since w1/m9 each check targets the caller's workspace — `workspace:tea-<id>` for a tenant member or a bound API key — with `workspace:default` reserved for the platform bootstrap (`bex-bootstrap`) and the legacy store-off mode. Subjects are exactly the strings the auth gate resolves (`api.IdentityFrom`): Hydra `client_id`s and Kratos identity ids. The full role×permission matrix was verified live against OpenFGA on the mock cluster.

**Verified against Render's live dashboard** (2026-07-06, [docs/render-artifacts/team-members.graphql](render-artifacts/team-members.graphql)): the Team Members tab's own GraphQL is `usersAndPendingForTeam(ownerId) { owner { usage { users { used limit } } team { members { role active user{…} } pendingInvites { id email role expiresAt } } } }`, and a member's `role` comes back as the **UPPERCASE** enum (`"ADMIN"` confirmed live; `DEVELOPER`/`CONTRIBUTOR`/`VIEWER`/`BILLING` per the docs). Render uses three interchangeable words for this entity — **workspace** (UI), **team** (`owner.team`, id `tea-…`), **owner** (the polymorphic resource parent, user-or-team). bex's `workspace` type _is_ that entity, and its five roles + capability split match Render's exactly. bex's members/roles API shipped in w4/m12 ([members.md](members.md)): invite by email / list / change-role / remove across REST·GraphQL·MCP·UI, writing `tenant_members` rows and the OpenFGA role tuples together, mapping the UPPERCASE wire enum to these lowercase FGA relations, with invites redeemed into a membership on the recipient's first login. It flattens Render's `owner.team` nesting into workspace-scoped queries (bex has no polymorphic `owner`) and keys members by identity subject rather than `user{email}` — the two recorded divergences.

**Enforcement** lives in bex-api's Core (one guard per verb, mapped to the Render matrix — lists/details/metrics ⇒ `can_view`, logs ⇒ `can_view_logs`, restart/suspend/resume ⇒ `can_operate`, create/delete resources ⇒ `can_create`, connection-info ⇒ `can_view_sensitive`, API keys ⇒ `can_manage_keys`) through the injected `Checker` seam (`lego/backend/internal/api/authz.go`):

- `BEX_OPENFGA_URL` **unset ⇒ nil checker ⇒ every verb allowed** — the pre-authorization behavior, byte-for-byte. Setting the URL is the deliberate enforcement flip (set in [lego/operator/config/api/deployment.yaml](../lego/operator/config/api/deployment.yaml) since w1/m9, paired with `BEX_CP_DB_URI`): tenant onboarding now mints a workspace on a human's first login and binds each minted API key to its tenant, so enforced authz no longer 403s every freshly minted key.
- Wired ⇒ **fail closed**: denial is 403 (`ErrForbidden`, distinct from the gate's 401), an unreachable OpenFGA is 503, never a pass-through. Positive checks cache ≤ 30s (revocation latency bound), negatives never (fresh grants apply immediately).
- The **stdio MCP transport never wires the checker** (main.go): its trust boundary is the subprocess itself — no auth gate, no identity, so a wired checker would deny every tool. A reflection sweep in the test suite (`TestAuthzGuardsEveryCoreVerb`) asserts every exported Core verb carries its guard, so a new verb can't silently ship unguarded.

**Ops**: [scripts/authz-model.sh](../scripts/authz-model.sh) idempotently ensures the `bex` store, applies `model.json` only when it differs from the latest applied model (models are append-only), and seeds `bex-bootstrap → admin of workspace:default`; deploy.yml runs it after the OpenFGA rollout on every deploy. bex-api's copy of the preshared key lives in `bex-system/bex-openfga` (written by `auth-secrets.sh`, `.env` key `OPENFGA_PRESHARED_KEY`).

**Session callers (the dashboard) need no special-casing.** The auth gate resolves a Kratos session to the same `core.Identity{Subject, Method: "session"}` shape a Hydra bearer resolves to `Method: "oauth2"`; `Base.Authorize` checks both as `"user:" + id.Subject` against the caller's workspace — `workspace:tea-<id>` when the control-plane store resolves the caller to a tenant, `workspace:default` otherwise — so every verb — including the api-key mint/list/revoke a dashboard user drives from Settings (w4/m8, [bex-api.md#auth](bex-api.md#auth) has the UI-side details) — is reachable and enforced identically regardless of which credential authenticated the request (`TestAPIKeys_SessionCaller`, `internal/api/server_test.go`). Provisioning is automatic since w1/m9: a human's first authenticated call mints a personal tenant (`tenant_members` row + `user:<kratos-id> admin workspace:tea-<id>` grant), and each minted API key is bound to the caller's tenant (another `tenant_members` row, role `developer` + `user:<client_id> developer workspace:tea-<id>` grant — the same table w6/m1's workspace lifecycle writes, since `subject` covers both Kratos identity ids and Hydra client ids). `bex-bootstrap` remains the one subject seeded by `authz-model.sh` on `workspace:default` — the platform operator that mints the first tenant out of band.

## Verification

`scripts/auth-verify.sh` (exit 0 = pass, used as the milestone's behavioral test) proves on the local mock cluster, via port-forwards to the cluster-internal Services:

1. **Identity** — `POST /admin/identities` (Kratos admin) creates an email identity; it reads back.
2. **Token** — `POST /admin/clients` (Hydra admin) registers a client; the public `POST /oauth2/token` completes `client_credentials`; admin introspection returns `active: true`.
3. **Negative** — a wrong client secret yields an OAuth2 error and no token; a garbage token introspects `active: false`.
4. **Durability** — after `kubectl rollout restart` of both Deployments, the identity still exists and the same client still gets tokens: state is in the CNPG clusters, not the pods.

Local bring-up order (what Argo's waves do in prod): mock cluster → local-path provisioner → CNPG operator → DB Clusters → `scripts/auth-secrets.sh` → Kratos/Hydra charts (base values + the local overlay's inline overrides) → `scripts/auth-verify.sh`.

Local-CAPD quirks (mock cluster only, none apply to prod):

- **Pin everything to the control-plane node** (nodeSelector `node-role.kubernetes.io/control-plane: ""` + the matching toleration) — worker-node pods can't reach the apiserver or cross-node services under OrbStack/Calico, the same workaround docs/deployment.md documents for the bex operator. `scripts/mock-cluster.sh` pins **coredns** for this reason; the **local-path provisioner**, the CNPG operator, and the auth pods need the same pin.
- **PSA**: the cluster enforces `baseline` — label the `auth` namespace `pod-security.kubernetes.io/enforce=privileged` so local-path's hostPath helper pods (created in the PVC's namespace) can run.

## Alternatives considered

- **Keep the static token + roll our own users table** — no token revocation, no scopes, no standard for agents to hold credentials; every future integration reinvents OAuth2 badly. Rejected.
- **Auth0 / Clerk / hosted IdP** — SaaS dependency inside a self-hostable open-source platform; every bex installation would need a third-party account. Rejected.
- **Keycloak** — capable but monolithic (see §1). Rejected.
- **One shared Postgres for both Ory services** — explicitly unsupported by Ory (shared DB), and a shared CNPG cluster couples their lifecycles for a ~256Mi saving. Rejected.
- **hydra-maester (CRD-managed OAuth2 clients)** — disabled; clients will be managed through the admin API by bex-api (w4/m2), keeping the control plane the source of truth.

## Consequences

- bex-api is hard-dependent on Hydra by design: introspection outage ⇒ the whole API 503s (fail closed). Operational recovery during an Ory incident goes through kubectl, not bex-api — accepted trade-off of deleting the shared-secret escape hatch.
- Like `bex-db`, both auth DBs are `instances: 1` with no backup schedule yet — **`kratos-db`/`hydra-db` must join the w1/m7 backup/HA work**; losing them loses all identities and clients.
- No SMTP is configured, so Kratos' courier (verification/recovery mail) is off; email flows need an SMTP secret + `courier.enabled: true` later.
- Prod DNS records for `auth.bex.co`/`oauth.bex.co` must exist before first sync, or ACME HTTP-01 will retry until they do.
