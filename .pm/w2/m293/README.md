# w2 · m293 — dev-N has a working platform registry, so repo builds, static publishes and deletes work locally

**Worker:** worker2 **Goal:** the local CAPD mock runs the platform Zot registry that the operator's `BEX_REGISTRY` names (`zot.bex-registry.svc:5000`), with its htpasswd and config Secrets. Per-App registry credentials can then be issued, repo-backed services build and deploy on dev-N, and deleting any App finishes on its own. **Status:** todo

## Tasks (in order)

| id   | title                                                                                   | est  | depends_on |
| ---- | --------------------------------------------------------------------------------------- | ---- | ---------- |
| t001 | `mock-cluster.sh` installs Zot in `bex-registry` (production chart/pin, htpasswd + config Secrets, bex-builder/bex-puller), idempotent | 1.5h | — |
| t002 | `mock-cluster.sh operator` wires the build/push and pull credentials the operator expects (`RegistryPushSecret`, the pull secret) | 45m  | t001 |
| t003 | dev-2 walk: a repo-backed web service builds and goes Live; a static site deploys; deleting each finishes with no manual finalizer edits | 45m  | t002 |
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
