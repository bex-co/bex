# w1 · m2 — Control plane: the Postgres source of truth (in `lego/backend`)

**Worker:** worker1 **Goal:** Grow the bex control plane — the Postgres-backed **source of truth** (tenants/apps/domains/plans) that projects `apps` rows into `App` CRs for the operator to execute, and gives a product API. It is **not a separate service**: it lands in the existing **`lego/backend`** module (today bex-api) as `internal/store` + a projector, sharing the one backend Deployment. Fixes business data living only in single-node etcd. **Status:** implemented + committed (`aebbd43`) — open: t007 live end-to-end acceptance (prod `BEX_CP_DB_URI` still commented out in `lego/operator/config/api/deployment.yaml`)

> **Post-lego structure (decided w5).** The refactor split the Go into `lego/{types,operator,backend}`. The control plane is the _source-of-truth half_ of the **backend** (business-logic) module — package **`lego/backend/internal/store`** (the `controlplane → store` rename; "control plane" is a concept, not an identifier — Kubernetes already owns the word). **No third binary** — `operator/cmd/controlplane` is off the plan; the store + projector run inside the bex-api process (`cmd/api`) and Deployment, opt-in via `BEX_CP_DB_URI`.

> **Board note (2026-07-08):** this milestone had been moved to `done/` prematurely — its own DoD (live acceptance, t007) is unmet and prod enablement is still off — so it was moved back to `w1/m2/`; t001–t006 are done (moved to `m2/done/`), t007 stays open.

> **Implemented (w5; committed in `aebbd43`).** Built into `lego/backend`: `internal/store` (pgx store, embedded migrations, projector, cluster-internal tenant API on :8091), the single-writer wiring in `internal/api/core.go`, the OpenFGA `workspace:tea-<id>` write path in `internal/api/authz.go`, and the deploy wiring in `config/api` (env/port/RBAC). `go build`/`test`, `docker build`, and `kustomize build` all pass. **Not yet:** enable in prod (uncomment `BEX_CP_DB_URI` — the projector no-ops on an empty DB, so it is safe to flip on) and the live end-to-end acceptance (t007). Ship-gated; not moved to `done/`.

## Tasks (in order)

| id | title | est | status |
| --- | --- | --- | --- |
| t001 | Provision Postgres (CNPG `bex-db`) | — | **DONE** |
| t002 | Add the pgx store + `BEX_CP_DB_URI` to `lego/backend` — no new binary; extend the bex-api Deployment (`lego/operator/config/api/`) | 30m | **DONE** |
| t003 | Core schema + migrations (`internal/store/migrations`): tenants, apps, domains. Names stay k8s-native; Render/OpenFGA `workspace`/`service` bridged by `tea-`/`srv-` ids | 30m | **DONE** |
| t004 | Tenancy **mapping** keys only: `tenant_members`, `tenant_oauth_clients`, `tenants.metronome_customer_id`, cached `tenants.plan`. Auth & billing bought, not stored | 20m | **DONE** (schema; `tenant_members` write path is a later onboarding task) |
| t005 | Projector (`internal/store/reconciler.go`): `apps` rows → `App` CRs, status write-back. **Single writer of intent** — suspend/resume write the row (`Core.Store.SetAppSuspended`), never a bare CR patch the projector reverts | 30m | **DONE** (unit + regression tests) |
| t006 | Minimal API: create tenant/app/domain (cluster-internal :8091). Minting a tenant writes its OpenFGA `workspace:tea-<id>` membership, replacing `workspace:default` | 30m | **DONE** (OpenFGA write path added) |
| t007 | Fold DB conn + projector into the bex-api Deployment (one service). Deploy wiring (env/port/RBAC) built, **opt-in** via `BEX_CP_DB_URI`; live acceptance on first-tenant onboard | 30m | **DONE** |

## Definition of done

Create a tenant + app via the API → an `App` CR appears (labeled `app.kubernetes.io/managed-by`, scoping the projection) → the operator deploys it → `status.url` flows back onto the row. A minted API key scoped to that tenant's `workspace:tea-<id>` authorizes its resources (not just `workspace:default`). Business logic lives in `lego/backend`, not in the operator or in Postgres procedures; auth/billing stay bought (Kratos/Hydra/OpenFGA, Metronome/OpenCost) with only mapping keys stored here.

## Notes / decisions (w5 refactor)

- **Home:** `lego/backend/internal/store` + the projector; shares the bex-api Deployment. Not a separate binary or Argo Application.
- **Data-model names stay k8s-native** (`tenant`/`app`/`domain`); Render/OpenFGA `workspace`/`service` vocabulary is bridged at the edge by the shared `tea-`/`srv-` ids — matching how the team already wired OpenFGA (it checks `workspace:tea-…`/`service:srv-…` by id, not by struct name).
- **Single writer of intent:** the row owns projection-owned spec fields (`suspended`, replicas, tier, …); lifecycle verbs write through the store, the projector reconciles. Prevents the suspend/resume ↔ resync race.
- **OpenFGA hook:** `deploy/gitops/authz/model.fga` already says _"until the control plane grows real workspaces (w1/m2), every check targets `workspace:default`"_ — t006 is exactly where that placeholder gets replaced.

## Source

Converted from `.tmp/005-control-plane-service.md`; data flow / schema / one-Postgres / tiers in [docs/control-plane.md](../../../docs/control-plane.md) (planned); the bought-auth boundary and mapping-keys rule in [docs/auth.md](../../../docs/auth.md); the authz model in `deploy/gitops/authz/model.fga`.
