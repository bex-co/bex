# m49 diagnosis and local regression evidence — 2026-07-20

Status: complete. Deterministic regressions, production rollout, original-fixture recovery, unmodified-CLI acceptance, and cluster/Zot residue audits all passed.

## Official CLI wire contract

- Client: unmodified `render-oss/cli` v2.21.0 from the ignored local `cli/` checkout.
- Generated model: `Image.ImagePath` and `Image.OwnerId` are both non-omitempty strings.
- Create builder sets top-level owner and image path, leaving nested owner at its zero value. Sanitized fragment: `{"ownerId":"<workspace>","image":{"imagePath":"<ref>","ownerId":""}}`.
- Update builder sets the replacement image path, again leaving nested owner at its zero value. Sanitized fragment: `{"image":{"imagePath":"<replacement-ref>","ownerId":""}}`.
- Production result before the fix: Render's pinned OpenAPI accepted the request; the real strict handler returned a named unknown-field 400 for nested `ownerId` because its REST wire struct omitted the field.
- Local regression: the exact fragments pass authentication, pinned OpenAPI validation, and the real services handler. Conflicting owners return a named `image.ownerId` 400 with no mutation; a non-member nested owner remains 403; a planted unknown image sibling remains 400.

No authorization header, API key, registry password, kubeconfig, or full authenticated production response is stored here.

## Exact production fixture

- Public service id: `srv-d9f9oalju7gs73fvngqg`
- Display name: `cli-oapi-r-220737-u`
- App CR name: `tea-d98210cbbpdc73dcrkvg-cli-oapi-r-220737`
- App UID: `8231c540-62e9-4b0b-a973-78dd5e66ab88`
- Workspace: `tea-d98210cbbpdc73dcrkvg`
- Repository: `https://github.com/render-examples/go-gin.git`
- Created: `2026-07-20T22:07:39Z`
- Deletion requested: `2026-07-20T22:07:42Z`
- Observed state: phase `Building`/tenant view `Deleting`, with `app.bex.co/finalizer` retained.

## Root-cause timeline

1. The build reconcile started at create and blocked inside `build.Build`. It logged `job ... did not finish within 20m0s` at `22:27:39Z`; controller-runtime could not dispatch a deletion reconcile for that App while the original reconcile was still polling.
2. Finalization then repeatedly failed exact-UID build inventory: the controller ServiceAccount could delete build Secrets and ServiceAccounts but could not list them, and it could list Pods but could not delete them in `bex-build`.
3. Zot cleanup attempted tag listing without auth and received 401. `BEX_REGISTRY_PUSH_SECRET` named `bex-registry-push`, but that Secret was absent from `bex-build`; production used per-App `reg-pull-*` credentials instead.
4. The old finalizer called credential revocation in the same pass even while execution/registry cleanup was pending or failed. It removed the App's htpasswd/ACL/pull credential before repository absence was proven, making later least-privilege retry impossible.

The deployed operator image was `ghcr.io/bex-co/bex-operator@sha256:3b8cfc…`. Secret values and kubeconfig contents were neither printed nor retained.

### Pre-deploy follow-up audit

A later read-only production audit confirmed that the same App still had deletion timestamp `2026-07-20T22:07:42Z`, phase `Building`, and `app.bex.co/finalizer`, with no `app.bex.co/registry-purge-complete` annotation. Exact-UID inventory in `bex-build` was zero for Jobs, Pods, Secrets, ServiceAccounts, NetworkPolicies, kpack Images, and kpack Builds. The App namespace's deterministic `reg-pull-<app>` Secret was absent. All bex workloads, including `bex-controller-manager`, still ran digest `sha256:3b8cfc62f6a035960a7be116e9cfd439ecd78e1189ec1940287fa10cdb3d41ba`.

This narrows the remaining stall: Kubernetes build residue is already absent, but the old controller has revoked the only per-App registry authority without persisting proof that the Zot repository is absent. Resource absence alone is therefore not sufficient evidence to strip the finalizer; the repaired controller must restore the scoped credential, prove the registry stage, revoke last, and converge normally.

## Local correction and gates

- BuildKit and kpack waits now query the owning App through the supplied uncached client every poll and return `ErrAppDeleting` while leaving the UID-labeled artifact for finalizer inventory.
- Finalization logs explicit execution/external/credential pending or error stages. It preserves registry credentials until execution and external cleanup are both done.
- Per-App cleanup auth is idempotently restored and activation-checked. Registry empty/404 proof is persisted on the deleting App before credential revocation, so manager restart cannot re-create/revoke in a loop.
- `bex-build-credentials` gains only namespaced `list` for Secrets/ServiceAccounts and `get,list,delete` for Pods; a source-level GitOps guard pins that exact scope.
- The verifier adds image create/update/delete and delete-during-first-build legs; every delete requires GET 404 and official-CLI list absence within 300 seconds. Cleanup uses the same bounded wait on success, failure, INT, and TERM.

Passing deterministic checks at this evidence revision:

