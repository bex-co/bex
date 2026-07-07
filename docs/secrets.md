# ADR: platform secrets — OpenBao for tenant credentials

**Status:** accepted; substrate deployed on the local mock cluster (w4/m5). bex-api does not yet read/write through it — that's product wiring in w4/m6.

## Context

[docs/auth.md](auth.md) covers _identity_ (who is calling) and _authorization_ (what they may touch). Neither answers a third question: once an agent deploys an app on a tenant's behalf (docs/vision.md pillar 4, "deploy-from-chat"), where does that app's own credentials — a database password, a third-party API key — actually live? Today the only place is the `App` CR itself: plaintext in etcd, readable by anyone who can `get` the CR. That's fine for bex's own platform secrets (Kratos/Hydra secrets, the zot registry credential, etc. — those are w1/m7 t003's concern: **platform** secrets committed to git via SOPS/sealed-secrets, because they're static and bex-operated). It is not fine for **tenant** secrets: a growing, per-tenant, per-app set of credentials that must be versioned (rotate without downtime), revocable, and readable only by the scope that owns them — not by every other tenant's app, not by anyone who can `kubectl get apps`.

This ADR is the substrate for that: a versioned, policy-scoped secret store running as platform infrastructure, so w1/m2's `tenants` table and w4/m6's product wiring have somewhere to put credentials before either exists.

## Decision

### 1. OpenBao, not HashiCorp Vault

Same governance test CNPG, Ory, and OpenFGA already passed ([postgresql-management.md](postgresql-management.md) §1): open-source, self-hostable, no SaaS dependency. HashiCorp re-licensed Vault under BSL 1.1 in 2023 — source-available, not OSI open source, and explicitly restricts competing-use. That disqualifies it for a platform whose whole premise is "Apache-2.0, run it yourself" ([vision.md](vision.md)). **OpenBao** is the Linux Foundation-governed MPL-2.0 fork: same HTTP API, same `secrets`/`auth`/`policy` model, drop-in for everything below.

### 2. Storage backend: integrated Raft, not Postgres or Consul

OpenBao (like Vault before it) supports several storage backends. Bex already operates Postgres well via CNPG — the instinct is to reuse it, as Kratos/Hydra/OpenFGA all do. Rejected anyway:

- **Postgres backend** is community-supported, not the recommended path, and HashiCorp deprecated recommending external-DB backends in favor of integrated storage back in Vault 1.4 (2020) for good reason: OpenBao would need its own DSN Secret bootstrapped out-of-band exactly like Kratos/Hydra — except the thing being bootstrapped is itself the credential store, a chicken-and-egg dependency this design doesn't need to take on.
- **Consul** is another moving part bex doesn't otherwise run, for the same benefit Raft gives for free.
- **Integrated Raft storage** is self-contained (one PVC per pod, no external dependency), gives built-in snapshotting, and is the chart's own recommended default. Locally: `replicas: 1` (single-node raft — same storage engine, no quorum). **Prod: bump to `replicas: 3`** for quorum (tolerates one node down) — not built this milestone, tracked the same way `kratos-db`/`hydra-db` at `instances: 1` are (see Consequences).

### 3. Unseal strategy: Shamir secret sharing via `.env`, not auto-unseal

OpenBao encrypts everything at rest behind a master key split into Shamir shares; a freshly started (or restarted) node is **sealed** until a threshold of those shares is presented. The alternative — cloud KMS auto-unseal (AWS/GCP) — is rejected for the same reason bex's `infra/` targets Hetzner via Cluster API rather than a single hyperscaler: it would tie a self-hostable platform to a specific cloud's KMS.

Instead: [scripts/bao-init.sh](../scripts/bao-init.sh) runs `bao operator init` once (5 shares / 3 threshold, the OpenBao default), capturing 3 unseal keys + the root token and writing them into `.env` (gitignored — same rule as `bex.kubeconfig`) — **never printed** to stdout or logs, mirroring `auth-secrets.sh`'s never-echo convention. Every subsequent run detects "already initialized," reads the same three keys back out of `.env`, and unseals idempotently — including after a pod restart (raft persists the encrypted data to the PVC, but the _decryption_ state is always sealed on process start; this is correct behavior, not a bug, and `scripts/bao-verify.sh` asserts it explicitly).

