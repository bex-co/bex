# w6 · m518 — An SNI-proxy roll relays its external clients for up to the proxies' idle timeout, and nothing waits on that drain

**Worker:** worker6 **Goal:** when a deploy changes `/kv-sni-proxy` or `/pg-sni-proxy`, each node's old pod relays its open external Key Value and Postgres streams for up to the proxies' 1 h idle timeout, not w6/m515's 30 s. Neither the deploy's wait nor the platform's health signal waits on that drain: each passes once every node runs a Ready pod of the new revision. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [The deploy's SNI wait passes once each node's new pod is Ready](t001.md) | 1h | — |
| t002 | [The `bex-operator` Application stays Healthy while an old SNI pod drains](t002.md) | 2h | t001 |
| t003 | [Both SNI proxies drain for the longer bound](t003.md) | 45m | t002, w6/m515 t006 |
| t004 | [Prove it on the local cluster](t004.md) | 1h30m | t003 |
| t005 | [Record it in ADR021 § 10, ADR009 and the operator guide](t005.md) | 30m | t003 |
| t006 | [Ship and verify on production](t006.md) | 1h30m | t004, t005 |
| t007 | [Simplify](t007.md) | 15m | t006 |
| t008 | [Test coverage](t008.md) | 30m | t006 |
| t009 | [Closeout](t009.md) | 10m | t007, t008 |

## Definition of done

- `scripts/operator-pins.sh wait` passes once every node that should run each SNI DaemonSet runs a Ready, non-terminating pod of the DaemonSet's current template, while an old pod is still draining. It still fails on a new pod that never becomes Ready, and on a template that never carries the pinned image. `scripts/operator-pins.test.sh` covers each case.
- While an old SNI pod drains past 15 minutes, the `bex-operator` Application reads Healthy once each node's new pod is Ready, and `PlatformGitOpsUnhealthy` stays quiet. A new pod that never becomes Ready still reads Progressing and still fires that alert. The alert and its 15 m window are unchanged.
- Both DaemonSets set the chosen bound in their drain variables (`BEX_KV_PROXY_DRAIN_SECONDS`, `BEX_PROXY_DRAIN_SECONDS`), with a `terminationGracePeriodSeconds` above the 5 s `preStop` pause plus the bound. `scripts/gitops-validate.sh` and `TestTheDaemonSetGivesTheDrainItsBound` hold.
- On the local cluster, a binary-change roll of both DaemonSets keeps a held external client that sends a command every few seconds for longer than 30 s, and up to the bound. `operator-pins.sh wait` passes within seconds of each node's new pod being Ready, and the egress bytes of a stream that closes during the drain reach the scrape.
- On production, across a deploy that moves an SNI pin, a held external client on each proxy keeps its connection past 30 s and until it ends the session or the bound passes. The deploy's wait step passes without waiting for the old pods.
- ADR021 § 10 and ADR009 § SNI proxy state the new bound, the wait and health rules, and the gap that remains.

## Source + Goal linkage

- **Source:** promoted from inbox note [w6/146](../done/146.md) on 2026-11-19 by `/loopx w6`. w6/m515 t005 filed it as the longer-bound variant that milestone's Design weighed.
- **Goal linkage:** ADR008 § Render parity first.
  - Render breaks a datastore's connections for its own infrastructure only in service-affecting maintenance, which "usually occur[s] no more than once every three months", announced by email ([platform maintenance](https://render.com/docs/platform-maintenance), read 2026-11-19).
  - bex closes every external Key Value and Postgres stream without notice on each deploy that changes an SNI binary. After w6/m515 that close comes 30 s after the old pod's `SIGTERM`. No ADR018 row signs it off.
