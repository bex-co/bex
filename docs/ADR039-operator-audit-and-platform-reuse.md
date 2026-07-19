# ADR039 — Operator correctness audit and platform reuse boundary

**Status:** Accepted · 2026-07-19

## Context

bex has two different products hidden behind the phrase "Render parity":

1. a Render-compatible control plane — REST wire shapes, GraphQL, MCP, the unmodified Render CLI, typed ids, workspace authorization, deploy records, audit, usage, and product-specific lifecycle semantics; and
2. a Kubernetes data plane — build an image, run a web service/worker/cron job, route traffic, observe health, and provision stateful resources.

The first is bex's differentiating compatibility contract. Much of the second is commodity platform engineering that existing controllers may already implement more safely. This ADR records a code-level audit of the current operator, researches existing operators and Kubernetes PaaS projects, and defines the boundary at which bex should reuse rather than rewrite.

This ADR supplements [ADR003](ADR003-control-plane.md), [ADR018](ADR018-render-parity.md), [ADR022](ADR022-tenant-isolation.md), [ADR028](ADR028-security-review.md), and [ADR034](ADR034-scalable-build-pipeline.md). O-01 and O-02 were closed by w2/m59 on 2026-07-19; ADR022 records the resulting verified build/admission boundary. O-03 through O-06 were closed by w2/m60 on the same date. O-07 through O-10 remain open remediation units.

## Audit scope and method

The review covered the controllers and helpers under `lego/operator`, the `App`/`Database`/`KeyValue` contracts under `lego/types`, the backend mutation paths that can change those contracts, and the rendered production manifests under `deploy/gitops/base`.

The review specifically traced:

- deletion timestamps, finalizers, owner references, and cross-namespace artifacts;
- primary and owned-resource watches, predicates, status-only events, observed generations, and requeue behavior;
- workload-kind transitions and desired-state pruning;
- PVC growth and plan downgrade behavior;
- build, publish, pre-deploy, registry-credential, service-account, namespace, node, and network boundaries; and
- mutable Secrets consumed by already-running stateful workloads.

The findings were compared against current upstream behavior in CloudNativePG, External Secrets Operator, Prometheus Operator, BuildKit, and Kubernetes documentation. Verification completed during the audit:

- `make test` passed, including envtest; controller-package statement coverage was approximately 76%;
- `make lint` passed with zero findings;
- targeted race tests for controller, build, publish, pre-deploy, and webhook packages passed; and
- `kubectl kustomize deploy/gitops/base` reproduced the admission-webhook selector mismatch.

Passing tests establish the current baseline; they do not close the missing lifecycle, transition, status-event, and isolation cases below.

## Findings

The identifiers are stable remediation units. O-09 and O-10 were found in the same review path but are kept separate because they have different failure modes and owners.

