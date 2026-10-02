# m49 acceptance — 2026-10-02

## Automated verification

- Full backend suite passed: `source /tmp/bex-w8-m48-test.env && GOWORK=off go test -p 1 ./...`, with dedicated PostgreSQL, OpenFGA, and OpenBao. Final log: `/tmp/bex-w8-m49-backend-clean-instance.log`. The private environment file is not published.
- Focused controller/pre-deploy regressions and final `GOWORK=off GOFLAGS=-p=1 make test` passed, including codegen, formatting, vet, and Kubernetes 1.35 envtest. No generated deltas. Final log: `/tmp/bex-w8-m49-operator-make-test-final.log`.
- Final `GOFLAGS=-p=1 make lint` passed across operator, backend, types, and CLI, including whole-program dead-code checks. Final log: `/tmp/bex-w8-m49-lint-final.log`.
- Twenty adapter refusals cover REST, GraphQL, MCP, and deploy hooks, including unchanged App/deploy/store state. Create/update/Blueprint checks cover actionable grammar and registry errors, digest lengths, valid short/tag/digest forms, the 512-byte boundary, and refusal before any Blueprint declaration is applied. Existing repository deploy and restart controls pass.
- Operator regressions cover current/stale/deleting/foreign pods, exact pre-deploy Job UID selection, immediate malformed-image failure, retained retry grace for other pull errors, failed Job deletion, first-release failure, and preservation of a serving release.
- Three simplify reviews completed. The lowercase-repository assertion was strengthened; lint follow-up centralized the app container name and separated first-release/serving-release tests without removing assertions. Final quality review found no further issues.

## Live acceptance

The [summary](evidence/summary.json) records **57/57 final checks**: 40 named refusals across REST/current Bex/unmodified Render/GraphQL/MCP, two invalid-create checks, eleven Blueprint grammar/policy controls, and four configured-digest rollout/content checks. Refused deploy inputs create no deploy row and preserve the saved image. Current Bex and unmodified Render share upstream pin `a764810a7682` (2.27.0); the unchanged CLI binaries were reused from m48. The rebuilt API SHA-256 is `b5b9c99faf94d0dc06392763b95e8323bcc4a7f11a148c73367bdca79b22b9a8`.

The conclusive pull fixture was configured directly with `docker.io/library/busybox@sha256:73aaf090f3d85aa34ee199857f03fa3a95c8ede2ffd4cc2cdb5b94e566b11662`. Its App, Deployment, pod, and runtime image references matched that digest. Both CLIs' same-digest `--image … --wait` controls exited zero and reached live, creating exactly one deploy each. A temporary forward to the owned Service returned HTTP 200 and `m49-digest-ok` after both. Public ingress/TLS was not asserted.

The shared local operator (`bex-operator:w1-108-pins`) was not updated. It has no supported fixture-scoped manager mode, so launching a second manager would race other workstreams. The new `InvalidImageName` behavior is verified by the current operator's full suite/envtest. Consumption of a **different** per-deploy image by that older live operator remains unverified; the initial tag-based override controls do not prove it and are excluded from the conclusive digest checks. The already parked m44 lifecycle record is not reopened here.

## Earlier attempts and environment recovery

All 60 raw live attempts are preserved: 58 successful and two unsuccessful. One upstream CLI wait exited 1 while a tag-based deploy remained in progress and later reached live; the first harness did not capture stderr, so its exact failure cause is unknown. One Blueprint harness used an unsupported `services[].port` field; the original rejection is retained, and the corrected eleven image cases passed. See [attempts and limits](evidence/attempts-and-limitations.json).

The local engine stopped after host disk exhaustion; one supported restart recovered it without resetting or pruning shared resources. Only dev-8's auth deployments and exhausted database/API connection processes were refreshed. An initial full backend run found an old deploy-hook success fixture using an image service for a Git ref; that fixture now uses a repository service. Reusing the preceding milestone's test database also exposed retired replay epochs; creating a second database then exposed cluster-wide test-role grants in the old database. Recreating only the dedicated test PostgreSQL instance resolved that fixture state. The final full suite passed. These earlier runs are not represented as green, and no live dev-8 database was reset.

## Parity and cleanup

[Primary sources and surface audit](evidence/parity-research.json) document Render's Git/image selector split. Its exact live commit-on-image response remains unverified; no Render account was mutated. Bex preserves its existing registry policy and ability to select another trusted repository, whereas Render documents a same-repository override restriction. The dashboard already gates specific-commit actions to eligible repository services and displays shared GraphQL errors; no dashboard code changed.

All **8/8 cleanup checks** passed: both services returned DELETE 204 then GET 404, the disposable identity and OAuth client returned 404, the owned tenant namespace and PVs were absent, and temporary Service forwards were stopped. Base dev-8 and dedicated test infrastructure remain for subsequent w8 items. The [hash index](evidence/SHA256SUMS.json) covers the sanitized artifacts; private state, credentials, CLI configs, environment files, kubeconfigs, and raw API logs are excluded.