- **Expected outcome:** an external session that ends within the bound is never cut by an SNI roll: an interactive `psql` session, a migration, a long query or a batch job's subscriber. A connection that lives indefinitely (a pool, a permanent subscriber) is still closed once at the bound. That close is later than before, but it still happens (§ Design).
- **Why now:** w6/m515 made the SNI DaemonSets surge and drain, so the drain no longer holds the edge. The 30 s bound was kept only because the deploy's wait counts draining pods. Five of the six production pins w6/m470 t006 read moved both SNI pins, each for a `lego/types/v1alpha1` change, so these rolls are frequent.
- **Render parity task omitted:** this is a platform-internal mechanism. No REST, GraphQL, MCP or dashboard field or message changes. t004 and t006 check the tenant-visible effect.

## Findings at promotion (2026-11-19, code at `e7e25454a`)

- **The note's premise holds.**
  - `scripts/operator-pins.sh` `wait_for_proxies` (line 123) waits for each pinned image, then runs `kubectl -n bex-system rollout status daemonset/bex-<proxy> --timeout=300s`. `.github/workflows/deploy.yml` runs it as the step at line 1400, with `timeout-minutes: 12`.
  - The DaemonSet controller's status reads each node's oldest pod, and terminating pods count while they still run (`updateDaemonSetStatus`, `getNodesToDaemonPods(…, false)` skips only terminal pods that are being deleted). So `updatedNumberScheduled` stays below `desiredNumberScheduled` until the draining pod is gone. w6/m515 t003 measured this: 91.5 s for a 90 s drain.
  - No other gate waits on these DaemonSets. `scripts/verify-egress-meter.sh` runs `rollout status` on a fresh cluster, where no old pod exists.
- **A second consumer of the same status, which the note did not name.**
  - Argo's DaemonSet health applies `kubectl rollout status`'s rule ("Borrowed at kubernetes/kubectl/rollout_status.go", `gitops-engine` `pkg/health/health_daemonset.go`, read 2026-11-19). So the `bex-operator` Application reads Progressing for every second of a drain.
  - `bex-operator` is in `bex:platform_gitops_expected` (`deploy/gitops/base/prometheus.yaml`), and `PlatformGitOpsUnhealthy` fires after 15 m of non-Healthy (`deploy/gitops/base/rules/alerting_rules.yml`). A drain past 15 min would therefore fire it on every SNI-moving deploy.
  - Argo's sync does not stall. Both DaemonSets are in the last sync wave (wave 0; only `config/activator/platform-host-fallback-https.yaml` sets `-1`), and the last wave succeeds as soon as it is applied (`sync_context.go`: "EVEN if those objects subsequently degraded"). `deploy.yml` reads no Application's health except `openfga`'s.
  - The repository has no Argo health customization. Argo CD is installed and patched by `deploy.yml` step 3 from a pinned `install.yaml`.
- **The drain variables already parse any whole number of seconds** in w6/m515's stash (`sniproxy.DrainSeconds`), so the bound itself is a manifest change.
- **The scrape keeps a draining pod.** The `bex-pg-proxy` and `bex-kv-proxy` jobs use `kubernetes_sd_configs` `role: pod` and keep by `app.bex.co/component`, and a draining pod keeps its metrics listener (w6/m515 stash: `ctx` is the hard stop). So a stream that closes during the drain is billed at the next 15 s scrape. Only streams that close within one scrape of the process's exit are lost (§ Out of scope).
- **Memory and scheduling.** Each pod requests 32 Mi and is limited to 512 Mi (`config/*-sni-proxy/daemonset.yaml`). So a third pod per node, from a deploy within the bound, schedules. A draining pod accepts nothing new, and its memory falls as its streams end.
- **The rollout cadence.** Up to about 40% of pins move an SNI pin (22 of 56, an upper bound, ADR021 § 10), so a second SNI-moving deploy within an hour is plausible but not the rule.

## Design