| ID | Severity | Status | Finding | User or platform impact | Required correction |
| --- | --- | --- | --- | --- | --- |
| O-01 | **Critical** | **Closed — w2/m59** | Untrusted build and pre-deploy execution crossed the platform namespace and credential boundary. | A tenant build could reach destinations not covered by tenant egress policy. The former BuildKit mode also made registry-push credential confidentiality unproven. | Dedicated `bex-build`, token/placement/network defaults, Pod-user-namespace BuildKit with its process sandbox, serial credential phases, and the 38-control live matrix now enforce the boundary. |
| O-02 | **High, latent** | **Closed — w2/m59** | The tenant-image signature webhook selected a namespace label that production did not set and failed open. | Enabling signing as documented would not have selected tenant Pods, so unsigned images could still run. | Workload object label selection, `failurePolicy: Fail`, zero-unavailable manager rollout, startup key validation, and signed/unsigned/tampered live controls close the activation gap. |
| O-03 | **High** | **Closed — w2/m60** | `Database` deletion could be filtered before finalization. | Backup-purge and public-TLS cleanup could leave a terminating CR or leaked artifacts. | A shared lifecycle predicate admits deletion timestamp and finalizer changes while filtering status noise; envtest proves metadata-only deletion and retry-after-cleanup-failure complete. |
| O-04 | **High** | **Closed — w2/m60** | Postgres and Valkey did not consistently implement the advertised grow-only storage contract. | A Postgres plan downgrade could request PVC shrink; a Valkey storage increase did not resize its PVC. | Status and live-resource high-water marks make storage monotonic; explicit shrink is rejected; Valkey patches a bound expandable PVC and reports requested/capacity convergence. |
| O-05 | **High** | **Closed — w2/m60** | `App.spec.type` was mutable while reconciliation did not prune resources from the previous type. | `web -> cron_job` could leave an old Deployment/Service/Ingress running; the reverse could leave a CronJob firing. | CRD admission makes the effective type immutable, and Blueprint updates reject every cross-type transition before mutation; delete-and-recreate is the supported path. |
| O-06 | **High** | **Closed — w2/m60** | Global generation predicates suppressed status-only events from owned Deployments/StatefulSets. | An App could remain `Ready` after its Pods crashed, and KeyValue could report readiness from an old revision. | Primary and child predicates are separate; Deployment, StatefulSet, Pod, and PVC updates enqueue their owner; readiness requires current rollout revision plus live Pod readiness and has a bounded safety requeue. |
| O-07 | **Medium-high** | Open | Cross-namespace build credential copies are not fully owned or deleted. | Clone, env/file, registry, kpack Secret, and per-App kpack ServiceAccount copies can outlive the App in the shared build namespace. | Give every copy an App-identity label, inventory all generated identities, and make finalization list/delete them idempotently. Prefer tenant-scoped build namespaces to reduce shared residue. |
| O-08 | **Medium-high** | Open | Finalizers acknowledge cleanup before cleanup succeeds. | Registry, static-object, backup, and TLS material can survive a successful API delete with no controller left to retry. Removed custom hosts can also leave TLS private keys. | Make cleanup synchronous or persist a cleanup Job and wait for terminal success; retry errors; remove finalizers only after every required artifact is absent; inventory historical host artifacts. |
| O-09 | **Medium** | Open | Mutating a KeyValue password Secret does not roll the StatefulSet. | Clients receive the new password while the running Valkey process still accepts the old password, causing an outage until restart. | Treat the generated credential as controller-owned/immutable, or add a deterministic Secret-content hash to the Pod template and define a safe rotation procedure. |
| O-10 | **Medium** | Open | Registry and Prometheus calls use unbounded default HTTP clients. | A stalled endpoint can hold scarce reconcile workers indefinitely and amplify the synchronous-build bottleneck. | Use shared clients with connect, TLS, response-header, and total request deadlines; propagate reconcile contexts and bound response bodies. |

### O-01: untrusted execution boundary — closed

w2/m59 moved BuildKit, kpack, static publish, and pre-deploy execution into the dedicated `bex-build` namespace. It has ingress/egress default-deny, narrow DNS/Zot/public-source rules, dynamic same-workspace datastore egress, Cilium host/remote-node/link-local denies, and explicit tenant-pool placement. Shared hardening stamps App/workspace identity, `automountServiceAccountToken: false`, `hostUsers: false`, and RuntimeDefault pod seccomp across direct execution variants; kpack uses the equivalent fields its upstream Image API exposes and a tokenless ServiceAccount.

