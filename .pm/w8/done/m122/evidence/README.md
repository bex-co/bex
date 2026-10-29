# w8/m122 evidence

## t001 — which digest the kubelet records (2026-10-20)

Local kind/CAPD cluster (kindest/node v1.36.1, containerd, arm64 worker `bex-worker-0-jcswp-9r6xl-grztd`). A throwaway pod in a temporary `dev-8` namespace (deleted afterwards, `namespaces "dev-8" not found`):

```
kubectl -n dev-8 run m122-digest-probe --image=docker.io/ealen/echo-server:0.9.2 --restart=Never --image-pull-policy=Always
.status.containerStatuses[0].image   = docker.io/ealen/echo-server:0.9.2
.status.containerStatuses[0].imageID = docker.io/ealen/echo-server@sha256:006b92e1d6682442e29d1d0d73c34a61166d42f8a5bf1ea10c318f07ad4790e2
```

`docker buildx imagetools inspect docker.io/ealen/echo-server:0.9.2`:

```
MediaType: application/vnd.oci.image.index.v1+json
Digest:    sha256:006b92e1d6682442e29d1d0d73c34a61166d42f8a5bf1ea10c318f07ad4790e2
  linux/amd64  sha256:bcfc131396341339d5f5040c999766e4138131b4cb1e17e17d56ca8bd7cb32cd
  linux/arm64  sha256:74afa5ffd1f0cd81bea9a3ef2f27341dc7e93ed221f2817a2119c230c25cc8a2
```

The kubelet reports the tag's **index** digest, not the platform manifest. The DoD's `skopeo` comparison therefore uses the index digest (`skopeo inspect --raw docker://… | sha256sum`, or the registry's `Docker-Content-Digest` for the tag). Documented in [ADR004 §Deploy image digest](../../../../docs/ADR004-app-deployment.md#deploy-image-digest-w8m122).

## t001–t006 verification (2026-10-20, pre-ship)

- Operator: `make test` (lego/operator) exit 0; new `TestImageDigestFromRef`, `TestRecordImageDigest`, `TestServingReleaseRecordsItsOwnImageDigest`, `TestWorkerReleaseRecordsImageDigest`.
- Backend: `scripts/backend-test-deps.sh up` + `BEX_TEST_REQUIRE_DEPS=1 go test -p 1 ./...` exit 0 (real Postgres: `image_sha` round-trips in `store_pg_test.go`); new `TestLiveDeploysRecordTheirOwnImageDigests`, `TestLiveCloseNeverTakesAnotherReleasesDigest`, `TestFailedCloseRecordsNoImageDigest`, `TestDeployImageShaOnEverySurface`.
- `make lint` (all four modules + deadcode) exit 0.
- Dashboard: `tsc -b` + `tsc -p tsconfig.test.json` clean; vitest deploys/routes/capabilities/services 1949/1950 pass. The one failure, `capabilities-startup.test.tsx` "sends nothing without a verified session", also fails on clean HEAD and on origin/main in this checkout: its `node_modules` is stale (`@apollo/client` 4.1.3 installed vs 4.3.3 locked). It passes in CI (`f4d9dfb20` green). Local ESLint does not load for the same reason.