- **The wait reads pods, not the DaemonSet's status (t001).** It passes when the DaemonSet's `observedGeneration` has reached its `generation`, and its pods include one per scheduled node that is Ready, not terminating, and carries the template's revision. That revision is `pod-template-generation` matching the DaemonSet's `deprecated.daemonset.template.generation` annotation, or `controller-revision-hash` matching the current ControllerRevision. The count must reach `desiredNumberScheduled`. Pass the same bound (300 s) for the new pods, as `rollout status` gets today. It must pass on today's `maxUnavailable: 1` DaemonSets as on w6/m515's surge ones.
- **The health signal keeps meaning "the roll is stuck" (t002).** Never exclude these DaemonSets from `PlatformGitOpsUnhealthy`, and never widen its window. Options, to be weighed with evidence:
  1. **A custom Argo health check scoped to these two DaemonSets.** `resource.customizations.health.apps_DaemonSet` in `argocd-cm`, patched by `deploy.yml` like `argocd-cmd-params-cm`. It applies per kind, so its Lua must keep the stock rule for every other DaemonSet. The Lua sees only the DaemonSet object, and its status cannot tell a draining old pod from a stuck new one. So it needs a signal written onto the object, for example an annotation the operator sets once every node runs a Ready pod of the current generation. That annotation must not make the Application OutOfSync (Argo syncs with `ServerSideApply=true`).
  2. **Release the draining pod from the DaemonSet** by removing its selector label, so the controller counts only the new pod. This also removes the pod from the Services' EndpointSlices, not just marks it terminating. Whether Cilium (production) and kube-proxy (local) keep an established NodePort flow to a backend removed that way must be proven before choosing it, because the drain exists to keep those flows. The scrape jobs keep by that label too.
  3. **Keep the bound below the alert's window** (about 10 min), and accept Progressing for that long after each SNI-moving deploy.
- Prefer the option that keeps the bound at the 1 h idle timeout. Choose 3 only if 1 and 2 both fail on evidence, and then file the longer bound as its own follow-up. Record the choice in t002.
- **The bound (t003).** Target 3,600 s, the proxies' idle timeout (`defaultIdleTimeout`, `BEX_KV_PROXY_IDLE_TIMEOUT`, `BEX_PROXY_IDLE_TIMEOUT`). An idle stream closes by itself within that time anyway, so a longer bound would add nothing. `terminationGracePeriodSeconds` becomes the 5 s pause plus the bound plus a margin. The ssh-gateway's 3,660 s grace for open sessions (w6/m482) is the precedent. Kubernetes evicts no DaemonSet pod in a node drain, so no node drain waits for it.
- **What it does not fix.** A client that keeps its connection for longer than the bound is still closed once, at the bound, without notice: a pool, a permanent subscriber. Render does not do this outside announced maintenance. The bigger lever is rolling these proxies less often. Each pin move w6/m470 t006 read (five of six pins) was for a `lego/types/v1alpha1` change, because both binaries link the whole CRD scheme (w6/m470 § Findings). That direction is not filed. t005 records it as the gap that remains, and t009 files it if it is still open.

## Sequencing constraint

- **t003 and every later task wait for w6/m515.** The drain itself is w6/m515's code, in stash `loopx/w6/m515` until w6/m470 t006 records its held-client deploy. Ship t003 only after w6/m515 t006 has recorded its production check, or w6/m515 is in `done/`. That check reads held idle clients closing at the 30 s bound, and a longer bound shipped first would change what it reads.
- **t001 and t002 may ship before**, but only if they change neither SNI binary (the packages w6/m470 § "Do not change the SNI binaries" lists) nor either DaemonSet template, because either change rolls both DaemonSets and spends the kept-pin deploy w6/m470 waits for. Anything that does change one waits with t003.
- **ADR021 § 10 changes in the stash.** w6/m515's stash rewrites § 10 and links this work as `../.pm/w6/146.md`. That note is now [w6/146](../done/146.md), so the link moves to this milestone when the stash is applied (`bash scripts/docs-links-validate.sh` reports it). t005 edits the section w6/m515 lands, not today's.