- `go test ./internal/apps ./internal/api`
- `go test ./internal/build`
- `go test ./internal/controller`
- `bash scripts/cli-services-parity-verify.sh self-test`
- `bash scripts/gitops-validate.sh` (with documented local warnings for unavailable optional `fga`/`promtool` binaries)
- `go test ./...` in `lego/backend`
- `make test` in `lego/operator` (codegen, fmt, vet, unit tests, and envtest)
- `make lint` in `lego/operator` (operator and backend; zero issues)
- `go test -race ./internal/apps ./internal/api` in `lego/backend`
- `go test -race ./internal/build ./internal/controller` in `lego/operator`
- Bash syntax, Prettier checks, and `git diff --check`

The isolated dev-9 live verifier was not run: its local API and Hydra ports were offline, and its disposable CLI key/binary files were absent. This is recorded as unavailable rather than credited as acceptance.

## Production rollout and two follow-up corrections

The normal ship path produced four relevant fixes:

- `dba95845` — nested official-CLI image ownership plus durable first-build cleanup/RBAC/registry retry changes.
- `79ebf77e` — removed unused package-manager tooling from the dashboard runtime image after the deployment's CRITICAL CVE gate found the bundled npm `tar`; the rebuilt image passed the same gate and deployment run `29799796965` completed.
- `927218c4` — accepted the official CLI's declared create-time `serviceDetails.region` in the strict decoder while continuing to normalize placement to the one configured platform region. Deployment run `29801075357` completed.
- `857e49b9` — quiesced the App Ingress and cert-manager Certificate before deleting TLS Secrets, and corrected verifier classification of the official CLI's `{service: ...}` list wrapper. Deployment run `29802277256` completed.

The last fix came from a production-only interaction found during the first post-rollout acceptance. Image create and update passed, but deletion reached the verifier's 300-second bound with `executionPending=false externalPending=true`. The App-owned Ingress and ingress-shim Certificate were still present while the finalizer deleted their TLS Secret, so cert-manager recreated the Secret on later passes. Kubernetes cannot garbage-collect those owner-referenced producers while the App finalizer remains. Finalization now observes an ordered Ingress → Certificate → Secret shutdown; the deterministic deletion test retains the finalizer until each previous stage is absent.

The final production image was `ghcr.io/bex-co/bex-operator@sha256:03db349ef81942dad2904827185bf17912bd9658783d67c9b7b42ea878c96987`. `bex-controller-manager` was 1/1 Ready, `bex-api` was 2/2 Ready, and `https://api.bex.co/healthz` returned 200. Production's truthful configured region is `fsn1`; the official CLI may submit `frankfurt`, but bex never persists that caller hint as placement.

## Original fixture recovery

The original `srv-d9f9oalju7gs73fvngqg` / `cli-oapi-r-220737-u` fixture converged through the repaired finalizer without forced removal. Raw service GET returned 404, the raw list and unmodified CLI list omitted it, and its App CR was absent. Exact UID/name inventory was zero across App, Deployment, Service, Ingress, Pod, Secret, ServiceAccount, NetworkPolicy, CronJob, Job, PVC, kpack Image, and kpack Build resources in both the App and build namespaces. Its per-App pull Secrets, Zot htpasswd user, Zot repository ACL/user strings, and authenticated registry repository/tag read were also absent. No unrelated production resource was mutated.

## Final unmodified-CLI acceptance

The pinned, unmodified `render-oss/cli` v2.21.0 ran the complete baseline against `https://api.bex.co/v1/` with the explicit disposable-production gate. The final run used the sanitized prefix `cp-051218-12572` and passed all required legs:

- image create/read, image update/read, and delete to raw GET 404 plus official-CLI list absence;
- repo-backed native service creation followed immediately by delete during its first build, also converging inside the 300-second deadline;
- full web, native cron, and static-site create/update readback;
- explicit-region clone, honest bare-clone closed-enum rejection for `fsn1`, upstream runtime-update rejection, and bex preview-create/update rejection;
- EXIT cleanup of every remaining fixture to GET 404 plus official-CLI list absence.

The harness now supplies the official CLI's required native build/start commands on the immediate-delete fixture and recognizes both flat list items and the CLI's `{service: ...}` wrapper. One earlier run caught a transient immediate web read; the unchanged strict assertion passed on the complete repeat, and no retry was added that could mask wire drift.

A second, independent production audit found zero matching objects across 19 App/workload/build/TLS/credential resource classes. The Zot htpasswd, repository ACL, and all config strings had zero prefix matches; a builder-authenticated `/v2/_catalog` returned zero repositories with the acceptance prefix. Tokens, passwords, kubeconfig data, full authenticated response bodies, and secret contents were never printed or retained.

## Final gates

- Backend: `go test ./...`, focused OpenAPI/Apps composition tests, lint, and race tests passed.
- Operator: `make test`, `make lint`, `go test -race ./internal/build ./internal/controller`, focused deletion tests, and the CI operator workflow passed.
- Verifier: Bash syntax, `services-parity-verify self-test`, required-leg census, wrapper classification, timeout/leak/redaction controls, and the production baseline passed.
- Supply chain/deploy: secret scan, image signing/SBOM, operator and dashboard CRITICAL CVE gates, GitOps write-back, and production rollout passed in run `29802277256`.
- Formatting: Prettier 3.4.2 and `git diff --check` pass at closeout.
