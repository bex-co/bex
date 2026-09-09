# w5 · m87 — Consistent instance IDs across logs, metrics, and SSH

**Worker:** worker5 **Goal:** Give each resource instance one public identity across diagnostics and live targeting without exposing Kubernetes pod names. **Status:** done

**Estimate:** 4h implementation; 6h 15m including standing closing tasks (9 tasks).

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Implement shared instance identity mapping and compatibility rules — **DONE** | 60m | — |
| [t002](done/t002.md) | Apply public IDs to metric series and instance discovery — **DONE** | 45m | t001 |
| [t003](done/t003.md) | Translate log output, discovery, and filters together — **DONE** | 60m | t001 |
| [t004](done/t004.md) | Align live SSH targeting and datastore instance variants — **DONE** | 45m | t001 |
| [t005](done/t005.md) | Align dashboard legends, chips, and existing instance links — **DONE** | 30m | t002, t003, t004 |
| [t006](done/t006.md) | Render parity — **DONE** | 30m | t005 |
| [t007](done/t007.md) | Simplify — **DONE** | 30m | t006 |
| [t008](done/t008.md) | Test coverage — **DONE** | 60m | t006, t007 |
| [t009](done/t009.md) | Closeout — **DONE** | 15m | t008 |

Task IDs in depends_on are relative to w5/m87 unless written as a full wN/mN/tNNN ID. Resolve completed dependencies through done/ locations; the ID remains stable when the file moves.

## Definition of done

- [x] For an authorized live App replica, serviceInstances, metric series, metrics INSTANCE discovery, log entries, and logLabelValues(instance) return the same public identity; no raw pod name is emitted in those instance fields.
- [x] Filtering by a returned log instance ID selects the same instance for history and live tail. The mapping works for App, Postgres, and Key Value paths and remains scoped to the authorized resource.
- [x] Historical metrics and logs retain stable identities and remain queryable after the pod is deleted. Live-only UID lookup is not the sole history-resolution mechanism.
- [x] Existing live SSH selectors continue to target their original eligible pod. A retired or foreign selector never falls through to a different live pod. Compatibility for previously issued IDs and bookmarked raw-name filters is explicit and exercised.
- [x] REST, GraphQL, MCP, dashboard, and the unmodified official CLI agree on their exposed fields and filters. Verify live and historical cases on a disposable dev-5 fixture, including replacement, no-data, and forbidden-resource controls.
- [x] Affected backend and dashboard checks pass, evidence and ADR018 are updated, and all standing closing tasks are complete.

## Source + Goal linkage

- **Source:** User-approved pm-brainstorm proposals 1–3 for w5, materialized 2026-09-08. Proposal 1 absorbs [w4/059](../../w4/done/059.md) and [w4/060](../../w4/done/060.md), live QA findings from 2026-09-08 rechecked against current code. Their move to done/ records promotion only; implementation remains pending here.
- **Goal linkage:** [ADR008](../../../docs/ADR008-vision.md) pillars 1–3: compatible APIs and consistent machine-readable hosting state. A user or agent can correlate the same replica across instance listing, metrics, logs, and SSH.
- **Expected outcome:** Public instance IDs agree across the exposed surfaces; log instance chips actually filter that instance; retained history remains usable after a pod disappears.
- **Why now:** Both source notes require the same UID-versus-pod-name decision. Combining them in available w5 capacity avoids incompatible fixes in separate packages. This supplies the stable identity required by w5/m89.
- **Render parity:** Included: REST, GraphQL, MCP, dashboard, and the existing official CLI/SSH consumers must agree. Correct the scoped gaps under ADR018's Application logs (query + live tail), Request / HTTP logs + structured filters, and Core metrics rows; those rows currently carry broad ✅ markers, so do not claim a new capability family or erase their documented divergences.

## Scope and constraints

- The current live helper derives IDs from pod UID; Prometheus and Loki history generally retain pod names only. Implement a compatible mapping before switching producers and consumers. A name-derived App mapping is a candidate, not permission to break existing selectors or conflate reused datastore pod names.
- The source findings are own-resource identifier exposure and contract inconsistency, not a cross-tenant data-leak claim.
- ServiceInstances and SSH already shipped in w6/m26 and w2/m39. This changes their identity interoperability, not their eligibility policy or transport. w6/m110 owns the distinct pod-selection/metering issue.
- Metric instance selection and MIN/MAX/AVG UI controls belong to w5/m89. Internal Loki/Prometheus pod labels may remain internal implementation data.
- Use the existing internal/id convention and leaf helpers; preserve operator → types ← backend and the standalone CLI boundary.

## Verification record

Shipped 2026-09-08 (w5/m87):

- Canonical public id = `id.ServiceInstanceID(resourceID, podName)`; legacy UID-derived SSH selectors accepted via `MatchServiceInstance`; raw-name log filters still accepted as inputs.
- Unit coverage: `internal/id` instance helpers; apps SSH legacy/ambiguous; logs `TestInstanceFilterAcceptsPublicID`; metrics projection on snapshot/range/limit/discovery/datastore; dashboard shortInstance for opaque 20-char suffixes.
- Docs: ADR020 compound instance note; ADR035 name derivation + legacy; ADR010 public vs store `pod`; ADR018 List instances row.
- Suites: `go test ./internal/{id,apps,metrics,logs}/`; dashboard log-line-list + log-viewer tests.
