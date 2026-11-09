# w2 · m293 — dev-N has a working platform registry, so repo builds, static publishes and deletes work locally

**Worker:** worker2 **Goal:** the local CAPD mock runs the platform Zot registry that the operator's `BEX_REGISTRY` names (`zot.bex-registry.svc:5000`), with its htpasswd and config Secrets. Per-App registry credentials can then be issued, repo-backed services build and deploy on dev-N, and deleting any App finishes on its own. **Status:** todo — t001, t002 done; t007 (image architecture on the arm64 mock) then t003

## Tasks (in order)

| id   | title                                                                                   | est  | depends_on |
| ---- | --------------------------------------------------------------------------------------- | ---- | ---------- |
| t001 | `mock-cluster.sh` installs Zot in `bex-registry` (production chart/pin, htpasswd + config Secrets, bex-builder/bex-puller), idempotent — **DONE** | 1.5h | — |
| t002 | `mock-cluster.sh operator` wires the build/push and pull credentials the operator expects (`RegistryPushSecret`, the pull secret) — **DONE** | 45m  | t001 |
| t007 | The operator accepts the cluster's node architecture for built images (arm64 mock); production stays amd64 | 1h | t002 |
| t003 | dev-2 walk: a repo-backed web service builds and goes Live; a static site deploys; deleting each finishes with no manual finalizer edits | 45m  | t007 |
| t004 | Simplify                                                                                | 15m  | t003 |
| t005 | Test coverage                                                                           | 30m  | t004 |
| t006 | Closeout                                                                                | 10m  | t005 |

## Definition of done

- On a fresh mock, after `mock-cluster.sh`, `mock-cluster.sh operator` and `dev-env.sh 2 up`:
  - a web service built from `https://github.com/bex-co/bex` `examples/hello-go` goes Live and serves 200;
  - the operator logs no `PerAppRegistryCredsFailed`;
  - deleting the service removes its App CR within a few minutes, with no manual finalizer edits.
- A static site from `examples/static-site` is created and deleted the same way, which closes w2/076's acceptance. Publishing may still be unavailable locally, since there is no object store; if so, that limit is recorded.
- Contract tests cover the new stages.

## Source + Goal linkage

- **Source:** the w2/076 dev-2 walk (2026-11-08).
  - The static-purge fix let never-published static sites finalize, but deletion still hung on `delete registry images: registry auth: restore per-app credential: htpasswd: secrets "zot-htpasswd" is forbidden … in the namespace "bex-registry"`.
  - The mock has no `bex-registry` namespace and no Zot, yet the operator runs with `BEX_REGISTRY=zot.bex-registry.svc:5000`.
  - Every repo-backed App on dev-N therefore fails at create (`PerAppRegistryCredsFailed`) and hangs on delete. The test Apps `tea-dbohvtpjg4r2a20i48f0-w2-076-site` and `-site2` are left Terminating on dev-2 as evidence.
- **Goal linkage:** dev-N is every workstream's verification environment (AGENTS.md). Without a registry it can only run prebuilt images.
- **Expected outcome:** build-from-repo flows (the main Render onboarding path) can be walked locally, and deletes never strand.
- **Why now:** it blocks w2/076's acceptance and any dev-N walk that builds from a repo.
- **Choice:** install Zot, matching production, rather than running the mock operator without `BEX_REGISTRY`. Unsetting it would hide the per-App credential path that production uses, and loosening the fail-closed registry purge is a security change, not a dev convenience.
- **Render parity omitted:** local tooling only.

## Work log — 2026-11-08 (`/loopx w2`)

- **t001–t002, done:**
  - **Zot install:** `mock-cluster.sh` installs Zot 0.1.121 from `deploy/helm-artifacts.lock` (sha-checked) with production's exact values. `zot-config` is seeded only when absent, from the operator's `baseZotConfig`.
  - **Dev credentials:** minted with `openssl rand`, never printed, and kept while they still verify. They are applied through `scripts/registry-secrets.sh` with `BEX_REGISTRY_SECRETS_NO_ENV_FILE=1` so `.env` is never read.
  - **Verification:** every node gets 401 anonymously and 200 for bex-builder via the hosts.toml NodePort path.
  - **`operator`:** now applies `operator-registry-rbac.yaml`, whose absence caused `zot-htpasswd is forbidden`, plus the push/pull Secrets.
  - **No bex-puller:** production retired it.
  - **Tests:** 43 new contract checks.
- **dev-2 walk:**
  - The w2/076 leftovers finalized on their own once the RBAC landed.
  - **Repo-built service** (`examples/hello-go`): per-App credentials were issued, with no `PerAppRegistryCredsFailed`. It cloned, built and pushed to Zot, and the node pulled with the per-App credential (refused anonymously). It did **not go Live**: the image is arm64 and the operator requires linux/amd64. That is filed as **t007**.
  - **Delete:** the service's CR was gone in 5 s; its htpasswd user, ACL and pull Secret were removed.
  - **Static site:** publish failed with `StaticCredentialUnavailable` as expected (no local object store). Delete finished in about 6 min with no manual edits; the delay came from one registry-probe timeout plus backoff.