The Dockerfile path is now `clone -> BuildKit OCI export -> Skopeo push -> optional Cosign sign`. Clone, private-base, App-repository, and signing credentials have different serial phase custody. BuildKit runs rootful only inside the Pod user namespace, keeps its default OCI process sandbox, and never mounts the output credential or signing key. Production explicitly enables the Kubernetes 1.31 `UserNamespacesSupport` gate and pins containerd 2.3.3/runc 1.5.1. The prior `--oci-worker-no-process-sandbox` flag is absent and regression-tested. The upstream [rootless process-sandbox warning](https://github.com/moby/buildkit/blob/6dd06999d5d369a217c3f3259a420f507e2db2c7/docs/rootless.md#L98-L104) remains the reason that mode is prohibited.

Live closure used authenticated private Git, a private Zot base, per-App repository authorization, and an adversarial Dockerfile. The resulting signed image reached `Running`. `scripts/verify-build-isolation.sh` then passed 38/38 named controls: allowed DNS/public Git/own Zot/same-workspace paths; denied metadata, kubelet, API server, real bex-api, platform fixture, cross-workspace service, and another existing repository; confirmed token/user-namespace/tenant-node/phase topology; and exercised signed allow, unsigned deny, post-signature tamper deny, and platform non-selection. `scripts/gitops-validate.sh` and table-driven generated-object tests provide cluster-less regression gates.

The live path also found and closed three masked compatibility defects: mirrored per-App docker configs now retain `.dockerconfigjson` and add Skopeo's `config.json` filename; configs include the standard base64 `auth` field as well as username/password; and the documented registry-rotation annotation is admitted by the App event predicate. Cosign uses the correct HTTP flag and disables public Rekor upload for private-key tenant signing.

### O-02: tenant-image admission activation — closed

The ValidatingWebhookConfiguration now uses `objectSelector.matchLabels[app.bex.co/verify-image]=enabled`, matching only workload templates emitted while signing is configured; it no longer depends on a nonexistent namespace label. `failurePolicy: Fail` makes verifier failure a denial. Manager startup fails if the signing Secret lacks `cosign.pub`, and a zero-unavailable rolling strategy keeps a Ready webhook endpoint during upgrades. The live controls above prove signed allow, unsigned/tampered deny, and unlabeled platform non-selection.

### O-03 through O-06: reconciliation correctness — closed

- **Database finalization:** the reusable primary-resource predicate admits spec generation, nonzero deletion timestamp, and finalizer changes while still filtering status-only noise. A real manager/envtest creates a finalized Database and owned CNPG Cluster, submits the metadata-only delete, and observes both disappear. An intercepted first cleanup failure separately proves the finalizer remains and the next reconcile completes. This follows [Kubebuilder finalizer guidance](https://kubebuilder.io/reference/using-finalizers) and the deletion-aware predicate pattern found in [External Secrets Operator](https://github.com/external-secrets/external-secrets/blob/fe392a2cb897270d376ac0ad91dc74e693cc95e1/pkg/controllers/externalsecret/externalsecret_controller.go#L1398).
- **Storage:** Postgres chooses the maximum of the new plan floor, explicit intent, persisted allocated high-water mark, and live CNPG request. A plan downgrade therefore cannot reduce storage; the REST/CLI PATCH path rejects an explicit shrink as a named 400 before mutation, while Blueprint sync applies the same rule. Valkey now discovers `data-<id>-0`, verifies that the bound StorageClass allows expansion, patches only a larger request, and waits for PVC capacity/filesystem convergence through `StorageReady`. Non-expandable classes fail with an actionable condition and a five-minute recheck rather than a hot loop. Tests cover Postgres downgrade/grow/shrink and Valkey real-PVC 1→5 GiB expansion. The mechanism follows [Kubernetes PVC expansion](https://kubernetes.io/docs/concepts/storage/persistent-volumes/#expanding-persistent-volumes-claims) and keeps CNPG as the lifecycle authority.
- **Type transition:** `AppSpec` carries CEL validation that compares the effective old/new type (`""` is the legacy spelling of `web_service`). The Blueprint apply path rejects the same semantic transition with a named bad request before it mutates the CR. Table-driven backend and direct-apiserver admission tests cover every ordered pair among all five workload kinds; same-type updates and the legacy empty/web spelling remain valid. Render likewise documents Blueprint service type as immutable, so delete-and-recreate is the supported transition.
- **Status events:** App and KeyValue now filter primary and child watches independently. Deployment/StatefulSet resource-version changes and labeled Pod/PVC status events enqueue their owner; App readiness requires the current Deployment generation and replicas plus current-revision Pod readiness, while KeyValue additionally requires `currentRevision == updateRevision`. Ready controllers retain a 30-second safety requeue. Manager/envtest proves a Running App leaves Ready on a Pod crash without an App spec edit, and Valkey tests prove old-revision replicas and pending PVC capacity cannot report Ready. The approach matches the child-watch patterns in [Prometheus Operator](https://github.com/prometheus-operator/prometheus-operator/blob/d7d70ebb896b1d655c1d7d88ae11a1ce6ba5b80c/pkg/operator/resource_reconciler.go#L541) and [CNPG](https://github.com/cloudnative-pg/cloudnative-pg/blob/e39ec6b4151236d142f7dbc6edc9b251ac24f156/internal/controller/cluster_controller.go#L1278).

## What an existing operator can and cannot replace

### The compatibility/data-plane split

No researched project is a drop-in implementation of bex's Render contract. Existing projects expose their own CRDs and APIs; none claims compatibility with Render's REST response shapes, official CLI, GraphQL schema, MCP tools, ids, deploy records, workspace roles, OAuth/device flow, audit/usage model, GitHub App flow, or bex's exact static-site, cron-history, managed Postgres/Valkey, and SSH semantics.

That does not mean bex should keep writing every controller. The reusable boundary is below `lego/types`:

```text
Render REST / GraphQL / MCP / CLI
               |
        bex core + authz
               |
       App / Database / KeyValue        <- stable bex contract
               |
     data-plane engine adapter          <- replaceable
        /          |          \
direct bex   OpenChoreo/Korifi   focused operators
controller       CRDs            (CNPG, Knative, KEDA, ...)
```

The bex API remains the source of product semantics. A selected engine may own build, rollout, route, health, and resource composition, but it must not leak its ids or lifecycle vocabulary into the Render-facing surfaces.

## Candidate code research

Research used official documentation and source as of 2026-07-19. Repository activity is evidence of maintenance, not a security or production-readiness guarantee.

| Candidate | What the code already provides | Material gaps or adoption cost | Disposition |
| --- | --- | --- | --- |
| **OpenChoreo** | Apache-2.0 Go controllers; separate control, workflow, data, observability, and experience planes; OpenAPI and MCP; component types that render Deployment/StatefulSet/CronJob/Job; immutable releases and bindings; Argo-based build workflows; ResourceTypes with outputs, secret references, readiness, garbage collection, and retention. The inspected tree at `f2fadb0` contained 576 Go test files under `internal` and reported version `1.2.0-rc.1`. | CNCF **Sandbox**, young project, public CRDs remain `v1alpha1`, and adoption introduces a second control plane plus a substantial topology/operations migration. It has no Render wire compatibility or bex-specific lifecycle. Production database examples deliberately require an external provisioner. | **Uncommitted future PoC candidate**, not current roadmap work or an immediate replacement. If explicitly reopened, preserve bex API/auth/id/audit and test `App -> Component/ComponentRelease/ReleaseBinding` plus managed resources via ResourceType targeting CNPG/Valkey. |
| **Cloud Foundry Korifi** | Apache-2.0 Go implementation of a compatible PaaS API backed by CRDs. Its `BuildWorkload`, `AppWorkload`, and `TaskWorkload` are explicit extension points; default controllers use kpack, StatefulSet, Job, and Gateway API. It also maps env, routes, services/bindings, logs, metrics, orgs, and spaces. Release `0.18.0` was published 2026-03-05; the inspected `273b857` tree had 309 test files in the relevant packages. | Implements a subset of **Cloud Foundry V3**, not Render. Default source flow is `cf push`, not bex's GitHub integration. Cron, static sites, managed-resource plans/backups, usage, SSH, and Render deploy semantics remain bex work. Full adoption duplicates the bex control-plane model and adds CF vocabulary/namespaces. | **Uncommitted future reference and possible second comparison.** If explicitly reopened after an OpenChoreo result, evaluate its pluggable workload contracts or a narrow backend, not its CF API as another public surface. |
| **Kubero Operator** | Heroku-like feature overlap: Git/image deploys, Dockerfile/Nixpacks/Buildpacks, web/worker, CronJob, HPA, ingress/TLS, sleep, add-ons, metrics, and push-to-deploy. | The operator at `ea8327a` is a GPL-3.0 Helm Operator: `watches.yaml` renders charts, the `KuberoApp` CRD preserves unknown `spec` fields, and no Go controller test suite exists in that repository. Its build templates explicitly set `automountServiceAccountToken: true`. The operator source last changed 2025-09-17, and its public GitHub release page lags the source version. GPL also prevents casually copying code into Apache-2.0 bex. No Render-compatible API. | **Reject as the core substrate.** It is useful as a product-feature checklist and perhaps an isolated black-box comparison, not as the quality/security answer to the current findings. |
| **Knative Serving** | Mature HTTP workload lifecycle with immutable revisions, routes, traffic splitting, request buffering, and scale-to-zero. | Stateless HTTP serving only; it does not supply Git builds, workers, cron history, static publishing, databases, Render APIs, or bex auth. Its networking and queue-proxy model would replace rather than merely wrap current Traefik/activator behavior. | **Focused evaluation** for web/private-service revisions and scale-to-zero after O-01–O-06. Do not force worker, cron, or stateful workloads through it. |
| **KEDA and Kueue** | KEDA owns event/metric-driven `0 <-> 1` activation and HPA-backed `1 <-> N`; Kueue owns resource-aware Job admission, quotas, and fair sharing. | Neither is a PaaS or compatibility layer. KEDA's HTTP scale-from-zero requires an interceptor/add-on; Kueue does not build images or make the current synchronous reconciler non-blocking. | **Prefer over custom loops** when the corresponding behavior is needed: KEDA for supported scaling signals, Kueue for the post-ADR034 build admission step. |
| **Crossplane / KubeVela** | Crossplane composes multiple resources behind a custom API and can target CNPG; KubeVela supplies extensible application components, traits, policies, and workflows. | Toolkits rather than Render platforms. bex would still author schemas/templates, lifecycle policy, compatibility, and tests. Crossplane's broad composition RBAC and another abstraction layer must be justified for two in-cluster datastores. | **Use selectively**, especially Crossplane only when bex provisions external cloud resources. OpenChoreo ResourceTypes would be the more coherent composition option inside any explicitly approved future OpenChoreo PoC. |
| **Cozystack** | CNCF Sandbox private-cloud/PaaS stack with Kubernetes, VMs, managed databases, applications, observability, and billing-oriented infrastructure. | It replaces much of `infra` and the cluster product, not just the App reconciler. It has no Render contract and would be a platform migration far beyond the failing controller mechanisms. | **Not an App-operator replacement.** Revisit only if bex chooses to replace its whole infrastructure substrate. |

Primary sources:

- OpenChoreo [architecture](https://openchoreo.dev/docs/overview/architecture/), [ComponentTypes](https://openchoreo.dev/docs/platform-engineer-guide/component-types/overview/), [ResourceTypes](https://openchoreo.dev/docs/platform-engineer-guide/resource-types/), [source](https://github.com/openchoreo/openchoreo), and [CNCF Sandbox record](https://www.cncf.io/projects/openchoreo/).
- Korifi [architecture and extension points](https://github.com/cloudfoundry/korifi/blob/273b857a7f2f1936626c4f75e962ca2490562dc1/docs/architecture.md), [API coverage](https://github.com/cloudfoundry/korifi/blob/273b857a7f2f1936626c4f75e962ca2490562dc1/docs/api.md), and [source/releases](https://github.com/cloudfoundry/korifi).
- Kubero [features](https://docs.kubero.dev/docs/usermanual/features/), [Helm-operator watches](https://github.com/kubero-dev/kubero-operator/blob/ea8327a69924ce61493dedfd02595d28ae81cbce/watches.yaml), [permissive KuberoApp CRD](https://github.com/kubero-dev/kubero-operator/blob/ea8327a69924ce61493dedfd02595d28ae81cbce/bundle/manifests/application.kubero.dev_kuberoapps.yaml), and [build Job templates](https://github.com/kubero-dev/kubero-operator/tree/ea8327a69924ce61493dedfd02595d28ae81cbce/helm-charts/kuberobuild/templates).
- Knative [Serving overview](https://knative.dev/docs/), [Services and revisions](https://knative.dev/docs/serving/services/), KEDA [scaling behavior](https://keda.sh/docs/2.20/concepts/scaling-deployments/), and Kueue [concepts](https://kueue.sigs.k8s.io/docs/concepts/).
- Crossplane [Compositions](https://docs.crossplane.io/latest/composition/compositions/), KubeVela [Application model](https://kubevela.io/docs/end-user/quick-start-cli/), and Cozystack's [CNCF project record](https://www.cncf.io/projects/cozystack/).

## Decision

### 1. Keep the Render-compatible bex control plane

Do not replace `lego/backend`, `lego/types`, or the Render-facing contracts with the public API of OpenChoreo, Korifi, Kubero, or another PaaS. A translation layer would still be required, and removing the bex layer would discard the compatibility work recorded in ADR018 and the CLI checklist.

### 2. Keep the direct reconciler and preserve only a future migration seam

Keep `App`, `Database`, and `KeyValue` as the stable mechanism contract. The direct controller remains the only committed implementation. If a future substrate evaluation is explicitly approved, define the smallest internal data-plane engine boundary needed to compile the same intent into candidate workload CRDs; do not build that abstraction speculatively. Any such boundary must expose observed generation, conditions, immutable release identity, cleanup completion, and artifact/log references without leaking engine-specific ids.

This is not permission to run two controllers against the same child resources. If a future engine is approved, exactly one engine must own a given App, selected by explicit platform configuration or migration state.

### 3. Future only: preserve an OpenChoreo-first, Korifi-second evaluation option

Neither candidate has a PM milestone or current implementation commitment. Completing O-01 through O-10 creates reusable evidence but does not automatically start a PoC. Evaluation begins only after an explicit future user decision and a fresh review of the candidates' then-current versions, APIs, maintenance, security posture, licensing, and operational footprint.

If reopened, an OpenChoreo-first spike must demonstrate:

1. prebuilt-image and Git-built web service deployment;
2. worker and cron workloads, including manual cron run/cancel/history owned by bex;
3. custom domain/TLS and scale-to-zero behavior;
4. env/file secret projection without secret values crossing unnecessary planes;
5. build, app, and request logs plus health/status propagation;
6. Postgres through CNPG and Valkey through a production ResourceType/operator, including grow-only storage and retention;
7. deletion with zero cross-namespace or registry/object-store residue;
8. namespace/network/service-account isolation at least as strict as O-01's closure criteria; and
9. unchanged Render REST/GraphQL/MCP/CLI acceptance tests against bex-api.

Korifi would be evaluated against the same representative workload only after the OpenChoreo result and a second explicit go/no-go, with special attention to whether its pluggable `BuildWorkload`/`AppWorkload`/`TaskWorkload` contracts can be reused without adopting the CF API and tenancy model.

Adopt a substrate only if a future approved spike removes a meaningful majority of bex's direct mechanism code, does not create two competing sources of truth, meets the isolation tests, and reduces total operational complexity. Feature count alone is not an acceptance criterion.

### 4. Reuse focused operators regardless of the full-platform result

- Keep CloudNativePG as the Postgres lifecycle authority; fix bex's plan/storage translation instead of reimplementing CNPG.
- Prefer Kueue over a home-grown distributed build semaphore once builds become non-blocking and exceed the two-worker baseline.
- Evaluate KEDA or Knative for scale-to-zero rather than expanding the bespoke activator loop without measurement.
- Use cert-manager/Gateway API-compatible controllers for certificate and route mechanics while bex retains Render domain semantics.
- Use External Secrets patterns or an operator only if the bex OpenBao API can preserve authorization, audit, and secret non-disclosure.
- Use Crossplane only for external managed resources whose lifecycle is otherwise provider-specific; do not add it merely to template a Deployment or StatefulSet.

### 5. Fix the audit independently of any future substrate evaluation

O-01 and O-02 were security activation blockers; O-03 through O-06 were correctness blockers. Their completed fixes apply to the direct engine and are acceptance tests for any replacement. O-07 through O-10 remain the next direct-controller remediation units.

The deferred replacement option must not become a reason to leave current production deletion, status, storage, or build isolation behavior unsafe.

## Consequences

### Positive

- bex preserves the product surface users and agents depend on while making commodity Kubernetes mechanisms replaceable;
- the audit becomes an executable acceptance suite for both the current operator and any new substrate;
- OpenChoreo and Korifi provide concrete, maintained reference implementations for immutable releases, workload extension points, status propagation, finalization, and tests;
- focused operators can remove bespoke scaling, admission, provisioning, and routing loops incrementally; and
- if a future PoC is approved, its result can reuse the audit acceptance suite without having delayed the direct operator fixes.

### Negative

- a full OpenChoreo or Korifi adoption adds CRDs, controllers, upgrades, failure modes, and a migration period;
- OpenChoreo is young and its CRDs are not yet stable; Korifi brings a second PaaS domain model; and
- introducing and maintaining a replaceable engine contract later would have a cost even if the direct controller remains the long-term implementation.

### Risks controlled by this decision

- **Feature-list selection:** candidates are judged by Render acceptance tests and isolation/lifecycle behavior, not marketing parity.
- **Double reconciliation:** one explicit owner per App prevents bex and a substrate from fighting over children.
- **API leakage:** engine-specific ids and states remain behind `lego/types` and core translation.
- **Migration lock-in:** the direct engine remains usable until a replacement proves cleanup, rollback, and status equivalence.
- **License contamination:** GPL Kubero code is not copied into Apache-2.0 bex.

## Follow-up acceptance register

| Order | Work | Exit evidence |
| --- | --- | --- |
| P0 — **DONE 2026-07-19** | O-01 build/pre-deploy/publish isolation and O-02 webhook activation | w2/m59: 38/38 adversarial live controls plus rendered/generated-object tests; private-source/private-base signed App reached Running; credential/network escapes and unsigned/tampered images were denied. |
| P1 — **DONE 2026-07-19** | O-03 finalizer events, O-04 storage, O-05 type immutability, O-06 child status watches | w2/m60: manager/envtests cover deletion, cleanup retry, Postgres downgrade/growth/shrink, real Valkey PVC growth, every type transition, crash-after-Ready, and failed StatefulSet rollout. |
| P2 | O-07 artifact inventory, O-08 durable cleanup, O-09 KeyValue rotation, O-10 HTTP deadlines | Delete/recreate leaves zero artifacts; cleanup retries survive manager restart; Secret mutation behavior is defined; stalled upstream calls time out. |

### Deferred future evaluation — not roadmap commitments

The following options are preserved for a possible future reopening. They are not sequenced after P2, must not be promoted automatically, and require an explicit user decision before PM materialization.

| Gate | Possible future work | Exit evidence if reopened |
| --- | --- | --- |
| Future A — uncommitted | OpenChoreo engine adapter | All nine spike scenarios pass through unchanged bex compatibility tests with one source of truth. |
| Future B — uncommitted | Korifi workload-contract comparison | Written result quantifies mechanism removed, translation retained, operational dependencies, and failed parity cases. |
| Future decision — uncommitted | Accept or reject substrate migration | Follow-up ADR with measured code/operations delta, migration/rollback plan, CRD version risk, and ownership map. |