This is a real trust tradeoff: whoever holds `.env` (or its prod equivalent — GitHub Actions secrets via `scripts/gh-secrets.sh`) can unseal OpenBao and mint root-scoped tokens. Accepted — it's the same trust boundary the Ory secrets and `bex.kubeconfig` already carry.

### 4. KV v2 mount `tenants/`

One versioned key-value engine (`kv-v2`: soft-delete + rollback), mounted at `tenants/`. Tenant credentials live at `tenants/<tenant-id>/...` — a **path convention**, not a mount-per-tenant. A mount per tenant doesn't scale (mounts are a fairly heavyweight, admin-managed concept) and buys nothing here: the policy in §5 already scopes every non-admin caller to the `tenants/*` subtree as a whole, and per-tenant isolation _within_ that subtree is w4/m6's job (the product layer knows which tenant a caller represents; OpenBao itself doesn't need to).

### 5. Kubernetes auth method — bex-api authenticates as itself

No shared token. OpenBao's chart enables `server.authDelegator` by default, granting OpenBao's own ServiceAccount the `system:auth-delegator` ClusterRole — the permission its Kubernetes auth method needs to call the apiserver's `TokenReview` API and validate a _caller's_ projected ServiceAccount token. [scripts/bao-k8s-auth.sh](../scripts/bao-k8s-auth.sh) (idempotent, same shape as `authz-model.sh`) then:

- enables the `kubernetes` auth method,
- writes a policy (`tenants-rw`) granting `create`/`read`/`update`/`delete`/`list` on `tenants/*` and **nothing else** — no `sys/*`, no other mount, so a compromised bex-api pod can't read seal status, re-key, or touch any other tenant of the store (there is only one, but the point generalizes),
- binds a role `bex-api` to that policy, scoped to ServiceAccount `bex-api` in namespace `bex-system` (the same ServiceAccount [operator/config/api/rbac.yaml](../operator/config/api/rbac.yaml) already defines).

This is the same shape as Hydra `client_credentials` or the OpenFGA preshared key: a machine identity, minimal scope, no human password anywhere in the loop.

### 6. GitOps shape, namespace `secrets`

[deploy/gitops/base/openbao.yaml](../deploy/gitops/base/openbao.yaml) is a multi-source Argo Application (pinned chart `0.28.4`, app v2.5.5) exactly like `kratos.yaml`/`openfga.yaml`: the Helm repo provides the chart, this repo's `ref: values` source provides [base/values/openbao.values.yaml](../deploy/gitops/base/values/openbao.values.yaml). Namespace **`secrets`**, not `auth` — this is tenant-credential infrastructure, a different trust boundary than platform identity/authz, and keeping it a separate namespace keeps RBAC/NetworkPolicy scoping simple later. `sync-wave: "1"`: unlike Kratos/Hydra/OpenFGA, OpenBao has no CNPG Cluster dependency (§2), so it doesn't need to wait for the CNPG operator wave — it's an independent base component alongside Traefik/cert-manager/metrics-server. Cluster-internal only, no ingress: bex-api reaches it via in-cluster Service DNS (`openbao.secrets.svc:8200`) only, the same shape as Hydra's/OpenFGA's admin APIs.

## Verification

`scripts/bao-verify.sh` (exit 0 = pass) proves, against the current kubeconfig cluster:

1. **Init/unseal is idempotent** — running `bao-init.sh` twice in a row changes nothing the second time.
2. **KV read/write** — a value written under `tenants/verify/...` reads back with the expected data.
3. **Scoped SA login** — the `bex-api` ServiceAccount logs in via the Kubernetes auth method and can read/write under `tenants/*`, but a request for `sys/mounts` (or any path outside `tenants/*`) with that same token is denied (403). (`sys/seal-status` itself is deliberately unauthenticated in OpenBao — not a useful negative test.)
4. **Restart durability** — after deleting the OpenBao pod (the chart's StatefulSet uses `updateStrategyType: OnDelete`, so a plain `kubectl rollout restart` doesn't actually recreate it — the pod has to be deleted directly), the pod comes back **sealed** (expected — no auto-unseal), the KV value from step 2 is unreadable until unsealed, and re-running `bao-init.sh`'s unseal step (same three keys from `.env`) restores it exactly. This proves both that state survives the pod (raft on the PVC) and that the unseal keys are load-bearing, not decorative.

Local bring-up order (what Argo's waves do in prod): mock cluster → local-path provisioner → `secrets` namespace labeled `pod-security.kubernetes.io/enforce=privileged` (local-CAPD quirk below) → OpenBao chart (base values + local overlay) → `scripts/bao-init.sh` → `scripts/bao-k8s-auth.sh` → `scripts/bao-verify.sh`.

Local-CAPD quirks (mock cluster only, don't apply to prod):

- The `secrets` namespace needs the same `pod-security.kubernetes.io/enforce=privileged` label the `auth` namespace already carries — local-path's hostPath helper pods run in the **PVC's** namespace and are blocked by the cluster's `baseline` PSA enforcement otherwise (same reasoning as [auth.md](auth.md)'s local-CAPD quirks section).
- Unlike Kratos/Hydra/OpenFGA, OpenBao's pod **does** need pinning to the control-plane node (`overlays/local/values/openbao.values.yaml`'s `nodeSelector`/`tolerations`), and this one is easy to misdiagnose: a worker-scheduled pod comes up `Running`, its port stays in `LISTEN`, and it even accepts TCP connections — but every HTTP request just hangs forever (confirmed via `kubectl exec` to `127.0.0.1:8200` from inside the pod itself, which hung identically to a port-forward from outside). The cause is `service_registration "kubernetes" {}`, which has OpenBao call the apiserver directly from inside its own request-handling path; on a worker node under this cluster's OrbStack/Calico networking that call never returns (the same reachability gap [auth.md](auth.md)'s local-CAPD quirks section pins coredns/local-path/CNPG for). Moving the pod to the control-plane node fixed it immediately, confirming the cause.

