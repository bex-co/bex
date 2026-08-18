# ADR070 — OpenChoreo whole-platform evaluation and plane ownership

> **Renumbered from ADR040 (2026-08-17, w6/m40).** `ADR040` was claimed by two documents; the number stays with [ADR040-billing-metronome.md](ADR040-billing-metronome.md), which carries ~5× the citations. Citations written before this date as "ADR040" may mean either document — read the context.

**Status:** Accepted — **Reject whole-platform and per-plane adoption** at OpenChoreo v1.1.2; retain as a pattern source and future watch item · 2026-07-20

## Context

[ADR039](ADR039-operator-audit-and-platform-reuse.md) separated bex's product contract from its Kubernetes mechanism and left an explicitly gated OpenChoreo-first evaluation. The original gate focused on `lego/operator`, but the evaluation installed all four deployable infrastructure planes and also had to assess the user-facing experience surface and cross-plane contracts. This ADR therefore answers the larger architecture question: should bex replace the whole platform, adopt any plane directly, reuse narrower upstream components, or learn selected designs while retaining bex ownership?

The required invariant is that adoption must preserve, unchanged:

- bex's Render-compatible REST, GraphQL, MCP, dashboard, and official-CLI behavior;
- the stable `App`, `Database`, and `KeyValue` contract and bex resource ids;
- bex authorization, audit, deploy records, usage, GitHub integration, secrets, and lifecycle semantics; and
- the correctness and isolation boundary closed by ADR039 O-01 through O-10.

OpenChoreo's [official architecture](https://openchoreo.dev/docs/overview/architecture/) defines five planes: Control, Experience, Workflow, Data, and Observability. This ADR evaluates all five. Identity/authorization/audit, managed resources/secrets, and fleet infrastructure are cross-plane concerns, so the comparison calls them out separately instead of hiding them in the nearest OpenChoreo plane.

This is not a general feature comparison. The choice must optimize the target architecture's reliability, security, product fit, strategic capability, and multi-year ownership cost under one clear owner per resource. Existing bex code is sunk cost and receives no preference. Migration and rollback are reported separately as execution risk, never used to make an inferior steady state look preferable. Similar names are not compatibility: OpenChoreo's Project, Component, ReleaseBinding, role, OpenAPI operation, or MCP tool does not have bex/Render semantics merely because it covers the same broad capability. Korifi remains outside this round; ADR039's future Korifi comparison is unchanged.

## Decision

**Reject OpenChoreo v1.1.2 as a replacement for bex as a whole and reject adopting any OpenChoreo plane wholesale.** Do not replace `bex-api` with this release, expose OpenChoreo's API/MCP/CLI as a bex surface, build a data-plane adapter, migrate Apps, install OpenChoreo in production, or run the direct and OpenChoreo reconcilers side by side.

The plane boundary is:

1. **Own product semantics.** bex retains one authoritative source of truth plus the Render REST/GraphQL/MCP/CLI contracts, ids, workspace authorization, audit, deploy records, usage, and lifecycle policy. The current database/core implementation is replaceable; those semantics are the product, not interchangeable infrastructure.
2. **Reuse commodity mechanisms directly.** Continue to run focused, mature projects behind bex-owned contracts: CloudNativePG, cert-manager, Ory Kratos/Hydra, OpenFGA, OpenBao, Gateway/Ingress implementations, and other narrowly accepted controllers. Evaluate KEDA, Knative, or Kueue independently when their bounded jobs justify them.
3. **Learn from platform implementations.** Apply proven OpenChoreo patterns—immutable release snapshots, environment bindings, declarative templates, reference-only secret propagation, outbound cluster connections, and observability adapters—only where they improve bex without importing OpenChoreo's resource model or source of truth.
4. **Keep platform adoption in the future.** OpenChoreo and Korifi are watch items, not roadmap dependencies. Reopening requires the plane-specific gates below and an explicit new decision.

The candidate successfully deployed prebuilt and Git-built web services, workers, and CronJobs; kept tested secret values in the data plane; composed CloudNativePG through a ResourceType; propagated build and application logs; survived a controller restart without request loss; and cleaned Kubernetes objects reliably. Those are useful design references.

It nevertheless fails five adoption gates and only partially meets two more. These are target-state failures, not objections to migration effort:

- stable v1.1.2's build pod is privileged, mounts a ServiceAccount token, and has no workflow-namespace egress policy, so it is materially weaker than O-01;
- it has no Render compatibility adapter, first-class custom-domain lifecycle, stable traffic wake-from-zero, Render request-log surface, or bex cron-run history;
- its generic CNPG readiness can report `Ready=True` before requested PVC capacity is real, and no production-ready Valkey operator met the gate;
- deleting a Git-built Component leaves the pushed registry tag behind; and
- after adding the required compatibility, lifecycle, and status adapters, no lower steady-state ownership or operational burden was demonstrated; the 33.4% removable-code upper bound is supporting evidence, not a deciding threshold.

OpenChoreo remains a **watch-only upstream**, especially for immutable releases, templating, ResourceTypes, outbound plane connectivity, observability adapters, and the KEDA module announced in v1.2. A future stable version may be reevaluated only after the hard gates at the end of this ADR are met.

## Decision vocabulary

This ADR uses four dispositions consistently:

| Disposition | Meaning for bex |
| --- | --- |
| **Own** | The semantics and source of truth stay in bex. Upstream design ideas may be reimplemented, but an upstream domain model must not leak into public contracts. |
| **Reuse** | Install or import a focused upstream component behind a narrow bex-owned boundary; do not fork its control logic into bex. |
| **Learn** | Study an implementation and adopt its invariant or algorithm in bex because directly using its API would create translation or ownership cost. Code copying still requires license/provenance review. |
| **Reject / watch** | Do not add the dependency, adapter, migration, or production operations now. Reconsider only after explicit gates are met. |

The rule is therefore **not** “build everything ourselves.” It is: **own the Render contract, reuse commodity controllers, learn proven distributed-systems patterns, and adopt a platform only when it removes more complexity than it adds.**

## Evaluation discipline: no incumbent advantage

The comparison applies the same gates to bex and OpenChoreo:

1. **Hard product gate:** preserve the Render-compatible contract, ids, authorization, audit, and durable lifecycle records, whether implemented natively or by an adapter.
2. **Hard safety gate:** no regression in build isolation, tenant boundaries, credential flow, state correctness, or deletion proof.
3. **Steady-state quality:** compare HA behavior, failure recovery, upgrade safety, operability, extensibility, ecosystem maturity, and the engineering/on-call burden over a multi-year horizon.
4. **Strategic capability:** credit a platform for capabilities bex would otherwise need to build, such as multi-cluster placement, release promotion, workflow orchestration, traces, or policy extension—even if adoption does not immediately delete more source lines.
5. **Migration feasibility:** measure adapter work, dual-running constraints, cutover, and rollback separately. Migration cost may affect sequencing and funding, but cannot convert a worse target architecture into a better one.

Under a zero-cost migration assumption, OpenChoreo v1.1.2 is still rejected because it fails product and safety gates. Conversely, a later release that passes those gates and offers lower long-term cost or materially better strategic capability must not be rejected merely because bex already has an implementation.

## Whole-platform plane comparison

| Plane or cross-plane concern | Required target state | bex evidence and weaknesses | OpenChoreo v1.1.2 evidence and weaknesses | Greenfield / steady-state decision |
| --- | --- | --- | --- | --- |
| **Control / product plane** | One authoritative model for Render-compatible workspaces, services, deploys, events, usage, and lifecycle. | Exact domain fit and substantial transactional/test evidence, but audit semantics, strict production configuration, and multi-replica loops need hardening. | Coherent API/controller model with strong platform abstractions, but its Namespace/Project/Component/Release hierarchy is a different product contract. A compatibility layer remains necessary even in a greenfield build. | **Build/retain a bex-owned product layer because Render compatibility requires it, not because the current implementation exists. Reuse or replace its internals when a better mechanism passes the same contract tests.** |
| **Experience plane: REST, GraphQL, MCP, CLI, UI** | One semantically consistent Render API plus GraphQL/MCP/dashboard and official-CLI behavior, backed by full contract tests. | All 119 authenticated REST operations in the bex-mux ∩ pinned-Render-spec intersection validate requests before side effects; response conformance remains CI-only and the adapters remain hand-written. | API-first OpenAPI/MCP/`occ`/Backstage experience is cohesive and extensible, but exposes OpenChoreo vocabulary rather than Render wire behavior. | **A bex compatibility surface is required greenfield. Reimplement it more spec-first if that lowers risk; do not equate preserving the contract with preserving every current adapter.** |
| **Identity, authorization, and audit** | Standards-based identity plus bex workspace roles, API keys, invites, fail-closed authorization, and transactionally truthful audit. | Focused Ory/OpenFGA engines are a sound decomposition, but optional OpenFGA configuration and pre-commit/fail-open audit violate the target state. | Integrated OIDC and hierarchical RBAC/ABAC are attractive, but its objects, roles, and audit semantics do not satisfy the bex contract. | **Own the domain policy; choose commodity engines independently. Current bex wiring must be fixed and may be replaced component-by-component if a better implementation preserves the policy.** |
| **Workflow / build plane** | Isolated untrusted builds, durable queued execution, cancellation, pre-deploy, provenance, logs, and horizontal scheduling. | Passes the stronger O-01 isolation boundary and bex deploy semantics, but synchronous reconciliation and limited distributed scheduling are long-term weaknesses. | Workflow/WorkflowRun abstractions, Argo execution, archive logs, and separate scaling are structurally stronger; the tested privileged/token-bearing/no-egress-policy build is an immediate security veto. | **Neither stack is the ideal end state today. Keep the safer path while evaluating a hardened OpenChoreo release or focused Argo/Kueue integration; prefer whichever proves better isolation and long-term scheduling/operations.** |
| **Runtime data plane** | Correct web/worker/cron/static rollout, health, domains, scale-to-zero, status, cleanup, and multi-tenant isolation with a replaceable engine boundary. | Strong live safety/lifecycle evidence and exact semantics, but bespoke controllers, synchronous build coupling, and limited release-promotion/multi-cluster abstractions increase future ownership. | Immutable releases, bindings, templates, and connected planes are stronger abstractions. Current custom-domain, wake-from-zero, health, registry deletion, and agent-privilege gaps fail the target state. | **Use the direct engine now on safety and correctness, not sunk cost. OpenChoreo may become the better engine if it closes the hard gaps and wins measured reliability/capability/TCO even when an adapter is still required.** |
| **Managed resources and secrets** | Production database/cache lifecycle, truthful capacity, backups, credentials, public access, usage, retention, deletion, and value-minimizing secret flow. | CNPG integration and bex product policy are mature; bespoke KeyValue and glue logic remain maintenance liabilities. | Generic ResourceTypes and reference-only secret transit are flexible and worked with CNPG, but readiness was falsely positive, retention was not backup, RBAC widened, and no production Valkey authority met the gate. | **Reuse CNPG/OpenBao directly under bex policy. Reconsider ResourceTypes on their lifecycle correctness and extensibility merits—not migration effort—once they pass the stateful gates.** |
| **Edge, networking, and elasticity** | Verified custom-domain lifecycle, TLS, routing, request metering, tenant network policy, and reliable scale-to-zero. | Domain semantics are stronger and cert-manager/Traefik are focused dependencies, but the bespoke activator is a risk and should compete against Knative/KEDA. | Gateway/Cell/module architecture has a stronger expansion path, but stable v1.1.2 lacked the tested activator/domain lifecycle and its agent authority was too broad. | **Keep product semantics and choose focused edge/elasticity controllers by evidence. An OpenChoreo module can win later without requiring wholesale platform adoption.** |
| **Observability plane** | Tenant-safe durable app/build/request logs, live tail, metrics, traces, alerts, usage inputs, and stable Render/bex query contracts. | Strong Render-specific log/metric behavior with a simpler deployed stack, but limited tracing and regional abstraction; operating Loki/Prometheus remains bex responsibility. | Richer logs/metrics/traces/alerts, domain enrichment, direct regional APIs, and adapters are strategically stronger; current schemas lack Render request/live-tail/usage behavior. | **No intrinsic preference for the existing stack. Select the backend/plane that preserves bex query semantics and demonstrates lower multi-year operations; avoid permanent duplicate storage during any transition.** |
| **Fleet / infrastructure plane** | Secure cluster provisioning, private connectivity, placement, failure containment, upgrades, and recovery at the scale the product actually needs. | CAPI/Terraform/Argo are simpler and already cover the current topology, but there is no equivalent mature multi-plane federation layer. | Outbound mTLS hub-and-spoke agents, multi-cluster environments, and promotion pipelines are materially stronger for regional scale, at the cost of more CRDs/controllers and independent upgrades. | **If multi-region/private-plane operation were required today, OpenChoreo's architecture could be superior. With no demonstrated requirement, defer the added steady-state complexity; reassess when the capability has real value.** |

