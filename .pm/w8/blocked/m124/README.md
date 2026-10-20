# w8 · m124 — Scope a service's logs and metrics to its own incarnation

**Worker:** worker8 **Goal:** A service created with the name of a deleted one never shows the predecessor's logs, request lines, instances or metric points, on any surface. Render's logs and metrics belong to the service ID. **Status:** blocked — t001–t006 done 2026-10-20; t007 live check on production after the release pipeline deploys bex-api and GitOps syncs the log shipper

## Tasks (in order)

| id   | title                                                                      | est | depends_on                 |
| ---- | -------------------------------------------------------------------------- | --- | -------------------------- |
| t001 — **DONE** | Log shipper: label App streams with the pod's `bex.co/app-id`              | 30m | —                          |
| t002 — **DONE** | Logs: every App read selects its own id and starts at the App's creation   | 60m | w8/m124/t001               |
| t003 — **DONE** | Metrics: Prometheus and Loki series exclude pods and points before the App | 60m | —                          |
| t004 — **DONE** | Render parity: logs/metrics identity in ADR018/ADR010                      | 15m | w8/m124/t002, w8/m124/t003 |
| t005 — **DONE** | Simplify the changed code                                                  | 15m | w8/m124/t004               |
| t006 — **DONE** | Test coverage: two incarnations of one name on every read path            | 40m | w8/m124/t004               |
| t007 | Close out the milestone                                                    | 30m | w8/m124/t005, w8/m124/t006 |

## Definition of done

Live on production after the release pipeline deploys bex-api and GitOps syncs the log shipper, on fresh free echo services deleted by ID with 404 proofs. Repeat the note's steps: create, hit a marker path, delete, recreate the same name.

- The new service's `bex logs -r <new id> --text <marker>`, `--path`, unfiltered history and `--tail` show none of the predecessor's lines, its SIGTERM lines included.
- Its instance discovery lists only its own instances.
- `GET /v1/metrics/{cpu,memory,instance-count,http-requests}?resource=<new id>` returns no point before the new service's creation and no series for the predecessor's pod.
- An existing service's history from before the label rollout still reads (unchanged surface for services that were never recreated).

## Source + Goal linkage

- **Source:** promoted from inbox note `w8/144` on 2026-10-20 (`/qa-find-bugs-cli` sweeps 1244 and 1245, [evidence](../../evidence/cli-sweep1244/README.md)). The note's text is kept in [`done/144.md`](../../done/144.md).
- **Goal linkage:** [ADR018](../../../../docs/ADR018-render-parity.md) logs and metrics rows; [ADR010](../../../../docs/ADR010-observability.md). Render scopes both to the service ID.
- **Expected outcome:** a deleted service's paths, client IPs and process output stop resurfacing under a new service, and a new service's history and charts start empty.
- **Why now:** severity major (deleted data resurfaces). Name reuse right after deletion is common, and the sweep recreated a name 2 s after deleting it.

## Design

- **Logs (t001, t002):** the shipper adds `app_id` from `bex.co/app-id` on App pods. Every App read keeps the namespace/app tenant matchers (`loki.go:438–444`) and adds `app_id=~"<srv-id>|"`, which matches the service's own streams and the unlabeled streams shipped before t001. A predecessor's terminating pod carries its own id and is excluded. The read's start is clamped to the App CR's `creationTimestamp`, which excludes the predecessor's unlabeled history and its request lines (Traefik streams carry no pod label). This applies at all five call sites (`service.go` `QueryLogs`, the initial tail, `LogLabelValues`, `followLogs`, `metrics/lokisource.go`).
- **Metrics (t003):** every App metric read clamps `start` to `creationTimestamp`. The Prometheus selectors (`metrics/service.go` cpu, memory, instance-count, the pod-scoped families) keep only pods created at or after the App: `and on(namespace, pod) (kube_pod_created >= <unix>)`. A pod created before the App cannot belong to it, so this is exact, and it removes the predecessor's terminating pod as an instance.
- **Review refinements (t005):** the floor is `core.AppNotBefore`, the earlier of the CR's creation and the service id's mint time. The store projector can re-create a CR for the same id, and a creation-only floor would hide that service's own history. The mint time only ever relaxes the creation floor. Unlabeled pods (Job pods) created before the floor are dropped too. The instance list and SSH targets in `apps/` use the same `core.OwnAppPods`.
- **Out of scope:** renames followed by recreation of the old name (unverified in the note), and Postgres/Key Value, which select by immutable id.
