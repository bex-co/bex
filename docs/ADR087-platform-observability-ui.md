# ADR087 — Platform observability UI: Grafana at obs.bex.co behind the ops-workspace gate

**Status:** Accepted (2026-09-07). Source: platform-monitoring discussion 2026-09-07 (w5). Composes with [ADR001](ADR001-go-and-gitops.md) (product ≠ GitOps), [ADR010](ADR010-observability.md) (customer-facing logs/metrics), [ADR012](ADR012-auth.md) (Ory Kratos + Hydra), [ADR024](ADR024-members.md) (workspace members & roles), and the [ADR072](ADR072-security-review-round7.md) §1 `onbex.co` cookie-isolation disposition.

## Context

The platform GitOps baseline already runs the monitoring backends, all in the `monitoring` namespace that tenant pods are denied from reaching (`network-policies.yaml`): Prometheus scrapes Traefik and platform services and evaluates the alert rules Alertmanager emails out (`deploy/gitops/base/prometheus.yaml`), Loki stores durable log history (`loki.yaml`), and an Alloy DaemonSet ships App pod logs into it (`log-shipper.yaml`). What is missing is a human UI: today an operator reads these backends through kubectl port-forwards and raw PromQL/LogQL. [ADR010](ADR010-observability.md)'s logs and metrics are the **customer** product surface — per-service, tenant-scoped, served by bex-api — and are deliberately not a platform-operations view.

Three questions are decided here: where the UI runs, what hostname it gets, and who can log in.

## Decision

### 1. Grafana as platform GitOps infrastructure

Grafana is deployed by an Argo CD Application in `deploy/gitops/base/` (per-env differences in the overlays, like every sibling), into the `monitoring` namespace alongside the backends it reads. Datasources (Prometheus, Loki) and dashboards are **provisioned as code** — committed JSON, no click-ops state to lose. The initial dashboard set targets platform availability, bex-api errors/latency, operator reconciliation, builds, databases (CNPG), and cluster capacity; it may start small and grow, but every dashboard lives in git.

Explicit non-choices:

- **Not a bex-hosted customer App.** Our operations tooling must not depend on the product it exists to observe.
- **Not folded into the product dashboard.** Customer service metrics remain ADR010's surface; platform operations is a different audience with different data.
- **Not an outage-proof monitoring plane.** Grafana in the production cluster goes down with the cluster. The external uptime check remains the whole-cluster-outage detector; moving the observability stack (or a telemetry copy) outside the production cluster is deferred until blast-radius goals demand it.

### 2. Hostname: `obs.bex.co`

The existing platform hosts are role-named, never tool-named — `api.`, `auth.`, `oauth.`, `dashboard.bex.co`, not `kratos.` or `hydra.` — so the observability portal is **`obs.bex.co`** (letsencrypt-prod TLS, prod overlay). Rejected:

- `metrics.bex.co` — the UI fronts logs (and eventually traces) too, and "metrics" reads as a scrape/ingest endpoint, not a human portal.
- `grafana.bex.co` — welds today's tool choice into DNS, certificates, bookmarks, and the OAuth redirect URI.
- anything under `onbex.co` — ADR072 §1: `onbex.co` is not on the PSL, so every tenant origin shares its registrable-domain cookie scope; an admin-authenticated UI must stay on the control-plane domain, out of tenant cookie-tossing range.

### 3. AuthN: an ordinary Hydra OIDC client

Grafana authenticates against the existing platform issuer (`https://oauth.bex.co`) as one more first-party client — no new identity system:

- Registered idempotently by `scripts/auth-bootstrap-client.sh` beside `bex-mobile`: confidential client, `authorization_code` + `refresh_token`, exact redirect `https://obs.bex.co/login/generic_oauth`, `skip_consent: true`.
- Scopes are identity-only (`openid profile email`) with **no access-token audience** — a Grafana token carries zero bex-api authority, so this adds no surface to the product API.
- Grafana's generic-OAuth config enables PKCE S256 — the consent gate (`hydra-consent.ts` `pkceSatisfied`) refuses any authorization-code flow without it.
- Login rides the existing Kratos-native bridge; an operator with a live dashboard session reaches Grafana with no consent screen (the trusted headless path).
- The client secret and Grafana's break-glass local-admin password live in SealedSecrets ([ADR016](ADR016-sealed-secrets.md)); names-only in `.env.example`.

### 4. AuthZ: the ops-workspace gate

Kratos is the **customer** identity pool, so authentication alone would admit every bex user. The gate is membership in a designated ops workspace — an ordinary `tea-*` workspace pinned by deployment config, whose membership (ADR024 tuples, invites, members UI) becomes the Grafana ACL:

- **bex-api** learns the id via `BEX_OPS_WORKSPACE` and exposes a server-only internal verb answering "what is subject S's role in the ops workspace" (one OpenFGA read), guarded by a static bearer (`BEX_OPS_ROLE_TOKEN`) and reachable only in-cluster.
- **The consent acceptor** (`dashboard/src/common/server-fn/hydra-consent.ts`) gains an ops-gated client class (`OAUTH_OPS_CLIENTS`, today just Grafana's client id): before **any** accept — including the trusted/skip headless path — it resolves `consent.subject`'s role through that verb. `admin` → GrafanaAdmin, `developer` → Editor, `viewer` → Viewer; `contributor`, `billing`, and non-members are rejected with `access_denied`. On accept it stamps `email`, `name`, and `ops_role` id_token claims. The gate runs before either path's accept call, so the "headless and human paths grant identically" invariant survives.
- **Grafana** maps `ops_role` via `role_attribute_path` with `role_attribute_strict` — defense in depth behind the server-side gate, not the gate itself. Token/session lifetimes stay short: role changes and removals take effect at next login (plus the 1h consent remember window), not mid-session.

The pinned workspace gets two guards: workspace deletion/suspension refuse it outright, and invite-time seat/plan gating (ADR024) exempts it — onboarding an operator must never be silently blocked by a seat cap, and deleting the workspace must never lock every operator out at once.

**Alternative considered:** granting operators tuples on `workspace:default` (the platform object ADR012 reserves for bootstrap) — no product-lifecycle semantics to guard, but no members UI or invite flow either; operator management would be out-of-band tuple writes. Rejected for now: the members surface is exactly the operator-management UX we want, and the consent-side gate is identical under either model, so switching later is cheap.

### 5. Environments and verification

The local overlay runs Grafana with only the local admin — no OIDC client, no public host — the same reduced shape as its other components; prod carries the full OIDC config. Verification extends the auth e2e family (`scripts/auth-oauth21-e2e.sh` pattern): an ops-workspace member completes the code flow and lands with the mapped role; a customer identity outside the ops workspace is denied at consent.

## Consequences

- New env vars enter the cascading inventories: `BEX_OPS_WORKSPACE` + `BEX_OPS_ROLE_TOKEN` (backend, [lego/backend/CLAUDE.md](../lego/backend/CLAUDE.md)); `OAUTH_OPS_CLIENTS` + the role-verb URL/token (dashboard SSR, [dashboard/CLAUDE.md](../dashboard/CLAUDE.md)).
- The consent acceptor acquires its first identity-conditional client class; every other client's behavior is byte-identical.
- The dashboard SSR runtime gains a server-to-server call into bex-api — new coupling, kept to one internal verb.
- Publishing the manifests exposes `obs.bex.co`'s existence and our dashboards' shape. Consistent with the repo's posture: alert rules and platform hostnames are already public, hostnames appear in CT logs regardless, and secrets never enter git.
