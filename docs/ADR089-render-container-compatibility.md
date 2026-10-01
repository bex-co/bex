# ADR089 — Deploy Render Docker examples without changing their source

**Status:** Proposed, 2026-09-29. Experimental implementation is in progress; hosted creation remains disabled by default. The end-to-end deployment and isolation gates have not passed.

**Scope:** Import and run the unchanged `render-examples/clickhouse` repository, with a reusable contract for legacy Blueprint aliases, container initialization, and multiple private TCP ports. This does not establish complete Render runtime parity.

**Related decisions:** [ADR049](ADR049-render-yaml-parity.md) owns Blueprint semantics; [ADR022](ADR022-tenant-isolation.md) owns the retained container hardening policy; [ADR043](ADR043-tenant-namespace-isolation.md) owns workspace isolation; [ADR041](ADR041-service-addresses.md) owns addresses and the current single-port limitation; [ADR082](ADR082-persistent-disks.md) owns persistent storage. Acceptance would amend ADR022's unconditional capability drop for the new execution policy and ADR041's single-port restriction. Their existing contracts remain in force while this ADR is proposed.

## Customer outcome

A customer connects `https://github.com/render-examples/clickhouse`, selects its `master` branch and `render.yaml`, reviews the compute/storage configuration, and deploys. They do not rewrite the Dockerfile, change `env` to `runtime`, supply a ClickHouse-specific Bex manifest, or patch Kubernetes resources.

The resulting private service has a persistent disk at `/var/lib/clickhouse`. An allowed peer connects through its stable hostname on HTTP port 8123 or native TCP port 9000. A row remains after replacement of the container. There is no public ingress.

Keep compatibility testing separate from recommending a database version: the upstream example currently uses ClickHouse 21. A customer guide should explain the legacy image and authentication setup rather than describe it as the latest stable release.

## Evidence and current gaps

Research used Bex source at `f064e0a225269d324f18bcea9dcad78358faff64` and upstream example commit `355817b1d95c1a9f2b088474437593234f4027f4` on 2026-09-29. These are source revisions, not assertions about the version deployed at api.bex.co.

