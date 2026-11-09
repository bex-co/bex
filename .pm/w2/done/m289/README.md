# w2 · m289 — A fresh local cluster runs dev-N end to end

**Worker:** worker2 **Goal:** starting from no cluster, `mock-cluster.sh`, then `mock-cluster.sh operator`, then `dev-env.sh N up` gives a working dev-N with no manual steps: web services go Live, and nothing depends on the shell's kube context or the macOS keychain. **Status:** done (2026-11-08)

## Tasks (in order)

| id   | title                                                                                     | est | depends_on |
| ---- | ----------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | `mock-cluster.sh` installs Traefik's CRDs (or the local overlay's Traefik) so IngressRoutes and web services apply — **DONE** | 45m | — |
| t002 | `mock-cluster.sh operator`: build `bex-operator:dev` from the tree, import on every node, `make deploy`, `IfNotPresent`, wait — **DONE** | 1h | t001 |
| t003 | The scripts pass `--context kind-bex-mgmt` / a private kubeconfig and never read the global current-context — **DONE** | 30m | — |
| t004 | Helm runs with isolated registry credentials so a locked keychain cannot hang it; contract test — **DONE** | 30m | — |
| t005 | Simplify — **DONE**                                                                                  | 15m | t002, t003, t004 |
| t006 | Test coverage — **DONE**                                                                             | 30m | t005 |
| t007 | Closeout — **DONE**                                                                                  | 10m | t006 |

## Definition of done

- After `kind delete cluster --name bex-mgmt` (or with no cluster at all), these three commands run unattended: `bash scripts/mock-cluster.sh`, `bash scripts/mock-cluster.sh operator`, `bash scripts/dev-env.sh 2 up`.
- On dev-2, a web service created through the API goes Live and serves 200.
- With `~/.kube/config` holding no current-context and the macOS keychain locked, the same sequence still succeeds.
- `scripts/mock-cluster.test.sh` and `scripts/dev-env.test.sh` cover each new stage.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w2`, 2026-11-08, item 2, from the w2 run of 2026-11-05 to 2026-11-08:
  - After a reprovision the operator had to be rebuilt, imported on each node and deployed by hand, with `imagePullPolicy` patched.
  - With no Traefik CRDs, `make deploy` failed its IngressRoutes and every dev-2 web service failed with `no matches for kind "Middleware"`.
  - helm hung in `docker-credential-osxkeychain get` and needed a throwaway HOME.
  - The global current-context was cleared by another session, which broke `clusterctl get kubeconfig`.
- **Goal linkage:** every workstream's dev-N verification (AGENTS.md "Local dev environments are pre-approved") depends on this path.
- **Expected outcome:** a rebuild takes minutes, not hours, and dev-N walks can use web services again.
- **Why now:** each cluster rebuild currently costs hours of manual repair, and the cluster is shared by every dev-N.
- **Render parity omitted:** local developer tooling only, no customer surface.

## Closed — 2026-11-08 (`/loopx w2`)

- **Shipped in `e22f9cad3`:**
  - Traefik 41.0.0 and its CRDs, plus CNPG 0.29.1, from `deploy/helm-artifacts.lock` (sha-checked).
  - `mock-cluster.sh operator`: builds, imports on every node, applies RBAC, runs `make deploy`, sets `IfNotPresent`, and waits.
  - A private `infra/local/bex-mgmt.kubeconfig` with an explicit context; `~/.kube/config` is never used.
  - `scripts/lib/helm-isolated.sh`. Root cause: with no auths configured, helm v4 auto-detects `docker-credential-osxkeychain` and calls it, so pointing `DOCKER_CONFIG` or `--registry-config` at an empty file did not help.
  - Fixes for the lock-on-TERM trap and node readiness, and the dev-env CNPG wait order.
- **Fresh-cluster fixes from the single-pass checks:**
  - `cluster.yaml` and the CNPG Cluster creates retry, bounded, while a just-installed operator's admission webhook is not serving yet.
  - The cluster-autoscaler helm wait is raised from 3 to 10 minutes, and the Loki wait from 600 to 900 seconds. First image pulls on a new cluster took 4–10 minutes.
- **Single-pass evidence:**
  - After `kind delete` of both clusters, `mock-cluster.sh` → `mock-cluster.sh operator` → `dev-env.sh 2 up` exited 0, 0, 0 unattended, and the CNPG webhook retry fired once.
  - `dev-env.sh 2 status`: ALL GREEN.
  - The m289 agent's from-scratch run proved a dev-2 web service (`traefik/whoami`) goes Live and serves 200 through Traefik. It ran with current-context unset and a hanging keychain helper first on PATH; the helper was called 0 times.
- **Not covered:** `docker build` in the operator step still uses Docker's own `credsStore: osxkeychain`, so a locked keychain is proven harmless for helm only.
- **Environment:** the shared local cluster was rebuilt four times. dev-3's data on the previous cluster was lost when the m289 agent rebuilt it; the cluster was already dead at the time (apiserver cert lost after a Docker restart).
