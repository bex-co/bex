# ADR089 — Deploy Render Docker examples without changing their source

**Status:** Accepted, 2026-10-20, with D2 amended to the `render-v1` policy. This follows the user decisions of 2026-10-17 ("Render parity outranks the earlier security hardening") and 2026-10-20 (Render parity is the top product principle, [ADR008 § Render parity first](ADR008-vision.md#render-parity-first)).

- Proposed 2026-09-29.
- The port slice (D3/D4) has been the creation default in every workspace since w6/m148 and passed its hosted gate (§ Port slice decisions).
- The capability policy `render-v1` (D2) is implemented (`67aa2bf35`). It is the creation default in every workspace: production widened `BEX_IMAGE_COMPATIBILITY_WORKSPACES` to `*` on 2026-10-21 (w6/m149 t009), and unset means `*` (w6/090).

**Scope:** Import and run the unchanged `render-examples/clickhouse` repository, with a reusable contract for legacy Blueprint aliases, container initialization, and multiple private TCP ports. This does not establish complete Render runtime parity.

**Related decisions:** [ADR049](ADR049-render-yaml-parity.md) owns Blueprint semantics; [ADR022](ADR022-tenant-isolation.md) owns the retained container hardening policy; [ADR043](ADR043-tenant-namespace-isolation.md) owns workspace isolation; [ADR041](ADR041-service-addresses.md) owns addresses and the current single-port limitation; [ADR082](ADR082-persistent-disks.md) owns persistent storage. Acceptance amends ADR022's unconditional capability drop for new services (the legacy contract stays in force for existing `strict-v1`/`image-v1` rows) and ADR041's single-port restriction.

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

### D2 — A versioned execution policy that matches Render's measured privileges

**Amended 2026-10-20 (w6/m149).** The original proposal below the policy table (a four-capability `image-v1` allowlist) is superseded for new services. It remains the valid contract for rows that already carry `image-v1`.

**Parity criterion.** Every image that starts on Render starts on bex unchanged, in every workspace. The measured evidence is [container-privileges.md](render-artifacts/container-privileges.md).

| Policy | Capabilities | Selection |
| --- | --- | --- |
| `strict-v1` (legacy) | Drop all; add none. | Existing services and direct App resources keep this policy unless explicitly migrated. |
| `image-v1` (legacy) | Drop all; add only `CHOWN`, `DAC_OVERRIDE`, `SETUID`, `SETGID`. | Superseded for new services on 2026-10-20. Existing rows keep it unless explicitly migrated. |
| `render-v1` | Drop all; add exactly `CHOWN`, `DAC_OVERRIDE`, `FOWNER`, `SETGID`, `SETUID`, `SYS_CHROOT` (Render's measured set). | Creation default for new Docker/image/Dockerfile services. Static sites and native runtimes are out of scope. |

- **Capabilities.** `render-v1` is `image-v1` plus `FOWNER` and `SYS_CHROOT`. Docker-default capabilities that Render lacks are **not** added: `NET_BIND_SERVICE`, `KILL`, `SETPCAP`, `SETFCAP`, `FSETID`, `MKNOD`, `AUDIT_WRITE` and `NET_RAW`. Leaving each one out matches Render.
- **`allowPrivilegeEscalation: false`** is kept. It matches Render's measured `NoNewPrivs=1`.
- **Low ports** are already bindable without `NET_BIND_SERVICE`: containerd 2.x sets `ip_unprivileged_port_start=0` in every pod, as Render does (w6/m149 t006).
- **Seccomp: RuntimeDefault, a named divergence.** Render applies no filter, so `io_uring` and unprivileged user namespaces work there; RuntimeDefault blocks both. Relaxation is tracked by w6/085. Under parity first its default direction is a Localhost profile (RuntimeDefault plus `io_uring` and `unshare(CLONE_NEWUSER)`), shipped with its own security review. It does not gate `render-v1`. **Update 2026-10-21 (w6/m150):** `render-v1` containers now run bex's Localhost profile `profiles/bex-render-v1.json` instead of RuntimeDefault. It is containerd's RuntimeDefault for the render-v1 capability set plus unmasked `clone`, the user-namespace mount/namespace syscalls (`unshare`, `setns`, `mount`, `umount2`, `pivot_root`, the new mount API, `sethostname`, `setdomainname`) and `io_uring_*`; `bpf`, `perf_event_open`, `keyctl`, `userfaultfd`, `kexec_*` and module loading stay blocked and `clone3` stays `ENOSYS`. Generated by `scripts/seccomp-render-v1/generate.sh`, installed on every node by `deploy/gitops/base/seccomp-profiles.yaml`, and admitted only for render-v1 workloads. Measured on the local cluster: a non-root `unshare -Ur` and `io_uring_setup` succeed under it and fail under RuntimeDefault. Residual risk, accepted under ADR008 § Render parity first: unprivileged user namespaces give tenant code more kernel surface on shared nodes (still a filter, unlike Render's none); `strict-v1`/`image-v1` keep RuntimeDefault.
- **Pod user namespaces are not part of `render-v1`.** Render runs none, so `hostUsers: false` would be bex-only defense in depth, not parity. The t002 spike was a go on ext4 and fails closed on a filesystem that cannot be idmapped. Adoption is tracked by w6/086, gated on proving idmapped mounts on Hetzner `hcloud-volumes`. **Update 2026-10-21 (w6/m151):** adopted. `render-v1` runtime pods (Deployment, CronJob, manual run; pre-deploy pods already did) run with `hostUsers: false` in every workspace (`BEX_RENDER_V1_USER_NAMESPACES=*`). The `bex` canary proved Hetzner `hcloud-volumes` accept idmapped mounts: disk-backed ClickHouse was Live with data intact across a restart, and the pod read `uid_map 0 2104885248 65536`. A container escape now lands as an unprivileged host UID. Rollback removes the variable; on-disk UIDs are unshifted, so neither direction needs a data migration.
- **Lifecycle.**
  - The creation default is `render-v1` for workspaces in `BEX_IMAGE_COMPATIBILITY_WORKSPACES`. Production widened it to `*` (every workspace) on 2026-10-21 after the `bex` canary (w6/m149 t009).
  - Existing `strict-v1`/`image-v1` rows never change on upgrade, Blueprint sync or redeploy.
  - Moving one existing service is an explicit operator-run migration (w6/m149 t008).
  - Rollback reverts only the creation default; it never migrates services already created.

The rest of this section is the original 2026-09-29 proposal. It still holds except where the amendment above replaces it.

Persist the resolved policy in authoritative service state and the projected App. Do not derive it afresh from a global flag on each reconciliation: an upgrade or Blueprint resync must not silently change an existing workload's privileges. New Blueprint and dashboard/API services use the same creation rule. The Blueprint example needs no extra field.

The image keeps its own entrypoint and user behavior. The platform does not recognize ClickHouse by name, rewrite its script, force UID 101 on unrelated images, or assume an arbitrary customer process relinquishes capabilities after initialization. The allowed capability set remains the policy for the whole container lifetime.

The four capabilities support changing file ownership, accessing files during root initialization, and switching process users/groups. They are part of Docker's [documented default capability set](https://docs.docker.com/engine/containers/run/#runtime-privilege-and-linux-capabilities) and permitted by Kubernetes [Baseline Pod Security](https://kubernetes.io/docs/concepts/security/pod-security-standards/#baseline). Neither fact establishes that the policy has the same attack surface as dropping all capabilities.

Retain `allowPrivilegeEscalation: false`, `RuntimeDefault` seccomp, disabled ServiceAccount-token mounting, resource quotas, and the existing namespace/Cilium isolation. Do not introduce privileged containers, host mounts, host networking/PID/IPC, extra devices, `SYS_ADMIN`, or tenant-selected arbitrary capability lists. This proposal does not add low-port binding privileges.

Review every path that executes the same service image, including pre-deploy and scheduled/one-off execution where supported. Each path must deliberately inherit the service policy or reject the operation; it must not acquire a different implicit policy. Platform containers, managed datastores, builds, and agent sandboxes retain their separate contracts.

Security review must assess the increased capability exposure on the actual hosting runtime and mounts. A passing startup probe is insufficient. If that review rejects the policy on shared hosting nodes, choose a stronger execution boundary before rollout; do not silently widen the allowlist. **Superseded 2026-10-20:** under Render parity first the allowlist widens to Render's measured set; a stronger boundary (w6/085, w6/086) is a follow-up, not a precondition.

### D3 — Resolve private TCP ports from the exact deployed image

Implement a bounded first phase using OCI image metadata. This solves the unchanged ClickHouse fixture without requiring a privileged node agent or scanning customer networks. It is not full Render discovery of arbitrary undeclared listeners.

For a new Docker/image private service in automatic port mode:

1. Resolve the final image digest after build/pull authorization. Read its declared TCP ports using the same authorized image identity. Do not inspect a mutable tag and later run a different digest.
2. Validate and sort the ports, reject reserved/unsupported values, and enforce a bounded port count. Adopt Render's documented 75-port ceiling for this phase; the exact platform-reserved set is explicit and tested.
3. An explicitly configured primary port wins. Otherwise choose 3000 if declared; otherwise choose the lowest declared valid TCP port. With no declarations, retain the current 3000 fallback and diagnose a failed listener clearly. This selection is a Bex policy, not a claim about Render's algorithm.
4. Configure startup/readiness against the selected primary listener. Metadata is not proof that a process has opened the port: keep the deployment unready until its existing TCP or configured HTTP check passes. Do not silently select another port because the primary became unhealthy.
5. Publish the validated private TCP ports under both existing CR-named and slug-named ClusterIP Services, with stable unique port names. ClickHouse receives 8123, 9000, and 9009; the primary is 8123. An individual unused declared port may refuse connections and must not be presented as independently healthy.

Keep configured/default port intent distinct from resolved ports. The current CRD defaults `spec.port` to 3000, so testing for `spec.port == 3000` cannot distinguish an explicit choice from omission. Introduce explicit mode/version state; do not infer intent from that integer. Existing services preserve their current primary port and exposure set until migrated.

Changing a release's port set must not send traffic to replicas that do not support it. Bind resolved metadata to the release/image, gate publication on the serving revision, and account for overlapping old/new replicas. **Revision pin only for a port change (w4/m243 t012, 2026-10-28):** the stable Services pin the serving revision only while a candidate's ports differ from the serving network's. A same-port rollout (a deploy, an env save, a restart) leaves them selecting every runtime pod. Old and new pods serve the same listener, and a pin there left the Services with no ready endpoint once the Deployment drained the old ReplicaSet, until promotion moved the selector (Traefik's raw `503 no available server` for 1–5 s on every deploy). **A port change routes early (w4/428, 2026-10-28):** while a candidate's ports differ, the Deployment holds its old pods for `minReadySeconds` (15 s) after a new pod is Ready, and the operator promotes the network at that first Ready pod, moving both Services and the Ingress to it. Then the Services pin the new revision until the rollout completes and no old pod lacking the new listener remains. `status.priorServingNetwork` records the network that was replaced, and a rollout that fails after routing early restores it. **The route's port never moves with the Services' (w4/430, 2026-10-28):** the first Service write at early promotion already pins the new revision, and while the old pods remain, the Services keep the replaced primary port, when the new image no longer declares it, as an alias of the new listener. The Ingress keeps naming that aliased port until the rollout completes, then moves to the new primary, and the next Service write drops the alias. Before this, Traefik could see a Service without the port its route named, drop the route, and send the host to the activator's `service not ready`. The first write could also select old pods on a port they lack (a `502`). Disk-backed replacement continues to use ADR082's stop/start lifecycle. Inability to resolve authorized metadata is an observable failure, not a successful deployment with guessed addresses.

Out of scope for this first phase (web-service runtime port detection shipped separately in w8/177, ADR018): undeclared-port discovery for private services, UDP or arbitrary protocol parity, Render's special web-service port-10000 alias, and changing the public web-service ingress selection contract. Those limitations stay explicit in ADR018. The customer outcome here requires both ClickHouse TCP interfaces, not only its HTTP health check.

### D4 — Addresses and Blueprint references follow the resolved primary port

`internalAddress`, dashboard Service Address, and `fromService` `port`/`hostport` must agree with the serving port. A stable hostname alone does not make `hostname:3000` correct.

Initial Blueprint planning can validate a reference before a Git image has been built, but cannot invent its resolved port. Build independently where possible and defer applying consumers' port-dependent runtime values until resolution succeeds. Surface unresolved dependencies and cycles instead of starting callers with an incorrect default. Existing `host` references keep the same slug identity.

For an upgrade that changes the primary port, retain the active release's address until the serving transition, identify and reconcile managed dependent references, and expose the pending change to operators. User-authored literal URLs remain the user's configuration. Test suspend/resume, rollback, and failed deploys so stale candidate metadata cannot replace active addresses.

### D5 — Prove deployment behavior before publishing a customer guide

Use the pinned upstream repository as a conformance fixture, preserving its Dockerfile and Blueprint bytes. Record its source commit and the image digest actually resolved in each run; the upstream `:21` tag is mutable. A network-enabled integration lane may report upstream drift, while offline schema tests use committed fixtures.

**Amended 2026-10-20:** the `BEX_IMAGE_COMPATIBILITY_WORKSPACES` allowlist is a rollout canary, not a permanent boundary. After the `bex` canary passes, w6/m149 t009 widens it to `*` (every workspace), per [ADR008 § Render parity first](ADR008-vision.md#render-parity-first).

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
- **Enable privileged containers or restore every Docker capability globally.** Much broader than the observed requirement and silently changes existing services. **Amended 2026-10-20:** parity first supersedes the minimal-allowlist posture, so new services get Render's measured six (D2). Docker-default capabilities beyond Render's set, privileged mode and changing existing services remain rejected.
- **Force a non-root UID plus `fsGroup`.** Worth supporting separately, but the correct UID and filesystem setup are image-specific and cannot be inferred from this image's root default as a general rule.
- **Use only `EXPOSE` as a health signal.** Incorrect: it is metadata, not a listening process. D3 uses normal live readiness checks.
- **Implement full socket discovery immediately.** Broader parity remains useful, but requires its own observer trust model, protocol policy, startup timing, and resource limits. Do not claim that work is delivered by metadata-based support.

## Delivery and decision gates

1. Fix and test the legacy alias under ADR049, independently of runtime changes.
2. Review this proposed runtime policy and its security consequences; settle the reserved-port list and persisted policy/mode schema before implementation.
3. Implement image metadata resolution, private Service projection, readiness, address/reference propagation, and lifecycle tests. Keep compatibility disabled for hosted creation until integration and isolation gates pass.
4. Enable the new creation policy with explicit versioned state. Migrate existing services only through a deliberate operation; roll back creation defaults without silently breaking already-created compatible services.
5. Update ADR018, ADR022, ADR041, the capability registry and CMS snapshot, plus their existing guards to assert both policies. Add the customer ClickHouse guide and translations after the actual Bex deployment passes.

This ADR records the proposed product tradeoff. The four-capability local test is evidence for review, not authorization to change production isolation or a claim that hosted ClickHouse already works. **Update 2026-10-20:** the authorization is now the user decisions cited in the status line, with D2 amended to `render-v1`.

## Implementation checkpoint — 2026-09-30

The working implementation is based on `f064e0a225269d324f18bcea9dcad78358faff64`, verified against GitHub `main` during implementation. It adds the reviewed `env` alias overlay, the persisted `containerPolicy` and `portMode` fields, verified image metadata lookup for Git-built private services, and private Service ports tied to a ready release. Migration `0135_app_image_compatibility` preserves the strict/configured defaults for existing rows. A create-time disk is now saved with the service and first deploy in one transaction; its initial metering period uses the same helper as explicit disk attachment.

An explicit primary port uses `image-configured-v1`, including an explicit 3000. Port updates persist both the number and mode; image metadata still supplies the other private TCP listeners. The API's numeric port and internal address retain the active release's primary during replacement. A failed Service write cannot advance the recorded serving network, and both stable hostnames must converge before that status changes.

The 2026-10-01 follow-up shares the reserved-port policy between API validation, image metadata parsing, and runtime projection. New service creation (including dry-run), direct port updates, and multi-field settings updates reject reserved image ports before writing. Regression tests assert that rejected requests leave the service and durable port unchanged, while legacy configured-port services retain their existing contract. The tests reproduced acceptance of all three reserved ports before the fix.

Canceling a replacement restores the image and network metadata bound to the last serving revision. A failed candidate may already occupy `status.image`, so that field alone is insufficient for recovery. Regression tests cover restoration with and without a recorded pod template, readiness after restoration, both stable hostnames, and retention of the saved configuration and candidate artifact for a later deploy.

`BEX_IMAGE_COMPATIBILITY_WORKSPACES` enables the new creation defaults in bex-api for the listed creating workspaces (`*` admits all). It is off when unset, and an entry that is not a workspace id refuses startup. **Amended 2026-10-21 (w6/090):** unset now means `*`, so self-hosted bex runs unchanged root-init images as production does; `off` restores the strict policy for every new service. The gate is a rollout control, not evidence that the release gates passed. Removing a workspace never migrates its existing services. Direct prebuilt-image port discovery and managed `fromService` port dependency propagation shipped in w6/m148, and production admits every workspace since 2026-10-21 (w6/m149 t009). CMS checker/snapshot parity is still pending (w6/081).

Passing local checks cover the unchanged upstream Blueprint through create and an unchanged second apply, durable policy/disk reconstruction, deployment/cron/pre-deploy capability projection, metadata digest and size limits, and the serving-port transition. The full shared-types test suite, backend/operator package compilation, and the backend/operator/types module lint checks also pass. Markdown formatting and `git diff --check` pass.

Real Postgres tests cover policy/disk round trips and atomic rollback when disk creation fails. On 2026-10-01 the full backend suite passed against a pristine Postgres 17 (the pinned CI image), after fixing the new fixtures' invalid tenant plan; `0135` down/up, its refusal while compatible services exist, and its CHECK constraints were exercised directly. The full operator `make test` (codegen plus envtest controller suite) passed with no generated drift. Hosted deployment, persistence and isolation gates remain open.

**Web services (w4/m243, 2026-10-25):** the same image-port rule now applies to new Docker/image web services in every workspace (`web-image-v1`, or `web-image-configured-v1` with an explicit port): the operator resolves the image's TCP `EXPOSE` set — digest-verified from the service's own built repository, or anonymously (no platform credential) for a prebuilt public image — and the web `Service`/`Ingress` serve the primary port (explicit/`PORT` env, else declared 3000, else lowest declared). Existing web services carry no mode and are untouched.

## Port slice decisions — 2026-10-19 (w6/m148)

These decisions split D3/D4 (ports and references) from D2 (capabilities). They ship in w6/m148. D2's capability policy for customer workspaces is w6/m149.

**Status, 2026-10-20:** D3 and D4 shipped in w6/m148 (`94533d795`) as the default in every workspace, and were verified live on production on 2026-10-20 ([evidence](../.pm/w6/done/m148/evidence/README.md)). The live checks covered a multi-port private service in a customer workspace under the strict policy, agreement between REST, GraphQL, MCP and the dashboard, `fromService` propagation on first discovery and on `setPort`, cross-workspace denial, and the unchanged ClickHouse Blueprint in `bex`. Still open from the 2026-09-30 checkpoint: port discovery for prebuilt-image and native-runtime private services (out of scope), and the CMS checker snapshot (w6/081). The remaining Render divergences are named in [ADR018](ADR018-render-parity.md)'s Open ports row and in [ADR049](ADR049-render-yaml-parity.md) § `fromService` port references follow the real port.

1. **The gate is split.**
   - Image-derived ports (`portMode` `image-v1`, or `image-configured-v1` with an explicit port) become the creation default for new Git-built Docker private services in **every** workspace.
   - `containerPolicy` `image-v1` stays behind `BEX_IMAGE_COMPATIBILITY_WORKSPACES`.
   - Port discovery needs no security exception: it reads the service's own digest-verified image config and publishes ClusterIP ports that are already reachable under the workspace network policy.
2. **Rollback switch.** `BEX_PRIVATE_IMAGE_PORTS` is `on` (the default) or `off`. `off` restores `configured-v1` creation. Any other value refuses bex-api startup. Neither value ever rewrites an existing row.
3. **Primary-port rule.**

   - An explicit port wins.
   - Otherwise 3000, if the image declares it.
   - Otherwise the lowest declared TCP port.
   - With no declarations, 3000.

   Ports below 1024 are **not** excluded. containerd 2.x's CRI default sets `ip_unprivileged_port_start=0` inside every pod, and a `strict-v1` root process binds `:80` on production (measured 2026-10-19, [container-privileges.md](render-artifacts/container-privileges.md)). w6/m149/t006 removed the old 1024 floor from every surface.

4. **References: backend-resolved literals, propagated by bex-api.**
   - A Blueprint `fromService` `port`/`hostport` stays a literal environment value, so it remains visible in the Environment view (w4/m120) and goes through the normal release machinery.
   - At apply, its value comes from the target's current serving port (`App.ServicePort()`, which follows `ServingNetwork` during a replacement), never from a parse-time default.
   - For a target whose port is not yet known (an image-port service before its first metadata read), the variable is **omitted**: never written as 3000.
   - The consumer records each such reference in the `app.bex.co/service-refs` annotation.
   - A bex-api loop compares each recorded reference with its target's serving port. On a difference it rewrites the literal through the tracked patch path, so the change opens a `config_change` deploy with normal history. That covers a later `SetPort`, a promotion that moves the primary, and the first discovery.
   - Suspended consumers are left until they resume.
   - An operator-resolved `valueFrom` reference was rejected: release identity is a pure function of the spec and is tied to backend-opened deploys, so an operator-only re-roll would bypass deploy history.
5. **Cycles need no special rule.** A service's own port never depends on its consumers' environment, so mutual `hostport` references converge independently.
6. **Read surfaces.** REST and MCP `serviceDetails.openPorts` (Render's required `serverPort[]`) and GraphQL `Service.openPorts` report the serving set:
   - the effective port for configured web and private services;
   - `[]` while discovery is pending;
   - absent for workers, cron jobs and static sites.

## Privilege slice decisions — 2026-10-19 (w6/m149)

**User decision, 2026-10-17:** Render parity outranks the earlier drop-ALL hardening. These decisions turn that into D2's contract. The evidence is a live probe of Render and of bex `strict-v1`/`image-v1` ([container-privileges.md](render-artifacts/container-privileges.md)).

1. **Parity criterion:** every image that starts on Render starts on bex unchanged. bex may be more permissive where Render is unmeasured, but it is never stricter on a measured axis.
2. **Policy `render-v1`.** It is versioned and persisted per service, like `image-v1`.
   - It drops `ALL` and adds exactly Render's measured set: `CHOWN, DAC_OVERRIDE, FOWNER, SETGID, SETUID, SYS_CHROOT`. That is `image-v1` plus `FOWNER` (an entrypoint that chmods after chowning fails without it, as the probe itself did) and `SYS_CHROOT`.
   - Docker-default capabilities that Render lacks are **not** added: `NET_BIND_SERVICE`, `KILL`, `SETPCAP`, `SETFCAP`, `FSETID`, `MKNOD`, `AUDIT_WRITE` and `NET_RAW`.
   - Ports below 1024 need none of them: containerd 2.x enables unprivileged ports in every pod (w6/m149 t006).
3. **Kept as is:** `allowPrivilegeEscalation: false` (Render measured `NoNewPrivs=1`), the AppArmor default (the same profile on both), and no host namespaces, devices or privileged mode.
4. **Seccomp: decided 2026-10-20, RuntimeDefault.** Render applies no filter, so `io_uring` and unprivileged user namespaces work there. bex's RuntimeDefault blocks both, and this is a named divergence ([ADR018](ADR018-render-parity.md)). Relaxation is tracked by w6/085; its default direction is a Localhost profile (RuntimeDefault plus `io_uring` and `unshare(CLONE_NEWUSER)`), shipped with its own security review. **Update 2026-10-21 (w6/m150):** `render-v1` containers now run bex's Localhost profile `profiles/bex-render-v1.json` instead of RuntimeDefault. It is containerd's RuntimeDefault for the render-v1 capability set plus unmasked `clone`, the user-namespace mount/namespace syscalls (`unshare`, `setns`, `mount`, `umount2`, `pivot_root`, the new mount API, `sethostname`, `setdomainname`) and `io_uring_*`; `bpf`, `perf_event_open`, `keyctl`, `userfaultfd`, `kexec_*` and module loading stay blocked and `clone3` stays `ENOSYS`. Generated by `scripts/seccomp-render-v1/generate.sh`, installed on every node by `deploy/gitops/base/seccomp-profiles.yaml`, and admitted only for render-v1 workloads. Measured on the local cluster: a non-root `unshare -Ur` and `io_uring_setup` succeed under it and fail under RuntimeDefault. Residual risk, accepted under ADR008 § Render parity first: unprivileged user namespaces give tenant code more kernel surface on shared nodes (still a filter, unlike Render's none); `strict-v1`/`image-v1` keep RuntimeDefault.
5. **Pod user namespaces: not part of `render-v1` (2026-10-20).** Render runs none, so `hostUsers: false` is bex-only defense in depth, not parity. The t002 spike was a go on ext4 and fails closed on a filesystem that cannot be idmapped. Adoption is tracked by w6/086, gated on Hetzner `hcloud-volumes` idmap proof. It is not a gate for `render-v1` or for the t009 widen. **Update 2026-10-21 (w6/m151):** adopted. `render-v1` runtime pods (Deployment, CronJob, manual run; pre-deploy pods already did) run with `hostUsers: false` in every workspace (`BEX_RENDER_V1_USER_NAMESPACES=*`). The `bex` canary proved Hetzner `hcloud-volumes` accept idmapped mounts: disk-backed ClickHouse was Live with data intact across a restart, and the pod read `uid_map 0 2104885248 65536`. A container escape now lands as an unprivileged host UID. Rollback removes the variable; on-disk UIDs are unshifted, so neither direction needs a data migration.
6. **Scope and lifecycle.**
   - New Docker/image/Dockerfile services (not static sites, not native runtimes) get `render-v1`. The creation default applies to workspaces in `BEX_IMAGE_COMPATIBILITY_WORKSPACES`, which is `*` in production since 2026-10-21 (w6/m149 t009) and when unset (w6/090).
   - `strict-v1` and `image-v1` rows never change on upgrade, Blueprint sync or redeploy.
   - Moving an existing service is an explicit operator-run migration of one service (w6/m149 t008).
   - Rollback reverts the creation default only.