## Prod deploy path

Not wired into `deploy.yml` this milestone (the DoD is local-mock-cluster verification only) — this is the intended path for w4/m6 or a follow-up task to implement:

1. First deploy against a real cluster: run `scripts/bao-init.sh` once, by hand, against the prod kubeconfig. It generates the unseal keys + root token and writes them into the operator's local `.env`.
2. `scripts/gh-secrets.sh` pushes `BAO_UNSEAL_KEY_1`/`BAO_UNSEAL_KEY_2`/`BAO_UNSEAL_KEY_3`/`BAO_ROOT_TOKEN` into this repo's GitHub Actions secrets (add them to that script's key list alongside the `KRATOS_*`/`HYDRA_*`/`OPENFGA_*` keys).
3. `deploy.yml` runs `bao-init.sh` (idempotent — detects "already initialized," just unseals) and `bao-k8s-auth.sh` after the OpenBao rollout on every subsequent deploy, the same shape as the existing `auth-secrets.sh`/`authz-model.sh` steps.
4. Production sizing: patch `server.ha.replicas: 3` and drop the local overlay's `storageClass: local-path` override (falls back to the cluster's default, e.g. `hcloud-volumes`) in a `overlays/prod` values layer, mirroring how `auth-dbs`'s local patch shrinks storage only for CAPD.

## Alternatives considered

- **HashiCorp Vault** — same product, but BSL-licensed (§1). Rejected.
- **Encrypted columns in the control-plane Postgres** (`pgcrypto`) — reinvents dynamic secrets, leasing, versioning, and audit logging that the Vault family already solved, and couples tenant secret material to the same database backing `App` CR metadata (one compromise or bad migration touches both). Rejected.
- **Cloud secret managers** (AWS Secrets Manager, GCP Secret Manager) — defeats self-hostability and ties bex to one cloud, even though `infra/` already targets Hetzner via Cluster API. Rejected.
- **A mount per tenant** — see §4. Rejected as unnecessary operational overhead.

## Consequences

- Once w4/m6 wires product usage, bex-api becomes hard-dependent on OpenBao for any credential-touching verb — an outage or sealed state should 503 that verb, mirroring the Hydra fail-closed precedent in [auth.md](auth.md).
- Single-node raft locally, `replicas: 1` — no quorum, no automated snapshot backup yet. **OpenBao must join the w1/m7 backup/HA work** before real tenant credentials live in it; losing the single node loses everything (same caveat `kratos-db`/`hydra-db` carry today).
- Root token + unseal keys are a manual, high-trust bootstrap step; rotating the root token or re-keying the Shamir shares is a manual runbook not yet built.
- Prod DNS/CI wiring (above) is deliberately deferred — the store exists and is verified locally, but nothing outside this milestone's scripts talks to it yet.
