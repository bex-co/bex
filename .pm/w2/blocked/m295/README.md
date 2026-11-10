# w2 · m295 — The local cluster survives a Docker restart; docker build and CNPG pins match production

**Worker:** worker2 **Goal:** restarting Docker Desktop never leaves the shared local cluster silently dead. `mock-cluster.sh` recovers it, or names the one command that does. The operator build cannot hang on a locked keychain, and dev-env installs the same CNPG version as production. **Status:** blocked — t001 implemented (live Docker-restart check pending the user), t002–t005 done

## Tasks (in order)

| id   | title                                                                                     | est | depends_on |
| ---- | ----------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | `mock-cluster.sh` detects a cluster broken by a Docker restart and recovers it or names the reprovision command | 1h | — |
| t002 | The operator step's `docker build` runs with an isolated credential config that keeps the docker context — **DONE** | 30m | — |
| t003 | dev-env's CNPG fallback installs 0.29.1 from `deploy/helm-artifacts.lock`, as production does — **DONE** | 20m | — |
| t004 | Simplify — **DONE**                                                                                  | 15m | t001, t002, t003 |
| t005 | Test coverage — **DONE**                                                                             | 30m | t004 |
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

## Work log — 2026-11-09 (`/loopx w2`)

- **t001 (implemented):** `mock-cluster.sh` now checks cluster health before anything else.
  - A healthy cluster is only read.
  - Wrong start order or IPs (the mgmt node, load balancer or control plane down, or on the wrong IP): it stops both clusters and starts them in creation order. The expected IPs come from files that survive a restart (`admin.conf`'s server line, the haproxy backend).
  - A stopped worker is started on its own.
  - A missing apiserver cert: it pauses cluster `bex`, rebuilds the kubeadm config from the bootstrap ClusterConfiguration with the node's current IP, runs `kubeadm init phase certs apiserver` against the existing CA, restarts the kubelet, waits, and unpauses.
  - Anything it cannot fix ends in a diagnosis plus `bash scripts/mock-cluster.sh reprovision`, a new subcommand.
  - **Prevention:** every bring-up seeds `/kind/kubeadm.conf` so kind regenerates the cert itself.
  - **Live evidence:** a generated cert has the same SANs as the live one; seeding was confirmed; healthy runs triggered no recovery.
- **t002 (done, live):** `docker build` runs with a private `DOCKER_CONFIG` (no credsStore, current context and plugins kept). With a hanging keychain helper first on PATH, the build and `mock-cluster.sh operator` finished with 0 helper calls; without isolation the build hung.
- **t003 (done):** dev-env's CNPG fallback installs 0.29.1 from the lock, sha-checked. This is contract-tested only, because the live cluster already had CNPG.
- **Tests:** about 90 contract checks; deliberately breaking each behaviour made the tests fail. The shared pinning helpers moved to `scripts/lib/helm-pinned.sh`.

## Blocked (2026-11-09)

The DoD's live check, that a real Docker restart is recovered or diagnosed in one run, was not done. The auto-mode classifier denied the agent's `docker stop`/`docker start` reproduction on the shared cluster.

**User options:**

- (a) restart Docker Desktop yourself, then run `bash scripts/mock-cluster.sh` and paste the last lines; or
- (b) allow the agent to stop and start the bex node containers for this test; or
- (c) accept the stub-tested recovery as the acceptance.

Run `/unblock w2`.
