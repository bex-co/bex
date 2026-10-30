# Controller envtest resource-version race — 2026-10-30 UTC

Main deploy run 41030129236 failed test `Pool recycle / Delete recycle strategy` at the one-shot release-annotation update. The controller legitimately patched BatchSandbox status after the test GET, so Kubernetes rejected the stale test update with HTTP 409. Controller inputs were unchanged from the green dc80 run; 35/36 specs passed. A second remote attempt was externally cancelled; this run did not duplicate it.

The carried test-only patch refetches and reapplies that annotation inside the upstream's existing `RetryOnConflict` convention. Every pool-allocation and released-Pod deletion assertion remains. Only the test workflow applies the patch; the runtime controller image is unchanged.

The fresh pinned upstream accepts all three carried patches and remains formatted. Five independent focused real-envtest runs passed. A scratch-only overlay deliberately created a real resource-version conflict, required the stale update to return HTTP 409 and required at least two retry attempts, then passed the original lifecycle assertions. Full upstream `make test`, generation, vet and controller golangci-lint passed. Local verification used Darwin/arm64 Go 1.26.4 and Kubernetes 1.33.0; CI uses Linux/arm64 Go 1.25.7. Earlier local metrics-port/provisioning failures and their corrected isolated reruns remain explicit in the receipt.

No owned controller/envtest processes remain. Raw logs and overlay source are losslessly compressed, and all stored/decompressed identities are recorded. No remote fixture, runtime image or application behavior was changed by this test repair. The corrected remote workflow still supplies the final CI result.