The upstream [Dockerfile](https://github.com/render-examples/clickhouse/blob/355817b1d95c1a9f2b088474437593234f4027f4/Dockerfile) contains `FROM yandex/clickhouse-server:21`. Its [Blueprint](https://github.com/render-examples/clickhouse/blob/355817b1d95c1a9f2b088474437593234f4027f4/render.yaml) declares a private Docker service, `plan: standard`, disabled automatic deploys, and a 10 GB disk at `/var/lib/clickhouse`.

| Layer | Observed behavior | Cause |
| --- | --- | --- |
| Blueprint | The CMS checker returns missing `runtime` and unknown `services[0].env`. | The pinned service schema requires `runtime`; the compiler has no compatibility normalization for this legacy spelling. |
| Startup | The upstream image exits with `failed switching to "101:101": operation not permitted` under Bex's container restrictions. | Its entrypoint switches identity and prepares owned directories; `tenantSecCtx()` removes the capabilities those operations need. |
| Networking | Bex defaults to 3000 and publishes one port under each service hostname. | `AppSpec.EffectivePort()` and `applyClusterIPService()` implement ADR041's single-port decision; they do not derive ports from the image. |
| Storage | The declared disk shape passes validation, but the implementation test found that create did not persist the disk row. | The store projector owns `spec.disk` and can remove it when no row exists. Service creation must save the disk and its first metering period in the service transaction. |

Render's [Blueprint reference](https://render.com/docs/blueprint-spec#runtime) identifies `env` as the predecessor to `runtime`. ADR049 D2 already promises normalization of documented deprecated aliases. This is a compatibility omission, not an intentional rejection of the Docker workload. The vendored schema is useful validation evidence, but it is not an exhaustive inventory of historical customer files.

Render documents [private services](https://render.com/docs/private-services) that bind at least one listener and [private networking](https://render.com/docs/private-network#port-restrictions) with multiple ports. That establishes the customer-visible requirement; it does not reveal Render's container capability policy or port-discovery implementation.

### Local runtime experiment

The amd64 `yandex/clickhouse-server:21` image resolved to manifest digest `sha256:edfee043e4f909dd471c6e282ce3cfd0ce90a4cad3fc234cb27633debe26ea05` and reported version `21.12.4.1`. It declares TCP ports 8123, 9000, and 9009.

With one CPU, 2 GiB memory, a fresh Docker volume with copying disabled, `--cap-drop=ALL`, and `--security-opt=no-new-privileges`, startup failed at the user switch. Adding `CHOWN`, `SETUID`, and `SETGID` got past that operation but failed while creating a data subdirectory after ownership changed. Adding `DAC_OVERRIDE` as well allowed `/ping` and `SELECT version()` to succeed.

This proves one workable capability set for this image in local Docker. It does not prove the minimum set for all images, a production security boundary, Kubernetes volume behavior, or deployment success on hosted Bex. The unchanged legacy image has not yet passed the full persistence and isolation suite below.

The distinct modern-image experiment used ClickHouse 26.9.2.8 with a custom direct-server entrypoint and configuration-based authentication. It passed startup, authenticated native/peer HTTP queries, invalid-credential rejection, and persistence across replacement with no capabilities. That alternative changes customer source, so it does not satisfy this ADR's outcome.

## Proposed decisions

### D1 — Normalize supported historical Blueprint aliases in the shared compiler

Add an explicit, evidence-backed alias table before canonical schema validation. Start with service `env` mapping to `runtime`, across root, project-environment, and ungrouped service declarations.

- An alias-only declaration receives the same type, enum, capability, and runtime compatibility checks as the canonical spelling.
- Keep original source locations and field-presence information so errors point to the authored `env` value. Never change the source file or relax unrelated unknown-field checks.
- If both spellings have the same valid value, normalize once. Conflicting values fail before mutation. Conflict rejection is a proposed Bex rule until authoritative Render evidence establishes precedence; do not describe an inferred precedence as parity.
- Validation, preview, plan, apply, Git sync, REST, GraphQL, and MCP use the same normalization. There is no filename-dependent legacy mode.
- Include aliases in the reviewed registry/provenance inputs and conformance fixtures. Keep the upstream schema snapshot byte-identical; represent historical compatibility in the local overlay.
- Regenerate the CMS capability snapshot and teach its browser checker the same rules after the backend contract lands. A green browser result still does not replace workspace validation.

This repair can ship independently under ADR049. It must not be advertised as successful ClickHouse deployment while startup and routing still fail.

### D2 — Introduce a versioned policy for ordinary image initialization

Propose two centrally defined execution policies, with final API field names determined during implementation:

| Policy | Capabilities | Selection |
| --- | --- | --- |
| Existing strict policy | Drop all; add none. | Existing services and direct App resources retain this behavior unless explicitly migrated. |
| Image compatibility v1 | Drop all; add only `CHOWN`, `DAC_OVERRIDE`, `SETUID`, `SETGID`. | Proposed default for newly created Docker/image application services after the rollout gates pass. |

Persist the resolved policy in authoritative service state and the projected App. Do not derive it afresh from a global flag on each reconciliation: an upgrade or Blueprint resync must not silently change an existing workload's privileges. New Blueprint and dashboard/API services use the same creation rule. The Blueprint example needs no extra field.

The image keeps its own entrypoint and user behavior. The platform does not recognize ClickHouse by name, rewrite its script, force UID 101 on unrelated images, or assume an arbitrary customer process relinquishes capabilities after initialization. The allowed capability set remains the policy for the whole container lifetime.

The four capabilities support changing file ownership, accessing files during root initialization, and switching process users/groups. They are part of Docker's [documented default capability set](https://docs.docker.com/engine/containers/run/#runtime-privilege-and-linux-capabilities) and permitted by Kubernetes [Baseline Pod Security](https://kubernetes.io/docs/concepts/security/pod-security-standards/#baseline). Neither fact establishes that the policy has the same attack surface as dropping all capabilities.

Retain `allowPrivilegeEscalation: false`, `RuntimeDefault` seccomp, disabled ServiceAccount-token mounting, resource quotas, and the existing namespace/Cilium isolation. Do not introduce privileged containers, host mounts, host networking/PID/IPC, extra devices, `SYS_ADMIN`, or tenant-selected arbitrary capability lists. This proposal does not add low-port binding privileges.

Review every path that executes the same service image, including pre-deploy and scheduled/one-off execution where supported. Each path must deliberately inherit the service policy or reject the operation; it must not acquire a different implicit policy. Platform containers, managed datastores, builds, and agent sandboxes retain their separate contracts.

Security review must assess the increased capability exposure on the actual hosting runtime and mounts. A passing startup probe is insufficient. If that review rejects the policy on shared hosting nodes, choose a stronger execution boundary before rollout; do not silently widen the allowlist.

### D3 — Resolve private TCP ports from the exact deployed image

Implement a bounded first phase using OCI image metadata. This solves the unchanged ClickHouse fixture without requiring a privileged node agent or scanning customer networks. It is not full Render discovery of arbitrary undeclared listeners.

For a new Docker/image private service in automatic port mode:

1. Resolve the final image digest after build/pull authorization. Read its declared TCP ports using the same authorized image identity. Do not inspect a mutable tag and later run a different digest.
2. Validate and sort the ports, reject reserved/unsupported values, and enforce a bounded port count. Adopt Render's documented 75-port ceiling for this phase; the exact platform-reserved set is explicit and tested.
3. An explicitly configured primary port wins. Otherwise choose 3000 if declared; otherwise choose the lowest declared valid TCP port. With no declarations, retain the current 3000 fallback and diagnose a failed listener clearly. This selection is a Bex policy, not a claim about Render's algorithm.
4. Configure startup/readiness against the selected primary listener. Metadata is not proof that a process has opened the port: keep the deployment unready until its existing TCP or configured HTTP check passes. Do not silently select another port because the primary became unhealthy.
5. Publish the validated private TCP ports under both existing CR-named and slug-named ClusterIP Services, with stable unique port names. ClickHouse receives 8123, 9000, and 9009; the primary is 8123. An individual unused declared port may refuse connections and must not be presented as independently healthy.

Keep configured/default port intent distinct from resolved ports. The current CRD defaults `spec.port` to 3000, so testing for `spec.port == 3000` cannot distinguish an explicit choice from omission. Introduce explicit mode/version state; do not infer intent from that integer. Existing services preserve their current primary port and exposure set until migrated.

Changing a release's port set must not send traffic to replicas that do not support it. Bind resolved metadata to the release/image, gate publication on the serving revision, and account for overlapping old/new replicas. Disk-backed replacement continues to use ADR082's stop/start lifecycle. Inability to resolve authorized metadata is an observable failure, not a successful deployment with guessed addresses.

Out of scope for this first phase: undeclared-port discovery, UDP or arbitrary protocol parity, Render's special web-service port-10000 alias, and changing the public web-service ingress selection contract. Those limitations stay explicit in ADR018. The customer outcome here requires both ClickHouse TCP interfaces, not only its HTTP health check.

### D4 — Addresses and Blueprint references follow the resolved primary port

`internalAddress`, dashboard Service Address, and `fromService` `port`/`hostport` must agree with the serving port. A stable hostname alone does not make `hostname:3000` correct.

Initial Blueprint planning can validate a reference before a Git image has been built, but cannot invent its resolved port. Build independently where possible and defer applying consumers' port-dependent runtime values until resolution succeeds. Surface unresolved dependencies and cycles instead of starting callers with an incorrect default. Existing `host` references keep the same slug identity.

For an upgrade that changes the primary port, retain the active release's address until the serving transition, identify and reconcile managed dependent references, and expose the pending change to operators. User-authored literal URLs remain the user's configuration. Test suspend/resume, rollback, and failed deploys so stale candidate metadata cannot replace active addresses.

### D5 — Prove deployment behavior before publishing a customer guide

Use the pinned upstream repository as a conformance fixture, preserving its Dockerfile and Blueprint bytes. Record its source commit and the image digest actually resolved in each run; the upstream `:21` tag is mutable. A network-enabled integration lane may report upstream drift, while offline schema tests use committed fixtures.

The release gate is a disposable Bex environment running the real API, operator, enforcing CNI, and representative storage. It must demonstrate:

1. Git-context inheritance supplies the omitted repository/branch; validate, preview, and deploy accept the unchanged manifest. A second unchanged sync is a no-op.
2. The exact image starts using the selected execution policy on a fresh volume, and can restart against an already-owned volume.
3. An allowed peer reaches HTTP 8123 and native TCP 9000 through the returned stable hostname. No test substitutes localhost or a direct pod IP for the customer connection path.
4. Insert a sentinel row, replace the workload, and read the same row. Exercise disk permissions, mount-path behavior, SIGTERM, and the existing single-instance restriction.
5. Cross-workspace, isolated-environment, node, cloud-metadata, and platform-service probes remain denied. Public ingress remains absent. Check the actual running pod's security settings as well as projected objects.
6. Legacy services do not change execution policy or exposed ports after upgrade. Explicit port configuration, delayed startup, image-metadata failure, port-set changes, failed rollout, rollback, and dependent references behave as specified.
7. Alias fixtures cover invalid values, duplicate keys, equal/conflicting spellings, nested resources, all adapters, and zero writes on rejection. CMS and backend diagnostics agree.

The unchanged old template's authentication defaults are compatibility evidence only. Separately validate a documented credential configuration and a currently selected ClickHouse release for a production-oriented guide. Do not imply Bex disk archives are database-consistent backups; retain ClickHouse-native backup guidance.

## Alternatives considered

- **Require customers to fork every Docker example.** A verified workaround exists, but moves platform incompatibility into customer maintenance and misses the unchanged-source goal.
- **Fix only `env`, or publish only port 8123.** Both are incomplete: accepted YAML can still crash, and HTTP-only connectivity omits the native interface.
- **Enable privileged containers or restore every Docker capability globally.** Much broader than the observed requirement and silently changes existing services.
- **Force a non-root UID plus `fsGroup`.** Worth supporting separately, but the correct UID and filesystem setup are image-specific and cannot be inferred from this image's root default as a general rule.
- **Use only `EXPOSE` as a health signal.** Incorrect: it is metadata, not a listening process. D3 uses normal live readiness checks.
- **Implement full socket discovery immediately.** Broader parity remains useful, but requires its own observer trust model, protocol policy, startup timing, and resource limits. Do not claim that work is delivered by metadata-based support.

## Delivery and decision gates

1. Fix and test the legacy alias under ADR049, independently of runtime changes.
2. Review this proposed runtime policy and its security consequences; settle the reserved-port list and persisted policy/mode schema before implementation.
3. Implement image metadata resolution, private Service projection, readiness, address/reference propagation, and lifecycle tests. Keep compatibility disabled for hosted creation until integration and isolation gates pass.
4. Enable the new creation policy with explicit versioned state. Migrate existing services only through a deliberate operation; roll back creation defaults without silently breaking already-created compatible services.
5. Update ADR018, ADR022, ADR041, the capability registry and CMS snapshot, plus their existing guards to assert both policies. Add the customer ClickHouse guide and translations after the actual Bex deployment passes.

This ADR records the proposed product tradeoff. The four-capability local test is evidence for review, not authorization to change production isolation or a claim that hosted ClickHouse already works.

## Implementation checkpoint — 2026-09-30

The working implementation is based on `f064e0a225269d324f18bcea9dcad78358faff64`, verified against GitHub `main` during implementation. It adds the reviewed `env` alias overlay, the persisted `containerPolicy` and `portMode` fields, verified image metadata lookup for Git-built private services, and private Service ports tied to a ready release. Migration `0135_app_image_compatibility` preserves the strict/configured defaults for existing rows. A create-time disk is now saved with the service and first deploy in one transaction; its initial metering period uses the same helper as explicit disk attachment.

An explicit primary port uses `image-configured-v1`, including an explicit 3000. Port updates persist both the number and mode; image metadata still supplies the other private TCP listeners. The API's numeric port and internal address retain the active release's primary during replacement. A failed Service write cannot advance the recorded serving network, and both stable hostnames must converge before that status changes.

The 2026-10-01 follow-up shares the reserved-port policy between API validation, image metadata parsing, and runtime projection. New service creation (including dry-run), direct port updates, and multi-field settings updates reject reserved image ports before writing. Regression tests assert that rejected requests leave the service and durable port unchanged, while legacy configured-port services retain their existing contract. The tests reproduced acceptance of all three reserved ports before the fix.

Canceling a replacement restores the image and network metadata bound to the last serving revision. A failed candidate may already occupy `status.image`, so that field alone is insufficient for recovery. Regression tests cover restoration with and without a recorded pod template, readiness after restoration, both stable hostnames, and retention of the saved configuration and candidate artifact for a later deploy.

`BEX_IMAGE_COMPATIBILITY_WORKSPACES` enables the new creation defaults in bex-api for the listed creating workspaces (`*` admits all). It is off when unset, and an entry that is not a workspace id refuses startup. The gate is a rollout control, not evidence that the release gates passed. Removing a workspace never migrates its existing services. Direct prebuilt-image port discovery, managed `fromService` port dependency propagation, and CMS checker/snapshot parity are still pending. Production therefore admits only the owner-approved `bex` workspace (`tea-d98210cbbpdc73dcrkvg`) to run the D5 hosted gate; do not admit customer workspaces while port-dependent Blueprint references can resolve to the legacy 3000 default.

Passing local checks cover the unchanged upstream Blueprint through create and an unchanged second apply, durable policy/disk reconstruction, deployment/cron/pre-deploy capability projection, metadata digest and size limits, and the serving-port transition. The full shared-types test suite, backend/operator package compilation, and the backend/operator/types module lint checks also pass. Markdown formatting and `git diff --check` pass.

Real Postgres tests cover policy/disk round trips and atomic rollback when disk creation fails. On 2026-10-01 the full backend suite passed against a pristine Postgres 17 (the pinned CI image), after fixing the new fixtures' invalid tenant plan; `0135` down/up, its refusal while compatible services exist, and its CHECK constraints were exercised directly. The full operator `make test` (codegen plus envtest controller suite) passed with no generated drift. Hosted deployment, persistence and isolation gates remain open.
