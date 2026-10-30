# m144 capacity evidence

Captured 2026-10-29 local / 2026-10-30 UTC on Linux/arm64 (OrbStack kernel 7.2.9). See [design](../design.md) for chosen mechanism, exact source/archives, result boundaries and remaining acceptance. These records contain disposable local test data only; no credentials or tenant files.

- `patched-kernel-test.txt`: actual patched CSI volume implementation, real LUKS/full/grow/remount/local snapshot restore and mutation control.
- `upstream-linux-arm64-tests.txt`: Linux arm64 targeted upstream package regressions.
- `bounded-node-prototype*`: earlier standalone node primitive and test harness; superseded by the actual patched-code kernel test.
- `rejected-project-quota*`: negative proof explaining why a quota-only solution was not selected.
- `proof-image.Dockerfile.txt`: local test dependency recipe; the maintained derivative Dockerfile and patch live under `deploy/hcloud-csi/`.

- `pvc-admission-regressions.txt`: rejected quota/class growth and subsequent remount intent stay at the admitted size; accepted growth and refused creation are covered. The patched node also retries a briefly stale post-admission Secret with `Unavailable`.

- `operator-test.txt`: full `make test` passed after the admission-order fix, including the controller/envtest suite (205.907s).

- `scoped-installer-tests.txt` / `scoped-rollout-validator-tests.txt`: credential-free installer/pin and every-pod readiness regression results for the prepared scoped rollout path.
- `scoped-rollout-workflow-validation.txt`: production environment/pool/ref/host-trust and credential cleanup guard validation.
- `actual-chart-rendered-image-pins.txt`: both images extracted from actual Helm rendering of the checksum-authenticated hcloud-csi 2.22.0 archive using `official.values.json`; both match its exact image digest with empty tags. This validates chart expansion, not derivative publication or deployment.
- [Published image review](published-image-902630131/README.md): successful factory, exact-source workflow signature, signed index-subject SPDX, anonymous pull/execution of both Linux architectures, and actual chart rendering of the reviewed derivative digest. This prepares the committed pin; production rollout and lifecycle acceptance remain open.

- `inventory-unit-tests.txt` / `inventory-protocol-tests.txt` / `inventory-workflow-validation.txt`: the prepared read-only App/PVC/PV/platform-adoption inventory, exact-lifetime joins, safe operation dispatch and credential scope checks. No real production inventory has yet run.
- `inventory-jsonpath-proof.txt`: the real Kubernetes JSONPath engine processed all five synthetic projections; a separate join check preserved binding/image facts while excluding private canary fields. These are protocol proofs, not production facts.

- [Render disk contract review](render-disk-contract-review-20261029.md): primary documentation review supplied by root; it preserves the separate migration, actual capacity, billing, metrics and cross-surface production acceptance requirements.
