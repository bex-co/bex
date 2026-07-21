# m49 diagnosis and local regression evidence — 2026-07-20

Status: local implementation and deterministic verification complete for the diagnosed paths; production deployment, original-fixture recovery, and final CLI acceptance remain pending.

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

## Remaining production gate

Do not strip the original finalizer manually. After an authorized ship/deploy, the new controller must restore its per-App auth, prove build and Zot residue absent, revoke the credential, and remove the finalizer. Closeout additionally requires raw service GET 404, absence from `render services`, a fresh disposable image CRUD pass, an immediate first-build deletion pass within five minutes, and zero UID-scoped artifact inventory.
