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

(Render verbatim: viewers are read-only and "can't view service logs or sensitive fields"; contributors "can't view sensitive fields (billing info, connection strings, environment variables)" and can't create/delete most resources; developers get full resource access but no org settings; billing gets billing only.) `service`/`postgres`/`api_key` objects are workspace-ownable for per-object tuples later. Until the control plane grows real workspaces (w1/m2), every check targets the single `workspace:default`. Subjects are exactly the strings the auth gate resolves (`api.IdentityFrom`): Hydra `client_id`s and Kratos identity ids. The full role×permission matrix was verified live against OpenFGA on the mock cluster.

**Verified against Render's live dashboard** (2026-07-06, [docs/render-artifacts/team-members.graphql](render-artifacts/team-members.graphql)): the Team Members tab's own GraphQL is `usersAndPendingForTeam(ownerId) { owner { usage { users { used limit } } team { members { role active user{…} } pendingInvites { id email role expiresAt } } } }`, and a member's `role` comes back as the **UPPERCASE** enum (`"ADMIN"` confirmed live; `DEVELOPER`/`CONTRIBUTOR`/`VIEWER`/`BILLING` per the docs). Render uses three interchangeable words for this entity — **workspace** (UI), **team** (`owner.team`, id `tea-…`), **owner** (the polymorphic resource parent, user-or-team). bex's `workspace` type _is_ that entity, and its five roles + capability split match Render's exactly. When bex grows a members/roles API (w1/m2), it mirrors that captured contract — `owner`/`team` nouns, the UPPERCASE `role` enum — the same Render-consistency rule the service/postgres/logs verbs already follow, with bex-api mapping the wire enum to these lowercase FGA relations.

**Enforcement** lives in bex-api's Core (one guard per verb, mapped to the Render matrix — lists/details/metrics ⇒ `can_view`, logs ⇒ `can_view_logs`, restart/suspend/resume ⇒ `can_operate`, create/delete resources ⇒ `can_create`, connection-info ⇒ `can_view_sensitive`, API keys ⇒ `can_manage_keys`) through the injected `Checker` seam (`lego/backend/internal/api/authz.go`):

- `BEX_OPENFGA_URL` **unset ⇒ nil checker ⇒ every verb allowed** — the pre-authorization behavior, byte-for-byte. Setting the URL is the deliberate enforcement flip (commented in [lego/operator/config/api/deployment.yaml](../lego/operator/config/api/deployment.yaml); off in prod until tenant onboarding grants minted keys their membership tuples — today a freshly minted key would 403 on everything).
- Wired ⇒ **fail closed**: denial is 403 (`ErrForbidden`, distinct from the gate's 401), an unreachable OpenFGA is 503, never a pass-through. Positive checks cache ≤ 30s (revocation latency bound), negatives never (fresh grants apply immediately).
- The **stdio MCP transport never wires the checker** (main.go): its trust boundary is the subprocess itself — no auth gate, no identity, so a wired checker would deny every tool. A reflection sweep in the test suite (`TestAuthzGuardsEveryCoreVerb`) asserts every exported Core verb carries its guard, so a new verb can't silently ship unguarded.

**Ops**: [scripts/authz-model.sh](../scripts/authz-model.sh) idempotently ensures the `bex` store, applies `model.json` only when it differs from the latest applied model (models are append-only), and seeds `bex-bootstrap → admin of workspace:default`; deploy.yml runs it after the OpenFGA rollout on every deploy. bex-api's copy of the preshared key lives in `bex-system/bex-openfga` (written by `auth-secrets.sh`, `.env` key `OPENFGA_PRESHARED_KEY`).

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
