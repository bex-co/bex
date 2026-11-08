# w2 · m289 — A fresh local cluster runs dev-N end to end

**Worker:** worker2 **Goal:** starting from no cluster, `mock-cluster.sh`, then `mock-cluster.sh operator`, then `dev-env.sh N up` gives a working dev-N with no manual steps: web services go Live, and nothing depends on the shell's kube context or the macOS keychain. **Status:** todo

## Tasks (in order)

| id   | title                                                                                     | est | depends_on |
| ---- | ----------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | `mock-cluster.sh` installs Traefik's CRDs (or the local overlay's Traefik) so IngressRoutes and web services apply | 45m | — |
| t002 | `mock-cluster.sh operator`: build `bex-operator:dev` from the tree, import on every node, `make deploy`, `IfNotPresent`, wait | 1h | t001 |
| t003 | The scripts pass `--context kind-bex-mgmt` / a private kubeconfig and never read the global current-context | 30m | — |
| t004 | Helm runs with isolated registry credentials so a locked keychain cannot hang it; contract test | 30m | — |
| t005 | Simplify                                                                                  | 15m | t002, t003, t004 |
| t006 | Test coverage                                                                             | 30m | t005 |
| t007 | Closeout                                                                                  | 10m | t006 |

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
