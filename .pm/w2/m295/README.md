# w2 · m295 — The local cluster survives a Docker restart; docker build and CNPG pins match production

**Worker:** worker2 **Goal:** restarting Docker Desktop never leaves the shared local cluster silently dead. `mock-cluster.sh` recovers it, or names the one command that does. The operator build cannot hang on a locked keychain, and dev-env installs the same CNPG version as production. **Status:** todo

## Tasks (in order)

| id   | title                                                                                     | est | depends_on |
| ---- | ----------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | `mock-cluster.sh` detects a cluster broken by a Docker restart and recovers it or names the reprovision command | 1h | — |
| t002 | The operator step's `docker build` runs with an isolated credential config that keeps the docker context | 30m | — |
| t003 | dev-env's CNPG fallback installs 0.29.1 from `deploy/helm-artifacts.lock`, as production does | 20m | — |
| t004 | Simplify                                                                                  | 15m | t001, t002, t003 |
| t005 | Test coverage                                                                             | 30m | t004 |
| t006 | Closeout                                                                                  | 10m | t005 |

## Definition of done

- After a Docker Desktop restart, `bash scripts/mock-cluster.sh` either brings the cluster back (nodes Ready, apiserver serving) or exits with a precise diagnosis and the single command to reprovision. It never fails with an unexplained timeout.
- `mock-cluster.sh operator` completes with a locked keychain.
- `dev-env.sh up` on a cluster without CNPG installs the lock-pinned 0.29.1.
- Contract tests cover each case.

## Source + Goal linkage

- **Source:** `/pm-brainstorm more for w2`, 2026-11-09, item 2.
  - **2026-11-04:** after a Docker restart, the CAPD node took the mgmt cluster's fixed IP and the mgmt control plane could not start.
  - **2026-11-08:** the CAPD control plane lost `/etc/kubernetes/pki/apiserver.crt` on restart, because kind's entrypoint regenerates it from `/kind/kubeadm.conf`, which CAPD nodes lack. The apiserver crashlooped and the cluster had to be rebuilt. dev-3's data was lost.
  - **w2/m289:** recorded that `docker build` still uses `credsStore: osxkeychain`, and that dev-env's CNPG fallback (0.26.0) differs from production's 0.29.1.
- **Goal linkage:** dev-N reliability for every workstream (AGENTS.md "Local dev environments are pre-approved").
- **Expected outcome:** a Docker restart costs minutes, not a rebuild that destroys other workstreams' data.
- **Why now:** it happened twice in a week on the shared cluster.
- **Render parity omitted:** local tooling only.
