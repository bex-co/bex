# w22 · m401 — A registry outage pages within minutes: Zot's availability and the tenant image-pull path get alerts, panels and a drill

**Worker:** worker22 **Goal:** when the platform registry (Zot, `bex-registry`) is down or a tenant pod cannot pull its image from it, an alert fires within minutes instead of a tenant reporting it. Both alerts have a panel, a row in the ADR088 §6 coverage table, promtool tests that fail when the rule is broken, and a recorded drill. **Status:** todo

## Tasks (in order)

| id   | title                                                                                                              | est | depends_on |
| ---- | ------------------------------------------------------------------------------------------------------------------ | --- | ---------- |
| t001 | `ZotRegistryNotReady`: the `zot` StatefulSet has fewer ready replicas than desired for 5 minutes, with promtool tests | 45m | —          |
| t002 | `TenantRegistryImagePullFailing`: a tenant pod whose image comes from the platform registry sits in `ImagePullBackOff`/`ErrImagePull`, with promtool tests | 45m | —          |
| t003 | Panels for both alerts, the ADR088 §6 "registry pull path" row, `obs-coverage-check.sh` green, the stale "zot is empty" comment corrected | 40m | t001, t002 |
| t004 | Drill on the local cluster: Zot at 0 replicas, a restarted tenant pod fails to pull; record what kubelet and the metrics showed in `docs/drills/` | 45m | t003       |
| t005 | Simplify                                                                                                           | 15m | t004       |
| t006 | Test coverage                                                                                                      | 20m | t004       |
| t007 | Closeout                                                                                                           | 10m | t005, t006 |

## Definition of done

- `ZotRegistryNotReady` fires when `kube_statefulset_status_replicas_ready` for `bex-registry/zot` stays below `kube_statefulset_replicas` for the rule's `for` window, does not fire one minute short of it, and resolves when the replica returns. promtool cases cover all three, and a StatefulSet of another name or namespace does not match.
- `TenantRegistryImagePullFailing` fires for a tenant-namespace pod container whose `image` is served by the platform registry and whose waiting reason is `ImagePullBackOff` or `ErrImagePull` for its `for` window. It does not fire for an image on another registry (a tenant's typo is not a platform defect), for a shorter wait, or for a pod in a platform namespace. promtool cases cover each.
- Both alerts have at least one panel that draws a non-context backing series, and `scripts/obs-coverage-check.sh` passes. `docs/ADR088-platform-observability-ui.md` §6 has a "registry pull path" row naming both alerts and their panels. The comment above `StrandedNodeLocalImages` no longer claims "zot is empty".
- `scripts/gitops-validate.sh`'s promtool step (`promtool check rules` and every `*_test.yml`) passes.
- `docs/drills/<date>-registry-outage.md` records the drill: what was scaled, how long a restarted tenant pod took to reach `ImagePullBackOff`, the real `image` string and waiting-reason labels the rules' matchers had to match, and whether the alerts fired live or only against fixtures built from those real labels.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w22` (2026-11-14), filed by `/pmx all to w22`. Evidence read at `65ee16b61` and re-checked at `d99de3415`:
  - `deploy/gitops/base/zot.yaml` deploys Zot from the pinned chart into `bex-registry`; the chart's workload is the StatefulSet `zot` (the manifest itself carries an `ignoreDifferences` entry for `StatefulSet/zot`).
  - `deploy/gitops/base/rules/alerting_rules.yml`: `PlatformDeploymentNotReady` (line ~83) reads only `kube_deployment_*`, but its namespace regex lists `bex-registry`, so it looks like it covers Zot and never fires for it. The only StatefulSet rule in the file is `KratosCourierNotReady`. Zot's own alerts are `ZotRegistryFillingUp`, `ZotRegistryAlmostFull`, `ZotGarbageCollectionFailure` and `ZotGarbageCollectionTelemetryMissing`: disk and GC, not availability. `PlatformPodCrashLooping` needs ten minutes of CrashLoopBackOff, so it misses Pending, a failed volume mount and NotReady.
  - `lego/operator/internal/controller/app_controller.go` (`tenantImagePullPolicy = corev1.PullAlways`, ADR022 § Node image cache, w4/426, 2026-10-27) makes every tenant container start confirm its manifest with the registry, so Zot is now a run-time dependency of every wake, scale-out and restart of a Git-built service, not only a build-time one.
  - `StrandedNodeLocalImages` (`alerting_rules.yml`, ~line 1640) is scoped to `namespace="default"` and its comment still says "no registry — zot is empty". It cannot see a tenant namespace.
  - ADR022 § Credential activation accepts one Zot restart per new service credential until `w4/m347` removes it, so planned restarts happen today and the `for` window must be chosen with them in mind (t001).
- **Goal linkage:** ADR008 § What's next → Ops hardening; ADR088 §6 requires every tenant-visible surface to have an alert, a probe or a recorded waiver.
- **Expected outcome:** a Zot outage or a blocked tenant pull alerts the operator within minutes, with a panel that shows how bad and since when.
- **Why now:** the `PullAlways` decision (2026-10-27) turned a Zot outage from "new builds fail" into "every Git-built service that wakes, scales or restarts fails". Until `w4/m347` lands, each new service credential also restarts Zot on purpose, so the window exists today.
- **Render parity omitted:** pure platform GitOps alerting. No REST, GraphQL, MCP or tenant UI surface changes, and Render documents no equivalent.