## Complete operator capability inventory against OpenChoreo

This inventory is complete at the level of externally meaningful operator responsibilities as of 2026-07-20. It covers the manager plus the `activator`, `staticserver`, `pg-sni-proxy`, `kv-sni-proxy`, and `egress-meter` entrypoints. It combines fields that share one mechanism—for example, all literal and Secret-backed environment injection is one row—and excludes controller-runtime scaffolding and behavior owned by `bex-api`. Any new operator responsibility must add or amend a row here.

The bex evidence is the current [`AppReconciler`](../lego/operator/internal/controller/app_controller.go), [`DatabaseReconciler`](../lego/operator/internal/controller/database_controller.go), [`KeyValueReconciler`](../lego/operator/internal/controller/keyvalue_controller.go), [build package](../lego/operator/internal/build), [execution boundary](../lego/operator/internal/execution), [manager wiring](../lego/operator/cmd/manager/main.go), and [production manifests](../deploy/gitops/base/build-boundary.yaml). The OpenChoreo column is pinned to v1.1.2 and cross-checked against its [architecture](https://openchoreo.dev/docs/overview/architecture/), [ComponentType](https://openchoreo.dev/docs/reference/api/platform/componenttype/), [ReleaseBinding](https://openchoreo.dev/docs/reference/api/platform/releasebinding/), and [ResourceType](https://openchoreo.dev/docs/reference/api/platform/resourcetype/) contracts plus the evaluated tag. **Generic** means OpenChoreo can render a custom template or workflow; it does not mean the lifecycle is supplied, tested, or Render-compatible out of the box.

### Reconciliation, state, and controller operations

| Bex operator functionality | Current bex implementation | OpenChoreo v1.1.2 comparison | Missing control or decision |
| --- | --- | --- | --- |
| Typed desired state | Separate typed `App`, `Database`, and `KeyValue` CRDs with defaults, enums, ranges, and identity validation. | Typed platform/developer CRDs plus OpenAPI-shaped ComponentType/ResourceType schemas and CEL validation. **OpenChoreo stronger for extensible policy.** | Keep the typed product contract, but add admission/CEL for cross-field invariants now discovered only during reconcile. |
| Level-triggered convergence | Reapplies desired children on every reconcile; manager restart replays every App even without a spec generation change. | Controllers and render pipelines also reconcile declarative intent. **Comparable.** | Add an explicit drift test for every emitted child kind; current App watches do not cover every Service, Secret, Job, Certificate, or Traefik object. |
| Primary and child event filtering | Generation/deletion/finalizer predicates; status watches for Deployment, Pod, CronJob, CNPG Cluster, Job, StatefulSet, PVC, Service, and Secret where wired. | Shared field indexes, reference watchers, referrer finalizers, and per-controller watch files. **OpenChoreo pattern is more systematic.** | Maintain a generated owner/reference-to-watch matrix so adding a child cannot silently omit its wake-up path. |
| Service-kind immutability | Exhaustive App field classification and reconcile-time rejection of web/private/worker/cron/static transitions. | ComponentType `workloadType` is immutable and release objects have validating webhooks. **OpenChoreo rejects earlier.** | Move immutable and illegal-transition checks to admission while retaining the defensive reconcile check. |
| Artifact/release/operational change classification | Exhaustive field map decides whether a mutation rebuilds, reruns pre-deploy, rolls the workload, or only reconciles operations. | Immutable ComponentRelease plus environment ReleaseBinding separates revision content from placement. **OpenChoreo model is stronger.** | Persist an immutable bex release snapshot/render digest so classification is inspectable instead of encoded only in controller logic and status fingerprints. |
| Release identity and no-op suppression | Artifact and release fingerprints, release generation, deterministic build names, active revision, and normal reconciliation when an identity is absent. | Content-addressed immutable ComponentRelease and stable per-environment bindings. **OpenChoreo stronger for rollback/promotion.** | Add a first-class desired/rendered release record before adding environments; do not mirror five OpenChoreo CRDs merely to obtain it. |
| Current-revision rollout gating | Checks Deployment observed generation, updated replicas, availability, revision labels, PodReady, StatefulSet revisions, and PVC capacity. | RenderedRelease health plus per-kind heuristics or authored `readyWhen`. Bex checks are more product-specific. | **P0:** App and KeyValue Pod checks currently return true on List failure or when zero current Pods are found; make inconclusive evidence non-Ready and test stale parent status. |
| Status and observed generation | Public phases, `Ready` conditions, observed generation, active image/revision, pre-deploy, autoscale transition, run, storage, failover, and export status. | Central condition/reason helpers and layered status propagation across release/binding/rendered resources. **OpenChoreo is more standardized.** | Centralize typed reasons and transition helpers; several bex failure/progress paths discard status-update errors and can hide a stale public state. |
| Actionable rollout diagnosis | Surfaces CrashLoopBackOff exit code and image-pull failures rather than leaving an opaque timeout. | Generic resource health and observability troubleshooting. **Bex stronger for the Render contract.** | Extend the bounded reason set to scheduling, probe, quota, and admission failures without exposing tenant secrets. |
| Finalizers and exact-lifetime ownership | App UID labels prevent a recreated same-name App from adopting old cross-namespace artifacts; finalizers wait for observed absence. | Broad per-resource finalizers, referrer indexes, and explicit finalize files. **Both have useful controls.** | Adopt OpenChoreo's per-resource finalize structure and reference indexes as the bex controller grows; preserve UID ownership and absence proof. |
| External cleanup | Durable cleanup Jobs purge static S3 and database backup prefixes; App deletion removes TLS history, build artifacts, credentials, and registry manifests and then proves absence. | Kubernetes object cleanup was reliable, but the tested source-build registry tag remained. **Bex stronger.** | Keep deletion-conformance tests for every non-Kubernetes store and add periodic orphan inventory to detect finalizers that never ran. |
| Error retry and bounded I/O | Controller-runtime retries plus bounded registry, Prometheus, S3, and runtime HTTP operations. | Controllers retry through their plane abstractions, but the tested install/upgrade exposed several ordering failures. | Add fault-injection tests for API conflicts, timeouts, truncated responses, process death between side effect and status, and finalizer restart. |
| Controller concurrency | Configurable App workers and a per-workspace concurrent-build cap; one active manager via leader election. | Independent controller/workflow/plane processes and chart scaling knobs. **OpenChoreo stronger structurally.** | The build wait still occupies a reconcile worker and the cap is not a durable distributed admission queue. |
| Manager high availability | Production enables leader election but runs one manager replica with no manager PDB or anti-affinity. | Controller-manager Helm supports replicas/HPA/PDB/NetworkPolicy and leader election. **OpenChoreo packaging is stronger.** | Run at least two replicas across nodes, add a PDB, and prove leader failover while builds, finalizers, and database operations are active. |
| Manager readiness | `/healthz` and `/readyz` are controller-runtime ping checks. | Plane services expose health and packaging controls, but the evaluation did not prove full dependency-aware readiness. **Both incomplete.** | Ready must cover cache sync, webhook certificate/verifier activation, required CRDs, and strict production dependencies without making optional engines mandatory. |
| Metrics and reconcile SLOs | Authenticated controller-runtime metrics on TLS; database resize Events; component-specific proxy/meter metrics. | Controller/API HPA, PDB, NetworkPolicy, observability plane, alerts, logs, metrics, and traces. **OpenChoreo broader.** | Add per-controller latency/error/requeue/finalizer-age/build-queue metrics and alerts; controller-runtime totals alone are insufficient. |
| Upgrade and API-version safety | All product CRDs remain `v1alpha1`; no conversion webhook or explicit operator skew matrix. | OpenChoreo is also `v1alpha1`; the RC dry-run changed many CRDs and failed reused values. **Both gap.** | Define stored-version migration, conversion, downgrade, controller/CRD ordering, and backup/rollback rehearsal before either product claims stable upgrades. |

### Source build, release, registry, and supply chain

| Bex operator functionality | Current bex implementation | OpenChoreo v1.1.2 comparison | Missing control or decision |
| --- | --- | --- | --- |
| Prebuilt-image deploy | Deploys public or credentialed external images and preserves the image as release input. | First-class image-based Workload and evaluated successfully. **Comparable.** | Conformance-test digest, tag movement, pull-policy, registry outage, and credential rotation behavior. |
| Git source selection | Repo, branch/commit, root directory, Dockerfile path, clone Secret, build filter intent, and external registry pull credentials. | Workflows accept source parameters and the evaluated build handled Git source. **Generic/partial.** | Keep Render-shaped source semantics in bex; a workflow adapter would still have to translate and test every field. |
| Dockerfile build | Rootless-host BuildKit inside a Pod user namespace produces an OCI archive, then a separate container pushes it. | Argo workflow templates build and publish images. **OpenChoreo more extensible, tested default less isolated.** | Preserve the bex isolation boundary if replacing the scheduler or steps. |
| Buildpack build | kpack Image/Build integration, deterministic tags, history limits, cache, resource bounds, and condition mapping. | Buildpack workflows and generic WorkflowRun engine. **Comparable mechanism, OpenChoreo more pluggable.** | Decide independently whether kpack remains the best builder; do not require OpenChoreo to change it. |
| Native runtime build | Generates Dockerfiles for Elixir, Go, Node, Python, Ruby, and Rust; exact build/start commands and Secret-mounted build env. | Can be authored as workflows/ComponentTypes, not supplied with Render-native semantics. **Bex stronger out of box.** | Add runtime-version pinning and deprecation policy; current language base tags are platform defaults. |
| Build idempotence and lifetime safety | Deterministic Job/Image names, strict current-App UID ownership, newest-wins cancellation, and old-build rejection. | WorkflowRun has its own lifecycle/finalizer and immutable releases link outputs to deployments. **Comparable patterns.** | Make cancellation and ownership invariant across a future queue or external workflow engine. |
| Build admission and scheduling | Configurable reconcile workers plus an active-build scan enforce a workspace cap. | Dedicated Workflow Plane and Argo provide durable workflow objects and independent scheduling. **OpenChoreo stronger.** | Replace synchronous polling with a durable queue/admission controller; evaluate Kueue or a narrowly integrated workflow engine. |
| Pre-deploy command | Generation-bound Job gates rollout; failure leaves the previous release serving; credentials and image pull are isolated. | No Render-compatible pre-deploy lifecycle was demonstrated; a generic workflow could implement one. **Bex stronger.** | Keep it a release gate with idempotent retry/cancel semantics if build execution moves. |
| Runtime/build Secret materialization | Literal values, SecretKeyRefs, env groups, file projections, cross-namespace copies, and native-build secret mounts. | SecretReference/output wiring keeps DP Secret references and supports env/file bindings. **OpenChoreo has a stronger generic reference model.** | Introduce typed reference wiring only where it reduces bespoke copies; never send values through a control plane merely for abstraction symmetry. |
| Token and host isolation | Direct execution Pods disable ServiceAccount token mounting, set `hostUsers=false`, use RuntimeDefault seccomp, and run only on tenant nodes. | Evaluated v1.1.2 build Pod was privileged, token-bearing, and lacked workflow-namespace egress isolation. **Bex materially stronger.** | Keep the 38/38 live isolation suite as a non-negotiable adapter test. |
| Credential separation | Clone, private-base pull, output push, signing, runtime env, and tenant runtime pull credentials are separate and mounted only in the step that needs each. | Workflow ServiceAccount RBAC was narrow, but the evaluated build process did not prove the same credential/process separation. **Bex stronger.** | Add a machine-readable credential-flow test so a refactor cannot mount push/signing material in tenant Dockerfile execution. |
| Build network boundary | Dedicated build namespace, default deny, DNS/Zot/public-only egress, private/node/metadata denial, Cilium host denial, and per-App datastore exception for pre-deploy. | No workflow-namespace egress policy was present in the evaluated release. **Bex stronger.** | Continuously verify policy with real CNI tests and treat a non-Cilium deployment as a different, unproven security profile. |
| Immutable artifact pinning | Resolves registry digest, records artifact image/fingerprint, and uses the resolved artifact through pre-deploy and rollout. | Immutable ComponentRelease snapshots configuration, but not every authored workflow necessarily pins an image digest. | Enforce digest-only tenant deployments in production and reject a “successful” build whose digest cannot be resolved. |
| Image signing and admission verification | Optional cosign signing after push and a validating Pod webhook for tenant-registry images. | No built-in v1.1.2 signing or verification path was found. **Bex stronger.** | Make the production policy explicit: mandatory signing/verification or documented exception; verify every workload-creation path carries the selector label. |
| SBOM, provenance, and vulnerability policy | Not implemented beyond image digest and optional signature. | Not supplied by the evaluated default; generic workflows could add scanners/attestations. **Both gap.** | Add provenance/SBOM generation, admission policy, scanner update/exception lifecycle, and evidence retention without trusting a workflow merely because it completed. |
| Per-App registry isolation | Mints per-App bcrypt credentials and Zot ACL/retention entries; verifies credential activation before rollout; falls back to a shared pull credential only by configuration. | The evaluated registry path used workflow credentials but did not supply bex repository-per-App lifecycle. **Bex stronger.** | Remove the operational need to restart Zot for credential activation or prove the bounce protocol under concurrency and outage. |
| Strict registry authentication | Production config enables registry auth, but some cleanup/verifier helpers silently use anonymous access when the configured Secret is missing, malformed, or lacks the target registry. | The evaluated path also depended on workflow/registry configuration rather than a strict bex production profile. **Neither solves it.** | **P0:** once an auth Secret name is configured, missing or invalid credentials must fail startup/reconcile; anonymous fallback must be dev-only and explicit. |
| Registry/static/certificate cleanup | Deletes all manifest digests, proves tag absence, relies on Zot GC for blobs, purges static prefixes, deletes cross-namespace Jobs/Secrets, and remembers historical TLS Secrets. | Component/resource objects cleaned up, but the tested registry tag did not. **Bex stronger.** | Add registry-blob and object-store orphan audits, finalizer-age alerts, and a break-glass cleanup procedure that preserves lifetime checks. |
| Build and pre-deploy logs | Jobs and kpack Builds carry stable labels for backend log retrieval; failures map to public status. | Workflow/observability planes archive build and app logs. **OpenChoreo stronger for generic workflow history.** | Decouple durable logs from Pod lifetime and keep bex deploy/run ids as the query key. |
| Extensible workflow catalog | Hard-coded Dockerfile/buildpack/native/pre-deploy/static-publish mechanisms. | Typed Workflows, allowed workflow lists, CEL schemas, and independent WorkflowRuns. **OpenChoreo materially stronger.** | Add an extension seam only after the execution policy can constrain arbitrary steps; otherwise extensibility widens the tenant escape surface. |

### Workload types, rollout, edge, and tenant runtime

| Bex operator functionality | Current bex implementation | OpenChoreo v1.1.2 comparison | Missing control or decision |
| --- | --- | --- | --- |
| Web service | Deployment, Service, optional Ingress/TLS, URL, health gate, and current-revision status. | Deployment/service ComponentType with endpoints and HTTPRoute; evaluated successfully. **Comparable core.** | Preserve Render host/health/status behavior in any adapter. |
| Private service | Deployment plus ClusterIP Service and internal URL, without public Ingress. | Internal/namespace/project endpoint visibility. **OpenChoreo visibility model is richer.** | Consider explicit project/environment visibility only with a namespace/isolation redesign. |
| Background worker | Deployment only, no port/Service/Ingress/URL or request-driven sleep. | Worker ComponentType or custom deployment template. **Comparable.** | Add workload-specific availability and drain tests. |
| Cron job | CronJob with `ForbidConcurrent`, manual run Job, cancel intent, durable bounded run history, suspend, and status mapping. | CronJob workload type and workflows; no equivalent Render run/cancel/history contract demonstrated. **Bex stronger.** | Keep product run records outside ephemeral Kubernetes Jobs; test controller restarts between cancel/delete/status. |
| Static site | Build or direct clone, publish to object storage, shared signed origin proxy, cache, SPA fallback, rewrites/redirects, headers, IP allowlist, and deletion purge. | No first-class equivalent; templates/workflows could assemble pieces. **Bex stronger.** | A production profile must fail readiness when its required origin is missing/unreachable; keep the current Ready-but-503 behavior only for an explicit disabled profile. |
| OpenSandbox runtime | Alternate App runtime can create, pause, resume, delete, and resolve endpoints. | No direct equivalent. | Keep it isolated from the durable App engine decision; creation/rollover cleanup currently discards Delete errors and can lose the leaked sandbox id, in addition to the documented single-host/no-HA limits. |
| Deployment projection | Tier resources, replica count, port, start-command override, pull Secrets, owner refs, and stable selectors. | ComponentType can render Deployment, StatefulSet, CronJob, Job, proxy, and arbitrary supporting resources. **OpenChoreo more programmable.** | Add a pure render/plan phase and golden outputs so a monolithic mutation closure is not the only specification of desired state. |
| Environment and secret files | Literal env, SecretKeyRef, env-group Secrets, file projections, unshadowable `PORT`, and automatic stale-source removal. | Workload env/file bindings and resource-output dependency wiring. **OpenChoreo more generic.** | Add explicit missing-key/rotation readiness rather than relying only on Pod startup failures. |
| Health checks and rollout | HTTP readiness for services, TCP readiness for Valkey, Deployment/StatefulSet revision gates, and child-health polling. | Per-kind health plus authored `readyWhen`; default template behavior varies. **Bex more specific.** | Fix fail-open Pod evidence and add startup/liveness policy where the product contract needs it. |
| Graceful shutdown | Render `maxShutdownDelaySeconds` maps to Pod termination grace; default remains Kubernetes 30 seconds. | Template-configurable, not a uniform product contract. **Bex stronger out of box.** | Add preStop/drain integration tests for HTTP, worker, and long-running connections. |
| Restart, suspend, and resume | Declarative restart annotation and scale-to-zero while preserving desired replicas, host, TLS, PVC, and credentials. | New release/binding for config; workload suspension can be templated but no matching lifecycle was demonstrated. **Bex stronger.** | Keep intent idempotent and avoid imperative child mutation by APIs. |
| Autoscaling | Custom CPU/memory loop with min/max, tier normalization, transition history, metrics-server/Prometheus readers, and poll interval. | ComponentType may render HPA; stable v1.1.2 had no tested KEDA/scale-to-zero module. **Neither is the final target.** | Prefer HPA/KEDA when it can preserve Render semantics; current autoscaler silently disables if its client cannot initialize and has no HA-specific ownership test. |
| Idle hibernation and wake | Free-tier TTL, last-active annotation, zero replicas, Ingress swap to activator, wake request, and restoration. | Stable traffic wake-from-zero did not pass evaluation; KEDA module was future work. **Bex stronger today.** | Prove no lost first request, thundering-herd behavior, activator HA, and last-active durability across replicas. |
| Maintenance mode | Public route swaps to platform/custom 503 page without waking or mutating workload replicas; safe redirects and size limits. | No first-class equivalent found. **Bex stronger.** | Run activator with HA/PDB and dependency-aware readiness because it is now an availability component. |
| Platform and custom hosts | Platform subdomain, explicit hosts, per-host TLS Secret, independent cert issuance, stable primary TLS name, and URL status. | Generated gateway host/path and global wildcard TLS; no tested bex custom-domain lifecycle. **Bex stronger for parity.** | Add certificate status watches and domain/certificate SLOs instead of waiting for unrelated App reconciles. |
| Apex/`www` redirects | Dedicated TLS Ingress and RedirectRegex preserve path/query and first-match policy; stale redirect resources are removed. | Can be expressed in templates, not supplied as product behavior. **Bex stronger.** | Keep host ownership/collision enforcement in the product plane and add edge-controller portability tests. |
| IP allowlists | Service and environment layers chain Traefik middleware; database/Valkey proxies enforce source CIDRs after trusted PROXY parsing. | Endpoint visibility and generated NetworkPolicies; no Render-shaped public-source allowlist parity. | Share one tested CIDR normalization/evaluation library across HTTP and TCP paths. |
| East-west isolation | Per-App NetworkPolicy selects same workspace or isolated environment; execution policies restrict cross-namespace pre-deploy access. | Per-project/environment data-plane namespaces, endpoint visibility, and network-policy e2e tests. **OpenChoreo topology is stronger.** | Evaluate per-workspace/environment namespaces or equally strong namespace-scoped tenancy; bex's shared apps namespace makes every selector/RBAC mistake higher impact. |
| Tenant Pod hardening | No ServiceAccount token, all Linux capabilities dropped, no privilege escalation, RuntimeDefault seccomp; execution Pods additionally use Pod user namespaces and tenant nodes. | Templates can enforce policies, but evaluated build security regressed; data-plane runtime policy is configurable. | Decide whether ordinary runtime Pods should also set `hostUsers=false`; document compatibility exceptions instead of relying on ambient cluster policy. |
| Workload availability controls | Deployment rolling update defaults; no tier-driven PDB, topology spread, anti-affinity, or configurable surge policy for tenant Apps. | ComponentTypes can emit HPA/PDB/topology policy and RenderedRelease classifies these operational resources. **OpenChoreo stronger as a policy vehicle.** | Add HA-tier PDB/spread/rollout defaults and verify single-node/free-tier behavior remains schedulable. |
| Progressive delivery | Previous release remains while pre-deploy fails, but normal rollout is Kubernetes RollingUpdate with no canary/blue-green/traffic split. | Templates can emit gateway/rollout resources; no stable first-class progressive-delivery result was evaluated. **Both lack proven product behavior.** | Add only when required, behind a focused controller such as Argo Rollouts and bex deploy records. |
| External registry credentials | Per-service external pull Secret is attached to Deployment, CronJob, manual run, pre-deploy, and build where needed. | Workload Secret/reference mechanisms can express pull credentials. **Comparable generic mechanism.** | Test rotation and revocation without rebuilding; restrict Secrets to the exact runtime namespace and ServiceAccount path. |
| WebSocket and public egress accounting | Traefik WebSocket middleware plus node-local eBPF meter account backend/client and App/public bytes with stable resource labels. | Observability/FinOps modules are broader, but no matching Render usage mechanism was demonstrated. **Bex stronger for current contract.** | Preserve fail-closed counter-loss behavior and add kernel/CNI compatibility gates before admitting a node. |
| Environment promotion | No operator-native immutable promotion from dev to staging to prod; environments mainly affect policy/metadata. | DeploymentPipeline, immutable release, ReleaseBinding, and per-environment overrides. **OpenChoreo materially stronger.** | Add bex-native promotion only when environments require it; this is the highest-value design to learn, not a reason to import OpenChoreo ids. |
| Multi-cluster placement | One app cluster/namespace target per operator deployment. | Data/workflow/observability planes connect outbound over mTLS and environments can span data planes. **OpenChoreo materially stronger.** | Defer until regional/private-cluster placement is a real requirement; then benchmark its agents against a narrower bex fleet adapter. |
| Component/resource dependencies | Apps consume explicitly named Secrets/env groups; no generic typed output graph across services and resources. | Workload resource dependencies bind ResourceType outputs to env/files by reference. **OpenChoreo stronger.** | Learn the typed reference graph, but require cycle detection, authorization, rotation, deletion, and value-minimization before exposing it. |

### Managed Postgres and Key Value

| Bex operator functionality | Current bex implementation | OpenChoreo v1.1.2 comparison | Missing control or decision |
| --- | --- | --- | --- |
| Postgres authority | Thin projection from `Database` to CloudNativePG Cluster; CNPG owns database mechanics. | ResourceType can render CNPG and the evaluation provisioned it successfully. **Comparable composition, bex more specialized.** | Keep direct CNPG reuse unless a generic resource layer proves lower operational cost without weakening lifecycle truth. |
| Plans, versions, storage, parameters | Tier catalog selects instances/resources/storage; version and Postgres parameters map to CNPG; `pg_stat_statements` remains enforced. | ResourceType parameters/environment configs/CEL templates can express these. **OpenChoreo more programmable.** | Generate plan-to-CNPG golden tests and version-skew validation rather than trusting arbitrary templates. |
| Grow-only storage and truthful capacity | Rejects shrink before mutation; watches PVC requested and actual capacity; waits for expansion and records high-water allocation. | `readyWhen` can inspect applied status, but evaluated CNPG declared Ready before requested PVC capacity existed. **Bex stronger.** | Preserve capacity convergence as a reusable stateful-resource invariant. |
| Database disk autoscaling | Prometheus reads the fullest PVC, applies threshold/cap/debounce, persists resize history, and emits Events. | Could be a Trait/ResourceType or external operator; no equivalent evaluated. **Bex stronger today.** | Move to a focused autoscaler only if it preserves grow-only audit history and failure debouncing. |
| Backups and PITR | CNPG WAL/base backup configuration, daily ScheduledBackup, retention, versioned server name, and recovery/target-time input. | ResourceType retention controls object deletion, not backup policy; generic templates can include CNPG backup fields. **Bex stronger.** | Add automated restore drills and backup freshness readiness rather than equating configured with recoverable. |
| Logical exports | Durable pg_dump Jobs, object keys, terminal failure, status history, expiry, and cleanup Jobs. | Generic workflows could implement it; no bex-compatible contract supplied. **Bex stronger.** | Retain durable export ids/status if execution moves to a workflow engine. |
| Managed users and credentials | CNPG managed roles and generated connection Secret coordinates surfaced without copying password values into status. | ResourceType `secretKeyRef` outputs are a strong generic pattern. **Comparable security intent.** | Add rotation/revocation and consumer rollout semantics for managed users. |
| Pooler | Optional PgBouncer Pooler with internal/external coordinates and cleanup. | Can render a CNPG Pooler through ResourceType. **Generic.** | Readiness must include the Pooler when the product advertises its URL. |
| HA, read replicas, and failover | HA instance floor, readiness for standby, current primary/status, read-replica endpoints, and planned switchover intent. | ResourceType can template CNPG, but supplies no Render lifecycle/status by itself. **Bex stronger.** | Make failover patch failure visible; current helper logs and continues, so `LastFailoverAt` can advance without a confirmed switchover. |
| Major-version upgrade | Detects CNPG upgrade phase/failure, reverts failed target intent, and requests a fresh post-upgrade backup after success. | Generic applied-resource health; no equivalent product transition demonstrated. **Bex stronger.** | Add supported-version graph, preflight, backup proof, downtime budget, and live rollback rehearsal. |
| Database suspend/restart | Maps hibernation and restart intent to CNPG annotations while preserving storage and endpoints. | Templateable, not supplied as a uniform resource lifecycle. **Bex stronger.** | Verify backup/failover interaction while hibernated. |
| Public Postgres | Exact SNI host routing, PostgreSQL SSLRequest support, trusted PROXY source, layered allowlists, read-replica/pooler endpoints, and backend-to-client byte metering. | ResourceType can expose gateway resources but no matching TCP contract was demonstrated. **Bex stronger.** | Add proxy HA/PDB and failover/connection-drain tests; one proxy must not be a regional database outage point. |
| Database deletion | Explicitly deletes CNPG first, waits for absence, then runs durable backup-prefix purge before removing finalizer. | Delete/Retain plus finalizers; Retain evaluation left a terminating binding and did not prove backup purge. **Bex stronger.** | Add orphan backup inventory and an explicit legal-retention override rather than bypassing finalizer safety. |
| Key Value authority | Direct single-instance Valkey StatefulSet, headless Service, PVC, credentials, exporter, TLS, and public status. | Sample Valkey ResourceType exists and arbitrary operators can be composed, but no production-ready authority passed the gate. **Bex works but is strategically weak.** | Reevaluate a focused Valkey operator independently; do not mistake a ResourceType for replication/failover implementation. |
| Key Value plans and persistence | Tier resources/version, maxmemory policy, append-only persistence mode, PVC retention on delete, and safe-to-evict false. | ResourceType/StatefulSet templates can express fields. **Generic.** | Document durability guarantees and data-loss modes; a single replica is not managed-cache HA. |
| Key Value storage growth | Rejects shrink, expands PVC, verifies StorageClass capability, and waits for request/capacity/current StatefulSet revision. | Authored `readyWhen` can express this but must be correct. **Bex stronger.** | Fix the fail-open Pod list/zero-Pod check and keep live resize tests. |
| Key Value credentials | Random authority Secret, separate connection Secret, credential revision rollout annotation, and self-healing Secret watch. | ResourceType reference-only outputs/External Secret pattern is more generic. **Comparable intent.** | Add supported credential rotation rather than only preserving the initial password. |
| Public Key Value | Per-instance TLS certificate, SNI pass-through proxy, trusted client source, layered allowlist, connection coordinates, and byte metering. | Gateway/templates may express TCP/TLS, but no equivalent evaluated. **Bex stronger.** | Add certificate-status readiness and proxy HA. |
| Key Value suspend/resume | Scales StatefulSet to zero while retaining PVC, Secret, Service, certificate, and public name. | Templateable. **Bex stronger out of box.** | Test interrupted PVC expansion and credential rotation across suspend. |
| Key Value HA, failover, and backup | **Not implemented:** one Valkey replica, no sentinel/cluster failover, no backup/restore or rolling topology migration. | OpenChoreo itself does not supply these; a ResourceType can target another operator. **Both target states incomplete.** | This is the largest managed-resource gap: select and prove a production Valkey authority before claiming Render parity. |
| Generic managed-resource contract | Separate bespoke Database/KeyValue reconcilers and secret wiring. | ResourceType/ResourceRelease/Binding, outputs, `includeWhen`, `readyWhen`, environment config, and Delete/Retain. **OpenChoreo materially stronger for breadth.** | Prototype a bounded internal renderer only for the next real resource type; typed product readiness and cleanup remain mandatory. |

### Auxiliary data-plane services and security boundary

| Bex operator functionality | Current bex implementation | OpenChoreo v1.1.2 comparison | Missing control or decision |
| --- | --- | --- | --- |
| Wake/maintenance activator | Resolves host to App, wakes hibernated workloads, negotiates interstitial response, and serves safe platform/custom maintenance pages without workload mutation. | Stable v1.1.2 had no proven traffic activator. **Bex stronger.** | Add multiple replicas, PDB/anti-affinity, and idempotent concurrent-wake/load/first-request tests; avoid introducing shared wake state unless it is demonstrably required. |
| Static origin server | Periodically snapshots App host mappings, signs S3 reads, caches by byte budget, applies edge rules, and serves SPA fallback. | Observability/data-plane gateways do not supply this Render static-site origin. **Bex-specific.** | Readiness should expose stale resolver/origin state; add HA cache behavior and object consistency tests. |
| PostgreSQL SNI proxy | Parses direct TLS and PostgreSQL SSLRequest, routes exact host, enforces trusted PROXY source/allowlists, meters server egress, and fails closed on unknown host. | No direct equivalent. | Add connection/time/resource limits, HA, certificate-expiry/error metrics, and fuzzing for preamble/TLS parsers. |
| Key Value SNI proxy | TLS passthrough to exact Valkey host, trusted PROXY parsing, layered allowlists, metrics, and egress accounting. | No direct equivalent. | Apply the same HA, bounds, fuzzing, and config-staleness controls as the Postgres proxy. |
| Node-local public egress meter | eBPF maps Pod IP through immutable Pod UID collision checks to a stable App id, excludes private networks, checkpoints counters, detects loss/decrease, and exports Prometheus metrics. | FinOps/observability architecture is broader, but no equivalent measured contract was tested. | Fail node admission/readiness on unsupported kernel/CNI; alert on checkpoint age and counter-loss state. |
| Image admission webhook | Verifies cosign signatures for labeled tenant Pods and refuses manager startup if a configured public key is missing. | Broad validating webhooks for platform CRDs, but no equivalent image signature gate. **Bex stronger for supply chain.** | Configured registry auth must also fail closed; add bypass tests for every Pod-producing controller and label mutation path. |
| Secret cache and RBAC isolation | Secret informer is restricted to the configured apps namespace; build and registry Secret grants are separate namespace Roles; ClusterRole omits Secrets. | Secret references and plane separation reduce value transit, but the evaluated data-plane agent had broad cluster authority. **Bex stronger for Secret scope.** | Product intent uses one apps namespace, but the manager watches product CRs and can mutate Deployments, Jobs, Services, CNPG, routes, and policies cluster-wide; scope caches/controllers and replace those grants with namespace Roles where possible. |
| Plane/process blast radius | Builds use a separate namespace/node boundary; proxies and meter are separate entrypoints, but the manager owns App, Database, and KeyValue in one process and cluster. | Separate control, workflow, data, and observability planes connected outbound over mTLS. **OpenChoreo stronger for fleet isolation, heavier operationally.** | Split only when a measured failure/security boundary justifies it; first separate durable build scheduling from the manager. |
| Tenant namespace topology | One configured apps namespace carries all tenant App/Database/KeyValue resources; labels and policies express workspace/environment isolation. | Each project/environment gets a data-plane namespace/cell-like boundary. **OpenChoreo stronger.** | Run a threat-model and migration experiment for namespace-per-workspace/environment; compare quota, RBAC, DNS, Secret, and cleanup consequences. |
| Extensible platform policy | Stable behavior is compiled into Go and typed CRDs. | ComponentTypes, Traits, ResourceTypes, Workflows, CEL, and arbitrary rendered resources create platform golden paths. **OpenChoreo materially stronger.** | Do not expose arbitrary templates until RBAC, Secret references, allowed GVKs, resource bounds, policy validation, and deletion are sandboxed. |
| Logs, metrics, traces, and alerts | Operator emits controller/proxy/meter metrics and Kubernetes status; durable tenant logs/metrics live behind bex-api/Loki/Prometheus; traces and resource alert rules are limited. | Dedicated observability plane, regional adapters, logs, metrics, traces, alert rules, and notification channels. **OpenChoreo broader.** | Learn the adapter and alert-finalizer patterns; preserve one bex query/auth contract and avoid duplicating Loki and OpenSearch. |

### Implementation findings generated by the comparison

The comparison is useful only if it changes the retained implementation. These findings are new operator work; they are not reasons to adopt OpenChoreo wholesale and not claims that OpenChoreo already solves each item.

| ID | Priority | Finding and evidence | Pattern to take from OpenChoreo or another focused upstream | Closure evidence |
| --- | --- | --- | --- | --- |
| **OC-I01** | **P0 correctness** | `deploymentPodsReady` and `keyValuePodsReady` return true on Pod List error and when no current-revision Pod exists, so stale Deployment/StatefulSet status can produce a false Ready. | Treat health evidence as a monotonic conjunction; OpenChoreo's explicit per-resource conditions/`readyWhen` make the evidence dependency visible. | Unit and envtest cases for List failure, zero Pods, deleted current Pod with stale parent status, old revision only, and recovery to Ready. |
| **OC-I02** | **P0 security/configuration** | Registry cleanup and admission verification can fall back to anonymous calls after a configured push Secret is missing or invalid. | Use fail-fast plane/config validation and a named dev profile; never infer that configured auth is optional. | Manager startup/reconcile tests for missing Secret/key/registry entry/malformed JSON and live proof that no anonymous request is sent. |
| **OC-I03** | **P0 correctness** | Multiple App/Database/KeyValue progress/failure helpers discard `Status().Update` errors. Database failover additionally advances `LastFailoverAt` after a best-effort CNPG status patch that may have failed, so a requested switchover can be falsely acknowledged and never retried. | Central condition/reason/operation helpers with conflict retry and observed completion, as in OpenChoreo's shared condition and patch packages. | No ignored state writes; failover intent remains pending until the new primary is observed; conflict/error injection proves truthful retry without masking the primary error. |
| **OC-I04** | **P1 availability** | The manager, activator, and static server lack a complete HA/PDB/anti-affinity posture; TCP proxy availability depends on DaemonSet/LB topology but has no session-drain/failover proof; manager readiness is only ping. | Reuse OpenChoreo chart patterns for leader election, replicas, HPA/PDB, topology, and NetworkPolicy—not its domain model. | Two-node drain and kill tests during rollout/build/finalizer/wake/TCP sessions; dependency-aware readiness and zero lost ownership. |
| **OC-I05** | **P1 security** | Product intent uses one shared apps namespace, while controllers watch product CRs and hold workload mutation RBAC cluster-wide; a selector bug or manager compromise can affect more tenants and unrelated namespaces than necessary. | Learn project/environment namespace isolation and reference indexes; use scoped caches, Kubernetes Roles, and quotas as hard boundaries. | Threat model plus prototype comparing namespace-per-workspace/environment; scope reconciliation, reduce manager ClusterRole to documented irreducible cluster grants, and test cross-namespace denial. |
| **OC-I06** | **P1 maintainability/correctness** | The large App reconcile path is both renderer and executor; release intent is represented by distributed fingerprints/status rather than one immutable rendered snapshot. | Learn ComponentRelease/ReleaseBinding and pure component render-pipeline tests without importing OpenChoreo ids. | Pure render plan with golden/property tests, immutable bex release snapshot, explicit diff classes, and unchanged Render behavior. |
| **OC-I07** | **P1 scalability** | Source builds synchronously occupy reconcile workers and use a scan-based concurrency cap rather than durable distributed admission. | Evaluate Kueue or a narrowly integrated workflow engine; keep tokenless/user-namespaced steps and bex deploy ids. | Crash-safe queued/running/cancelled states, fairness/quotas, two-manager tests, no duplicate builds, and the complete O-01 isolation suite. |
| **OC-I08** | **P1 drift and leak detection** | App-owned-child watches are incomplete and there is no generated inventory tying every emitted GVK/external artifact to watch, poll, finalizer, or orphan-sweep behavior. OpenSandbox error/rollover cleanup also ignores Delete errors after ids may be lost. | Learn OpenChoreo's separate watch files, field indexes, referrer finalizers, and render inventory. | Machine-checked matrix for every child/external artifact, persisted cleanup intent, orphan inventory, and mutation/deletion drift tests proving bounded convergence. |
| **OC-I09** | **P1 supply chain** | Signing is optional and there is no SBOM, provenance, vulnerability policy, or scanner exception lifecycle. | Use focused Sigstore/SLSA/scanner policies; generic workflows may execute them but are not the trust decision. | Production profile refuses unsigned images, attestations bind source/build/image digest, admission policy and break-glass are tested and audited. |
| **OC-I10** | **P1 managed service** | Key Value is single-replica with no automated failover, backup, or restore. OpenChoreo's generic ResourceType does not supply those mechanics. | Evaluate a focused production Valkey operator, then place it behind the existing typed bex contract. | Failover, upgrade, resize, backup/restore, TLS/ACL rotation, public proxy, usage, and deletion tests under the selected authority. |
| **OC-I11** | **P1 workload availability** | Paid/HA Apps have no tier-driven PDB, topology spread, anti-affinity, or configurable rollout budget. | Learn policy-bearing ComponentTypes, or directly reuse focused Kubernetes primitives in the bex renderer. | Multi-node disruption/rollout tests and a schedulable single-node fallback for low tiers. |
| **OC-I12** | **P2 platform capability** | No immutable environment promotion/binding or generic typed resource-output graph exists. | Learn ReleaseBinding/DeploymentPipeline and ResourceType output references at the bex model boundary. | Product requirement, authorization/deletion/cycle design, and end-to-end dev→staging→prod promotion with unchanged ids and deploy history. |
| **OC-I13** | **P2 operability** | Reconcile/finalizer/build queue SLOs, traces, and resource alert lifecycle are incomplete. | Learn OpenChoreo's observer adapters and alert-rule cleanup finalizer while retaining Loki/Prometheus and bex authorization. | Dashboards and alerts for error rate, reconcile latency, stuck finalizer age, queue delay, dependency loss, and orphan inventory. |
| **OC-I14** | **Deferred capability** | No outbound-connected multi-cluster placement/fleet layer. | Reevaluate OpenChoreo's mTLS agents or a narrower fleet adapter only for an approved regional/private-cluster requirement. | Least-privilege agent, credential rotation, disconnection behavior, stable upgrade/rollback, and per-plane failure-containment tests. |

The immediate conclusion is therefore more specific than “keep bex”: fix OC-I01 through OC-I03 before adding operator scope; schedule OC-I04 through OC-I11 as hardening/reuse evaluations; keep OC-I12 through OC-I14 requirement-gated. OpenChoreo supplies several good structures, but its v1.1.2 build and agent boundaries are not security baselines for bex.

No row has an **OpenChoreo direct-reuse** decision at v1.1.2. That follows from hard product/safety failures or absent current requirements, not from loyalty to the incumbent. Workflow, runtime, observability, and fleet are genuine future competitions; a later result can replace the corresponding bex mechanism without reopening the decision to own Render semantics.

## Confidence boundary for the retained bex planes

Rejecting OpenChoreo v1.1.2 is **not** a claim that bex's current control and experience implementation is the best possible stack. The current disposition is **retain, harden, and keep replaceable below the product contract**. Any current component that fails the same target-state gates must be fixed or replaced.

The 2026-07-20 audit found a substantial implementation rather than a prototype:

- `lego/backend` has 62,766 production Go lines, 69,210 test Go lines, 219 test files, 1,740 named tests, and 49 paired schema migrations;
- `go test -count=1 ./...` passed, as did race-enabled tests for API, authorization, core, deploy, store, and webhook packages;
- CI exercises the store and authorization paths against real ephemeral Postgres and OpenFGA, rather than skipping those integrations;
- typed ids come from a closed registry, deploy creation/transition uses Postgres transactions and row locks, and a partial unique index enforces at most one open deploy per App; and
- production enables the Postgres control plane and OpenFGA, runs two anti-affined `bex-api` replicas behind a PDB, and has a live [external health endpoint](https://api.bex.co/healthz).

Those controls provide an auditable baseline. They neither prove superiority nor excuse these reliability gaps:

| Priority | Current bex gap | Does OpenChoreo v1.1.2 solve it? | Required target-state mechanism |
| --- | --- | --- | --- |
| **P0** | The outbound-webhook worker documents a single-replica assumption, while production runs two API replicas; both can claim and send the same delivery. | No. It does not implement bex webhook/event semantics; a full control-plane adoption would still require a compatible durable event path. | Use a transactional distributed claim such as `FOR UPDATE SKIP LOCKED`, a lease, and idempotency keys; prove no duplicate/lost delivery across crashes and two replicas. |
| **P0** | Authorization is configuration-fail-open: omitting `BEX_OPENFGA_URL` installs no checker and allows authenticated verbs. The configured checker itself fails closed on OpenFGA errors. | Not without replacing the authorization model. Its integrated policy path is a useful design reference, but its relations are not bex relations. | Add a production profile that refuses startup unless Postgres, OpenFGA, and required credentials are configured and reachable. Keep permissive defaults only for an explicit dev profile. |
| **P0** | Audit writes are bounded and fail-open; many allowed-write events are emitted at authorization time, before the state mutation commits, so a later failed mutation can look successful. | No compatible solution was demonstrated. OpenChoreo's resource/audit vocabulary does not prove transactional truth for bex writes. | Record successful effects after commit, preferably through the same transaction plus an outbox; define which audit failures must fail the product write and prove denial/success ordering. |
| **P1** | `/readyz` reflects drain state, not Postgres/OpenFGA/Hydra dependencies; background loops have no leader election; rate limits are in-memory and therefore multiplied by replica count. | Its controller architecture offers patterns, but the evaluated restart/upgrade evidence did not prove better dependency readiness, loop ownership, or compatible distributed rate limiting. | Add dependency-aware readiness, leader election or distributed claiming per loop, and a shared limiter or explicitly documented edge enforcement; run dependency-loss and failover tests. |

Two gaps found during the review were closed before this decision was accepted: the internal control-plane API now refuses startup without `BEX_CP_TOKEN` unless the explicit `BEX_CP_INSECURE=1` development override is set, and the complete pinned Render specification now validates requests for the exact 119-operation authenticated REST intersection before side effects. Response conformance remains CI-only with a stale-checked divergence allowlist.

Until the P0 items are closed, the current control/experience implementation does not meet this ADR's target-state bar. It may remain the execution path while the gaps are fixed, but its existence is not evidence against replacing any mechanism below the bex product contract.

## Evaluation pin and method

The live target was the latest stable release available on the evaluation date:

| Item | Pin |
| --- | --- |
| OpenChoreo | [`v1.1.2`](https://github.com/openchoreo/openchoreo/releases/tag/v1.1.2), commit [`0f8bad3`](https://github.com/openchoreo/openchoreo/tree/0f8bad3ec468b830ac4781987104d7473c246eee), published 2026-07-08 |
| Candidate not used for the verdict | [`v1.2.0-rc.1`](https://github.com/openchoreo/openchoreo/releases/tag/v1.2.0-rc.1), a prerelease published 2026-07-17 |
| Cluster | isolated one-node k3d v5.9.0, K3s v1.32.9+k3s1 |
| CNPG integration target | [chart 0.29.0](https://github.com/cloudnative-pg/charts/releases) / [CloudNativePG 1.30.0](https://github.com/cloudnative-pg/cloudnative-pg/releases/tag/v1.30.0) |
| Valkey operator review | [`valkey-operator v0.3.0`](https://github.com/valkey-io/valkey-operator/releases/tag/v0.3.0), commit [`196a306`](https://github.com/valkey-io/valkey-operator/tree/196a306aaa2179d8cd3eb3a6a6184cb6b325fae0) |

The stable source contained 559 Go test files and 33 OpenChoreo CRDs. The live setup followed the official [k3d installation](https://openchoreo.dev/docs/getting-started/try-it-out/on-k3d-locally/) and installed the control, data, workflow, and observability planes. Production topology was assessed against OpenChoreo's [deployment requirements](https://openchoreo.dev/docs/platform-engineer-guide/deployment-topology/).

Evidence was matched to each decision rather than treating every finding as a live end-to-end result:

| Scope | Evidence used |
| --- | --- |
| Control | Live install, resource/reconciliation inspection, controller restart, source review, and server-side upgrade rehearsal. |
| Experience and identity/authz | API/MCP/CLI/UI contract and source comparison, authenticated/unauthorized API probes, and bex compatibility inventory. A redundant Backstage UX acceptance run was not required after the underlying public contract failed. |
| Workflow, runtime, resources, secrets, and edge | Live prebuilt and Git builds, web/worker/cron workloads, secret projection, CNPG integration, scale/deletion behavior, and rendered security/RBAC inspection. |
| Observability | Live application/build log ingestion and queries, authorization probe, plus schema comparison against bex request logs, live tail, metrics, and usage. |
| Fleet and operations | Live plane registration/bootstrap/restart/uninstall, measured footprint, official topology requirements, and non-mutating v1.1.2-to-v1.2.0-rc.1 upgrade rehearsal. |

All writes were confined to the disposable `openchoreo` k3d cluster. No bex production or development cluster was changed. Secret checks compared hashes and references; no secret value was committed or recorded. The port-forward and cluster were removed after the evaluation.

## Acceptance result

The nine scenarios come directly from ADR039. A partial result is a failed adoption gate: it records reusable behavior but does not establish Render parity.

| # | Scenario | Result | Live evidence and consequence |
| --- | --- | --- | --- |
| 1 | Prebuilt and Git-built web services | **Pass** | A prebuilt React Component rendered through `Component -> ComponentRelease -> ReleaseBinding -> RenderedRelease` into a Ready Deployment, Service, NetworkPolicy, and HTTPRoute. A Git build checked out `openchoreo/sample-workloads`, built and pushed `default-default-greeting-service:v1-85b81ec3`, generated its Workload, deployed it, and returned `Hello, bex!`. |
| 2 | Worker and cron, including bex-owned manual run/cancel/history | **Partial** | Worker rendered as a Deployment and scheduled task as a `ForbidConcurrent` CronJob. A manual Kubernetes Job could be created from and deleted against that CronJob, but no OpenChoreo cron-run resource or durable actor/run/cancel history exists; the ReleaseBinding status did not change. bex would still own ADR038 semantics. |
| 3 | Custom domain/TLS and scale-to-zero | **Fail** | Setting replicas to zero produced zero desired replicas and `ReadyWithSuspendedResources`, but stable v1.1.2 has no activator or installed elasticity module to wake on traffic. Gateway-wide wildcard TLS is configurable, but there is no first-class domain ownership, verification, DNS instruction, redirect, or per-domain certificate lifecycle. A custom ComponentType/Trait could emit HTTPRoutes and Certificates, which is new bex platform code rather than parity supplied by OpenChoreo. |
| 4 | Env/file secrets without unnecessary plane crossing | **Pass with a bex-owned write boundary** | A value seeded directly into the isolated OpenBao store reached both an environment variable and mounted file through `SecretReference`/ExternalSecret`; both hashes matched. The control-plane namespace held no copied Kubernetes Secret, and ResourceType secret outputs contained only `{name,key}` references. OpenChoreo's convenience secret-write path sends a supplied value through its API before storing it, so any target architecture should preserve an authorized direct-to-secret-store write boundary. |
| 5 | Build, app, and request logs plus status propagation | **Partial** | The logs adapter returned seven application records for the test marker and 146 archived records for the completed build workflow. Workload and build status propagated through conditions. The OpenChoreo log schema covers component/workflow logs, not Render's `type=request`, `method`, `statusCode`, `path`, `host`, or label-discovery semantics; an adopted backend would need a compatible query/ingest adapter. Generic readiness also produced the false-positive storage case in scenario 6. |
| 6 | CNPG Postgres and production Valkey, grow-only storage and retention | **Fail** | A custom ResourceType successfully rendered CNPG after adding missing agent RBAC. It preserved the same CNPG UID and test row across a `1Gi -> 2Gi` spec update, and CNPG rejected `2Gi -> 512Mi`. However, on the non-expandable test StorageClass the Cluster spec became `2Gi` while PVC request/capacity stayed `1Gi`, yet OpenChoreo reported `Ready=True`. `Retain` deliberately held the binding finalizer and database; switching to `Delete` removed RenderedRelease, Cluster, Pod, Services, Secrets, and PVC in five seconds. The shipped Postgres/Valkey ResourceTypes are explicitly ephemeral demos. The official Valkey operator v0.3.0 explicitly says it is not production-ready and lacks backup/restore, managed external access, and standalone/sentinel modes. |
| 7 | Deletion with zero cross-namespace, registry, or object-store residue | **Fail** | Deleting a prebuilt Component removed its Component, Workload, binding, releases, and data-plane objects in 32 seconds. Deleting the Git-built service removed those objects plus WorkflowRun, Argo Workflow, and workflow pods in five seconds. The pushed registry tag still existed afterward, so bex's Zot inventory/delete/read-back finalizer cannot be removed. Static and backup object-store cleanup was therefore not delegated. |
| 8 | Isolation at least as strict as ADR039 O-01 | **Fail** | The actual image-build pod used `workflow-sa`, default token automount, and a projected token in the tenant build container; its main `ghcr.io/openchoreo/podman-runner:v1.1` container was `privileged: true`. `workflows-default` had no NetworkPolicy. The token's RBAC was narrow, but the pod retained unrestricted cluster networking. Separately, the data-plane agent had cluster-wide `*` over Secrets, ServiceAccounts, Namespaces, Roles, RoleBindings, ClusterRoles, and ClusterRoleBindings. These manifest facts already violate O-01, independent of a packet probe. |
| 9 | Unchanged Render REST/GraphQL/MCP/CLI acceptance | **Fail** | OpenChoreo source contains no Render `/v1/services` contract, and bex contains no OpenChoreo engine adapter. Its OpenAPI/MCP surface uses OpenChoreo resources and lifecycle. The unchanged bex compatibility suites therefore cannot target it without building and maintaining the translation layer this gate is meant to measure. |

Overall: **2 pass, 2 partial, 5 fail**. The result is not close enough to justify an implementation spike inside the product repository.

## Detailed findings

### Control and experience planes overlap with the product instead of replacing mechanism

OpenChoreo's control plane is internally coherent: one API server validates and authorizes its Developer and Platform APIs, controllers resolve intent, and the cluster gateway reaches other planes through outbound mTLS connections. Its experience plane consistently exposes the same model through OpenAPI, MCP, `occ`, and Backstage. Those are good platform-design properties.

They are not an implementation of bex's contract. Adopting them creates three possible ownership models:

1. OpenChoreo becomes the source of truth and bex is a facade. Every bex id, Render request, role, deploy record, event, status, and delete must be translated bidirectionally into OpenChoreo resources.
2. bex remains the source of truth and mirrors intent into OpenChoreo. This keeps the whole bex control plane and adds drift, retry, ordering, and finalization across two state stores.
3. Both accept writes. This creates competing owners and cannot provide deterministic rollback or audit.

The third is architecturally invalid. The first or second can be valid if an implementation proves contract equivalence and its steady-state reliability, capability, and multi-year operating cost beat a bex-native implementation. v1.1.2 did not provide that evidence and failed independent safety gates. Therefore bex retains ownership of the control/experience **contract**, while its implementation remains replaceable; source-line deletion and migration inconvenience are secondary considerations.

### Identity protocols are reusable; authorization and audit semantics are not

OpenChoreo and bex can both speak OAuth2/OIDC, but protocol-level interoperability stops at identity. OpenChoreo authorizes its hierarchy and resources; bex authorizes workspace roles and Render-shaped actions over bex ids. Existing grants, API-key/device flows, invite behavior, audit event vocabulary, retention, and service-event projection all depend on the bex model.

The reusable layer is the focused engine: continue using Kratos/Hydra and OpenFGA rather than writing identity or relationship-authorization servers. The product layer must remain in bex and must close the fail-open startup and audit-commit gaps recorded above. Replacing that layer with ThunderID or OpenChoreo roles would be a product migration, not reliability hardening.

### Release and workload mechanics are strong but not the product contract

OpenChoreo's immutable release chain and stable RenderedRelease name are useful. Updating the CNPG Resource pin changed the same RenderedRelease and underlying CNPG object rather than creating a second database. Component deletion also found and deleted the separately authored Workload, bindings, immutable releases, and data-plane objects.

The chain adds coordination work for a bex adapter, however. Every bex service mutation would have to manage Component, Workload, ComponentRelease, ReleaseBinding, and sometimes WorkflowRun. Cron run history, pre-deploy behavior, static publishing, domain state, deploy records, SSH, registry lifecycle, and product status still belong to bex. OpenChoreo ids and conditions must not leak through the stable bex API.

### ResourceTypes compose operators; they do not supply bex lifecycle policy

OpenChoreo's [ResourceType](https://openchoreo.dev/docs/platform-engineer-guide/resource-types/) model can target arbitrary Kubernetes operators, expose values or Secret references, evaluate CEL readiness, and apply `Delete`/`Retain`. That successfully composed CloudNativePG.

Three integration obligations remain:

1. The data-plane agent requires explicit RBAC for every added operator API. CNPG failed with `forbidden` until `postgresql.cnpg.io/clusters` verbs were added.
2. Readiness is only as strong as the authored CEL. CNPG `Ready=True` did not prove the requested PVC capacity, so bex's current high-water mark and request/capacity convergence logic remain necessary.
3. Retention is deletion behavior, not backup policy. `Retain` kept the binding in `Terminating` with its cleanup finalizer; it did not replace bex backup retention, purge, restore, export, public endpoint, or metering semantics.

The official Valkey operator is promising: it implements shards, replicas, failover, slot movement, rolling changes, TLS, ACLs, and PVC reclaim policy with 23 test files in v0.3.0. Its own [README](https://github.com/valkey-io/valkey-operator/tree/v0.3.0#readme) nevertheless labels the project early development and not ready for production. It had only three releases between 2026-05-11 and 2026-07-02, a `v1alpha1` API, and no backup/restore. It is a future watch item, not a basis for replacing the current KeyValue path.

### The stable build boundary regresses a closed critical finding

The evaluated workflow ServiceAccount could only manipulate Argo `workflowtaskresults`; it could not read Secrets or Pods, exec into Pods, create workloads, read Nodes, or create ClusterRoles. That is a useful least-privilege control.

It is insufficient because the actual build container was privileged, received the pod token, and had no egress isolation. ADR039 O-01 requires tokenless untrusted steps, pod user namespaces, bounded credentials, and verified denial of Kubernetes API, metadata, kubelet, platform, cross-workspace, and foreign-registry traffic. OpenChoreo v1.1.2 cannot inherit bex's 38/38 claim.

The v1.2.0-rc.1 changelog announces fixes for workflow shell injection and root access removal. That is positive upstream movement, but a prerelease fix does not make the tested stable release acceptable. Any reevaluation must inspect and run the resulting pod specs rather than infer safety from release notes.

### Observability is useful but narrower than Render parity

OpenChoreo's optional observability plane and OpenSearch logs module retained completed workflow logs and component logs. The authenticated Observer boundary also rejected an unauthorized system token with 403.

The adapter's model is component/workflow oriented. bex still needs its durable `type=request` access stream and Render filters, label discovery, live SSE tail, Postgres/Valkey resource dispatch, and REST/GraphQL/MCP response shapes. Running both OpenSearch and bex's Loki merely for overlapping app logs would increase rather than reduce operations; an adopted engine would need to feed or defer to the existing bex log contract.

## Operational evaluation

### Footprint

The evaluated build-and-logs stack used 12 OpenChoreo/prerequisite Helm releases; adding the already-selected CNPG operator made 13. It installed 33 OpenChoreo CRDs, 105 CRDs in total, 15 namespaces, and had 40 Running pods including Kubernetes system and test workloads. Major moving parts were:

- control, data, workflow, and observability planes and three cluster-agent connections;
- kgateway/Gateway API, cert-manager, External Secrets Operator, OpenBao, and Thunder;
- Argo Workflows plus a registry for source builds; and
- Fluent Bit, Observer, a logs adapter, and OpenSearch for the tested logs path.

The official production minimum is three nodes with at least four CPU and 8 GiB RAM per node. Some dependencies could theoretically be integrated with bex's existing services, but that would depart from the tested topology and create a custom distribution bex must own.

### Installation and documentation

The first control-plane Helm install registered validating webhooks before their Service had endpoints. The same release then failed while applying ten bootstrap `ClusterAuthzRoleBinding` objects. Waiting for the controller and retrying the identical install succeeded as Helm revision 2.

The data-plane chart returned `deployed` before `cluster-agent-tls` existed; an immediate `kubectl wait` returned NotFound, and the Secret appeared roughly 25 seconds later. Fluent Bit also remained in `FailedMount` until the official `/etc/machine-id` prerequisite was applied to the k3d node. The current guide documents that requirement, so it is an installation dependency to automate rather than an upstream documentation gap. These are recoverable bootstrap conditions, but they require an idempotent bex runbook rather than a blind Helm invocation.

### Restart and upgrade

Restarting the control-plane controller-manager caused no data-plane outage: all 80 probes during the rollout returned `Hello, bex!`, and the Deployment returned to one available/updated replica.

On startup, the controller emitted seven `ComponentRelease hash mismatch ... possible manual modification or data corruption` messages and rewrote releases for otherwise untouched Components. Traffic remained healthy, but false corruption signals and writes on every controller restart are operational noise that should be resolved before adoption.

There was no newer stable release against which to test a supported upgrade. A non-mutating rehearsal from v1.1.2 to v1.2.0-rc.1 was still informative:

- all four `helm upgrade --reuse-values --dry-run=server` commands failed;
- control-plane, workflow-plane, and observability-plane values failed new schemas, while data-plane rendering hit a missing `gateway.tlsPassthrough` value; and
- 36 control-plane CRD files changed and four CRDs were added between the tags.

The repository exposes contributor upgrade targets but no end-user stable upgrade/migration runbook was found. Because Helm does not upgrade `crds/` automatically, a production adoption would need an explicit CRD ordering, conversion, backup, rollback, and skew policy. The RC result is version-risk evidence, not a claim that a prerelease must accept old values unchanged.

### Uninstall, licensing, and project maturity

The exact disposable k3d cluster, its network, and attached volume were deleted successfully, leaving no evaluation k3d cluster. A plane-by-plane uninstall from a shared production cluster was not proven; CRD and retained-resource ordering would need its own rehearsal.

OpenChoreo is Apache-2.0 and a [CNCF Sandbox project](https://www.cncf.io/projects/openchoreo/), so license compatibility is good. Sandbox maturity, all public APIs at `openchoreo.dev/v1alpha1`, the broad data-plane agent authority, and rapid CRD/API change are adoption risks rather than reasons to avoid learning from the code.

## Measured code and ownership delta — secondary evidence

The current bex code baseline is:

| Scope           | Production Go lines | Test Go lines |
| --------------- | ------------------: | ------------: |
| `lego/types`    |               3,252 |           770 |
| `lego/operator` |              15,952 |        14,325 |
| `lego/backend`  |              62,766 |        69,210 |

These counts do not assign value to code merely because it already exists. They estimate the recurring surface each target architecture would own. The most generous OpenChoreo replacement envelope is 5,321 production lines:

- `app_controller.go`: 3,203;
- `internal/build`: 1,517;
- `internal/predeploy`: 283; and
- `release_identity.go`: 318.

That is **33.4% of operator production Go**, with 2,502 directly named test lines. It is deliberately an upper bound, not a savings forecast. These files also contain behavior OpenChoreo did not replace: static sites, pre-deploy semantics, domain/route policy, registry credentials and deletion, secret materialization, release/status translation, isolation, health gating, and bex lifecycle. A steady-state OpenChoreo design must count the adapter and the retained behaviors against any removed direct-controller code.

The following remain unequivocally owned by bex:

- all of `lego/backend` and `lego/types`;
- Render ids, wire formats, CLI behavior, authorization, audit, usage, and deploy history;
- Database and KeyValue product policy on top of CNPG/current Valkey mechanics;
- cron manual-run/cancel/history, static publishing, pre-deploy, and SSH;
- custom-domain ownership, DNS instructions, redirect/TLS history, and cleanup;
- request logs and metrics semantics;
- OpenBao authorization and secret write paths; and
- registry ACL, signing/admission, retention, and verified deletion.

An OpenChoreo adapter would add at least five kinds of recurring coordination: intent translation, immutable release generation/pinning, WorkflowRun-to-deploy orchestration, status/log translation, and multi-object finalization. It would also add the multi-plane upgrade and security surface above. v1.1.2 did not demonstrate enough reliability or strategic-capability benefit to offset that steady-state ownership. This is not a permanent “delete more than half the code” rule: a future engine may be superior with less code deletion if it measurably reduces incidents/on-call cost or enables a required capability more safely.

## Reuse and learning register

The following table is the actionable boundary. “Reuse” means depend on and operate the upstream project through its supported API. “Learn” means carry the invariant into bex without adopting OpenChoreo as another source of truth.

| Capability | Action | Boundary |
| --- | --- | --- |
| Postgres lifecycle | **Reuse CloudNativePG** | CNPG owns database replication/PVC mechanics; bex owns plans, connection metadata, backup policy, public access, usage, and Render lifecycle. |
| Identity and OAuth | **Reuse Kratos/Hydra** | Ory owns protocol machinery; bex owns onboarding, API keys, device flow, workspace membership, and public responses. |
| Relationship authorization | **Reuse OpenFGA** | OpenFGA evaluates tuples; bex owns the relation model, production fail-closed configuration, and every service-level authorization gate. |
| Tenant secrets | **Reuse OpenBao** | OpenBao stores secret values; bex owns authorization, audit, projection, and the rule that values do not cross unnecessary planes. |
| Certificates and routing | **Reuse cert-manager and Kubernetes edge APIs/controllers** | Upstreams own certificate/route mechanics; bex owns domain verification, DNS instructions, redirects, status, history, and cleanup. |
| Build admission and elasticity | **Evaluate Kueue/KEDA/Knative independently** | Adopt only a bounded controller that passes bex security and behavior tests; an OpenChoreo installation is not a prerequisite. |
| Immutable release snapshot plus environment binding | **Learn from OpenChoreo** | Model revision immutability and promotion in bex deploy records/types only if it simplifies existing lifecycle; do not mirror ComponentRelease/ReleaseBinding objects speculatively. |
| Declarative ComponentType/Trait/ResourceType rendering | **Learn selectively** | Prefer typed bex fields for stable product behavior. Introduce a template seam only for genuine platform extensions, with truthful readiness and constrained RBAC. |
| Reference-only secret propagation | **Learn and preserve** | Pass typed secret references across reconciliation boundaries; never route tenant values through a new control-plane API for convenience. |
| Outbound mTLS plane connection | **Learn for future fleet scale** | Revisit when bex has a concrete multi-region/private-cluster requirement; do not add agents and long-lived tunnels to the current topology preemptively. |
| Observability adapters and domain enrichment | **Learn** | Keep Render/bex query schemas stable while allowing Loki/Prometheus/OTel backends to evolve behind them; avoid duplicating OpenSearch and Loki. |

OpenChoreo and Korifi remain valuable references. Neither is approved as a library, controller set, public surface, or runtime dependency by this ADR.

## Ownership, migration, and rollback

No migration is authorized by this version-pinned result. That is a consequence of failed target-state gates, not a desire to avoid migration. Control, experience, identity, audit, and product state remain bex-owned even if a later workload engine passes the reopening gates. The only acceptable engine ownership model remains:

```text
bex API/store/types (product source of truth)
                 |
          one engine adapter
            /          \
      direct bex     OpenChoreo
      per App         per App
```

An App must have exactly one active engine. A future migration would need to record the engine and phase in bex's source of truth, stop the old owner before creating the new owner's children, verify status/routes/logs, then delete and prove absence of old artifacts. Rollback is safe only before destructive old-artifact cleanup or after rebuilding the old engine from bex intent; OpenChoreo resources must never become the sole copy of product intent.

Databases and KeyValues should remain on their existing bex contracts even if a future workload engine changes. ResourceTypes may become an implementation detail only after they reproduce grow-only capacity readiness, backups, public access, retention, deletion, and credentials without gaining product ownership.

## Reopening gates by plane

Do not reopen implementation work merely because a new version exists.

### Control, experience, identity, authorization, and audit

These planes are closed by product strategy, not by an upstream version number. Reopening requires a new product ADR that explicitly accepts one of these outcomes:

- bex no longer promises Render API/CLI compatibility;
- a complete adapter preserves ids, authorization, audit, deploy/event history, REST, GraphQL, MCP, dashboard, and official CLI behavior while measurably improving steady-state reliability, capability, or multi-year ownership cost; or
- OpenChoreo itself adopts the bex/Render contract as a supported, versioned upstream interface.

An OpenChoreo release that is more stable, more secure, or more feature-complete does not by itself satisfy this gate.

### Workflow and runtime data planes

A future stable OpenChoreo evaluation must first show:

1. non-privileged, tokenless or equivalently bounded untrusted build steps and the complete ADR039 O-01 live isolation matrix;
2. a supported stable-to-stable upgrade and rollback runbook, including CRD ordering and version skew;
3. stable traffic wake-from-zero plus a first-class or narrowly extensible custom-domain/TLS lifecycle;
4. registry and object-store deletion with observed zero residue;
5. bex-owned pre-deploy, deploy, cron run/cancel/history, signing, and cancellation semantics through the adapter;
6. narrowed plane-agent privileges and a documented multi-tenant threat model; and
7. a measured steady-state win in reliability, required capability, or multi-year engineering/operational cost after counting the adapter and plane operations; source-line reduction is supporting evidence only.

### Managed resources and secrets

ResourceTypes may be reconsidered only if they prove actual PVC-capacity readiness, grow-only storage, backup/restore, public access, metering, credential consistency, retention, and deletion without making generic conditions or secret values the product source of truth. A production-ready Valkey authority must be selected independently; CNPG remains the direct Postgres authority.

### Observability plane

Adoption requires unchanged app/build/request log and metric behavior across REST, GraphQL, MCP, dashboard, and the official CLI; equivalent structured filters and live tail; tenant authorization; usage inputs; retention; regional failure behavior; and a measured plan that replaces rather than duplicates Loki/Prometheus storage and operations.

### Fleet infrastructure

Reopen only for a concrete multi-cluster or data-residency requirement. The evaluation must prove plane disconnection behavior, credential rotation, least-privilege agents, HA, stable-to-stable upgrades, partial and full uninstall, backup/restore, and per-plane failure containment.

Until those gates exist and pass, reuse focused upstream operators as ADR039 already decided and keep OpenChoreo/Korifi in the future-options register.

## Consequences

### Positive

- every plane now has an explicit owner, reuse boundary, and reopening rule rather than an undifferentiated “adopt or build” choice;
- the evaluation gives no credit to sunk implementation cost and separates target-state quality from migration feasibility;
- bex does not accept v1.1.2's measured security regression against ADR039's operator boundary;
- the decision is based on live release, build, secret, log, resource, deletion, restart, and upgrade evidence rather than a feature list;
- the retained control/experience decision records its own P0/P1 reliability gaps instead of treating ownership as proof of maturity;
- focused upstream engines remain directly reusable without importing another PaaS domain model;
- OpenChoreo remains a concrete source of release, template, multi-plane, and observability patterns; and
- Korifi stays a separate future question rather than being silently promoted after this rejection.

### Negative

- bex continues to maintain the direct workload reconciler and its tests;
- bex must close the multi-replica worker, strict production authorization configuration, audit-commit, dependency-readiness/leadership, and distributed-rate-limit gaps itself;
- retaining the current execution path creates a risk of inertia, so future comparisons must publish the greenfield verdict and steady-state cost before discussing migration;
- OpenChoreo improvements will not arrive automatically; and
- a future reevaluation must repeat the security, lifecycle, compatibility, and operational tests against a then-current stable release for every plane it proposes to adopt.
