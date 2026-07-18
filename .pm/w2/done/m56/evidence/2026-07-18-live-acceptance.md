# m56 live acceptance — 2026-07-18

## Scope

Acceptance ran against the local two-node CAPD app cluster with the release-identity operator image and regenerated App CRD. The fixture was a repo-backed `web_service` using public source at `bex-co/bex`, `examples/hello-go`, with a configured pre-deploy command. No credential value, Secret data, or kubeconfig content was read or recorded.

The cluster's pre-existing BuildKit Jobs were blocked by its default Pod Security policy and its development registry was absent. For the genuine-build control only, those already-stuck Jobs were suspended, the build namespace temporarily allowed rootless BuildKit's required seccomp/AppArmor profile, and a throwaway anonymous registry plus empty Docker-config mount were created. After acceptance, the fixture, registry, mount Secret, temporary namespace label, extra build pods, and worker-count override were removed; the original Jobs were restored to unsuspended state.

## Legacy incident shape

A source-backed App was seeded with an active generation-1 image/revision, successful generation-1 pre-deploy status, no new identity fields, and a deliberately absent clone Secret. An operational metadata update backfilled the identity without source access.

Before scale:

```text
app generation=4 observed=4 releaseGeneration=1 revision=rev-1 phase=Running replicas=1
image=traefik/whoami:v1.10.3
pod=m55-source-scale-68b586bb4c-sqwqn ready=true hash=68b586bb4c
build/pre-deploy jobs for fixture: none
```

After `spec.replicas: 1 -> 2`:

```text
app generation=5 observed=5 releaseGeneration=1 revision=rev-1 phase=Running replicas=2
image=traefik/whoami:v1.10.3
pod=m55-source-scale-68b586bb4c-sqwqn ready=true hash=68b586bb4c
pod=m55-source-scale-68b586bb4c-xwnpg ready=true hash=68b586bb4c
build/pre-deploy jobs for fixture: none
```

The original pod remained Ready, the Deployment retained the generation-1 template hash and revision, and the absent clone Secret was never read.

## Genuine deploy control

A true source/build change at release generation 6 produced exactly these Jobs:

```text
bld-m55-source-scale-gen-6              Complete
predeploy-m55-source-scale-gen-6        Complete
```

BuildKit cloned the public repo, built `examples/hello-go`, pushed one image, and the operator persisted the digest-pinned candidate across the pre-deploy requeue. The healthy release converged as:

```text
releaseGeneration=6 revision=rev-6 phase=Running replicas=2
image=zot.bex-registry.svc:5000/m55-source-scale:gen-6@sha256:b232d1ab190964d7ab3f46a599540dad038425b3f73402d9a6e5f81b2d3ce0f6
pod-template-hash=7769bd955f
```

This control initially exposed that the built candidate image needed its own persisted status field while `status.image` continued to represent the active release. `status.artifactImage` was added, the operator was rebuilt, and the same completed build/pre-deploy artifacts then rolled the intended image successfully without a second build or pre-deploy Job.

## Genuine source-built 1→2 scale with unusable clone credential

After the genuine release, `spec.cloneSecret` was changed to a deliberately nonexistent name and the service was converged to one replica. This was operational only:

```text
app generation=8 observed=8 releaseGeneration=6 revision=rev-6 phase=Running replicas=1
pod=m55-source-scale-7769bd955f-n95hg ready=true hash=7769bd955f
```

Scaling that source-built service from one to two replicas produced:

```text
app generation=9 observed=9 releaseGeneration=6 revision=rev-6 phase=Running replicas=2
pod=m55-source-scale-7769bd955f-n95hg ready=true hash=7769bd955f
pod=m55-source-scale-7769bd955f-mh2ld ready=true hash=7769bd955f
jobs: bld-...-gen-6 and predeploy-...-gen-6 only; no generation-8/9 Job
```

The original generation-8 pod remained Ready, both pods used the same digest-pinned image and template hash, and neither the unusable clone credential nor build capacity participated in scale convergence.

## Public surfaces and deploy history

All public adapters delegate to the same `apps.Service.Scale` path. Their scale fixtures were source-backed and carried the same deliberately unusable clone-Secret reference. The final full suites exercised:

- REST: `TestRESTScaleService` accepted `POST /v1/services/{id}/scale` with HTTP 202.
- GraphQL: `TestGraphQLScaleService` accepted `scaleService` and returned the desired replica count.
- MCP: `TestMCP_ScaleDelegatesToCore` accepted `scale_service` and wrote the same intent.
- Dashboard: `use-scale-service.test.ts` verified accepted and rejected mutations, including the asynchronous `Scaling to 2 instance(s)…` acknowledgement; the scaling route test retained rejected draft input.
- Deploy history: `TestScaleManagedAppWritesRowThenCR` verified a managed-App scale updates the row and CR without calling `CreateDeploy`.
- Genuine deploy credential refresh: `TestTriggerRefreshesCloneSecret` verified backend deploy triggers mint and patch a fresh clone Secret before opening the deploy.

## Final checks

```text
operator: make test — PASS
backend: go test ./... — PASS
dashboard: yarn typecheck && yarn lint && yarn test && yarn build — PASS
Go lint: make lint — PASS (0 issues in operator and backend)
dashboard tests: 233 files, 1474 tests — PASS
```

The fixture and all temporary acceptance infrastructure were deleted after capture.
