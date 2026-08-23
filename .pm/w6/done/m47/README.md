# w6 · m47 — Free-tier sleep/wake broken (blocker) + event/log copy conflates hibernate with suspend

**Worker:** worker1 **Goal:** a hibernated free-tier service actually wakes on the next request (the core free-tier value proposition), and the platform's own event/log copy stops asserting contradictory or misleading states. **Status:** done 2026-08-23 — all three defects triaged, root-caused, fixed, test-proven, and live-verified on the shared CAPD cluster

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Root-cause + fix why a hibernated free-tier service never wakes (404 forever) | 90m | — |
| t002 | Events feed labels auto-hibernate "Service suspended", contradicting the user-suspend-only invariant | 30m | — |
| t003 | Logs empty-state title never branches on the active filter | 15m | — |
| t004 | Render parity | 30m | t001, t002, t003 |
| t005 | Simplify | 20m | t004 |
| t006 | Test coverage | 45m | t004 |
| t007 | Closeout | 10m | t006 |

- t001 — **DONE** (root cause confirmed, fix shipped, live-verified before/after)
- t002 — **DONE**
- t003 — **DONE**
- t004 — **DONE**
- t005 — **DONE**
- t006 — **DONE**
- t007 — **DONE**

## Triage verdict

All three reported defects are real and were confirmed against the source, not taken on report. t001's cause — which the original filing left explicitly unverified and hypothesized as a misconfigured `BEX_ACTIVATOR_SERVICE` — turned out to be provable statically, and was something else:

**An Ingress backend resolves only within the Ingress's own namespace.** `ingressBackend` named the platform Service `bex-activator` (namespace `bex-system`) as the public backend for an auto-hibernating App, but under ADR043 every tenant App's Ingress lives in its own `tea-<xid>` namespace. The backend did not exist there, so Traefik answered with its own default 404 — the exact body/header shape observed — the activator never saw the request, and the phase never moved. The same function already handled this correctly for maintenance mode via an ExternalName alias; the hibernate branch simply never got one, and every hibernate test ran in namespace `default`, where the defect is invisible.

Fixes shipped for all three, each proven red-before/green-after (t006), and the wake path then verified live end to end (t001). The env var was wired correctly all along — the live operator had `BEX_ACTIVATOR_SERVICE=bex-activator` set correctly the whole time; the value was right, the namespace was unreachable.

The live leg also earned its keep on t002: `TestPushWorkerEnqueuesObservedLifecycleFacts` is `BEX_TEST_DB_URI`-gated and had been silently skipping, so the store change had never run against real Postgres. Pointed at a database it caught that an auto-hibernate no longer enqueues a `service_suspended` push — the intended change, so the test's Hibernated leg is now marked as a genuine user suspension. Migration 0097 applies cleanly and the full backend suite passes on a pristine database.

## Definition of done

- [x] A Free-tier web service, hibernated by letting its idle timeout elapse, wakes and serves its real application response on the next request — no `404 page not found`, no indefinite `Hibernated` phase. **Live-verified before/after on one real App in a tenant namespace** (transcript in `done/t001.md`): pre-fix, the Ingress backend `bex-activator` does not exist in the App's namespace; post-fix, the App-owned alias does, the sleeping service answers with the `Retry-After: 5` wake page instead of a 404, and it is serving its own `HTTP 200` about six seconds later.
- [x] The Events tab (and webhook/notification payloads) for that hibernate→wake cycle report something textually distinct from a user clicking Suspend/Resume — now `service_hibernated` / `service_woken`, never the `service_suspended` / `service_resumed` pair.
- [x] The Logs tab's empty state, when a filter yields zero results on a service with real log history, shows an internally consistent message.
- [x] REST, GraphQL, and MCP agree with the dashboard on all of the above (t004) — by construction, not by parallel edits; see t004 for why.
- [x] New regression tests for the hibernate/wake cycle, the event-fact branching, and the logs empty-state title exist and are proven red-before/green-after (t006).

## Source + Goal linkage

- **Source:** live `/qa-find-bugs` hunt, 2026-08-22 (part of the `w6` continuous QA cadence — see `w9/m89`, `w9/m92`, and `w6/m44`–`w6/m46` for the same lineage). Evidence: repeated `curl -D -` + GraphQL `phase` probes against `qa-20260822-sleep2` (deleted, cleaned up), plus `file:line` reads of `lego/operator/cmd/activator/main.go`, `lego/operator/internal/controller/app_controller.go`, `lego/backend/internal/store/event_facts.go`, and `dashboard/src/features/logs/components/log-viewer.tsx`.
- **Goal linkage:** [ADR029](../../../docs/ADR029-static-sites.md)'s sibling economics note and the operator's own `BEX_ACTIVATOR_SERVICE` contract ([lego/operator/CLAUDE.md](../../../lego/operator/CLAUDE.md)) — dense bin-packing and the free tier's cost model depend on auto-hibernate/wake actually working; a free service that never wakes is not "slow," it is silently broken, which undermines the entire free-tier pitch in [ADR008](../../../docs/ADR008-vision.md).
- **Expected outcome:** a hibernated free service is reachable again within its advertised wake window, every time; the platform's own audit trail is trustworthy enough that a user (or their webhook/notification integration) can tell an intentional suspend apart from a routine sleep cycle.
- **Why now:** this was the highest-severity finding of the hunt after `w6/m46`'s security issue — it breaks a core, load-bearing product promise (not an edge case; any idle free service reproduces it) and was caught live, not theorized.
- **Render parity:** included (t004). Render has no equivalent of the new hibernate/wake event types — it reports a slept free service through the same suspend/resume vocabulary bex used to — so both are recorded as deliberate `bexExtensions` in the dated vocabulary fixture, which CI gates.

## Incidental finding

Raising the environment surfaced a defect unrelated to this milestone, filed as inbox note `w6/032`: `lego/operator/config/rbac/role.yaml` grants no `replicasets`, but the operator lists ReplicaSets through its cached client (`app_controller.go`, added 2026-08-09 by w7/m79). Any `make deploy` operator therefore fails informer cache sync and reconciles **nothing** — which is what blocked this milestone's own live leg until the grant was added by hand. Production is unaffected: `deploy/gitops/base/operator-daytoday-rbac.yaml` grants it, so only the kubebuilder-generated role is short.

## Deploy note

The wake alias is admitted by a **fail-closed** `ValidatingAdmissionPolicy`. `deploy/gitops/base/operator-alias-admission.yaml` must reach the cluster before the operator image that creates the alias — its sync-wave `-3` already guarantees this under Argo, but a manual out-of-band operator roll would have every wake alias denied (surfacing as an alias/`IngressFailed` error on the App, not a silent regression).
