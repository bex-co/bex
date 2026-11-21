# w6 · m518 — An SNI-proxy roll relays its external clients for up to the proxies' idle timeout, and nothing waits on that drain

**Worker:** worker6 **Goal:** when a deploy changes `/kv-sni-proxy` or `/pg-sni-proxy`, each node's old pod relays its open external Key Value and Postgres streams for up to the proxies' 1 h idle timeout, not w6/m515's 30 s. Neither the deploy's wait nor the platform's health signal waits on that drain: each passes once every node runs a Ready pod of the new revision. **Status:** done

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [The deploy's SNI wait passes once each node's new pod is Ready](done/t001.md) | 1h | — | — **DONE**
| t002 | [The `bex-operator` Application stays Healthy while an old SNI pod drains](done/t002.md) | 2h | t001 | — **DONE**
| t003 | [Both SNI proxies drain for the longer bound](done/t003.md) | 45m | t002, w6/m515 t006 | — **DONE**
| t004 | [Prove it on the local cluster](done/t004.md) | 1h30m | t003 | — **DONE**
| t005 | [Record it in ADR021 § 10, ADR009 and the operator guide](done/t005.md) | 30m | t003 | — **DONE**
| t006 | [Ship and verify on production](done/t006.md) | 1h30m | t004, t005 | — **DONE**
| t007 | [Simplify](done/t007.md) | 15m | t006 | — **DONE**
| t008 | [Test coverage](done/t008.md) | 30m | t006 | — **DONE**
| t009 | [Closeout](done/t009.md) | 10m | t007, t008 | — **DONE**

## Definition of done

- `scripts/operator-pins.sh wait` passes once every node that should run each SNI DaemonSet runs a Ready, non-terminating pod of the DaemonSet's current template, while an old pod is still draining. It still fails on a new pod that never becomes Ready, and on a template that never carries the pinned image. `scripts/operator-pins.test.sh` covers each case.
- While an old SNI pod drains past 15 minutes, the `bex-operator` Application reads Healthy once each node's new pod is Ready, and `PlatformGitOpsUnhealthy` stays quiet. A new pod that never becomes Ready still reads Progressing and still fires that alert. The alert and its 15 m window are unchanged.
- Both DaemonSets set the chosen bound in their drain variables (`BEX_KV_PROXY_DRAIN_SECONDS`, `BEX_PROXY_DRAIN_SECONDS`), with a `terminationGracePeriodSeconds` above the 5 s `preStop` pause plus the bound. `scripts/gitops-validate.sh` and `TestTheDaemonSetGivesTheDrainItsBound` hold.
- On the local cluster, a binary-change roll of both DaemonSets keeps a held external client that sends a command every few seconds for longer than 30 s, and up to the bound. `operator-pins.sh wait` passes within seconds of each node's new pod being Ready, and the egress bytes of a stream that closes during the drain reach the scrape.
- On production, across a deploy that moves an SNI pin, a held external client on each proxy keeps its connection past 30 s and until it ends the session or the bound passes. The deploy's wait step passes without waiting for the old pods.
- ADR021 § 10 and ADR009 § SNI proxy state the new bound, the wait and health rules, and the gap that remains.

## Source + Goal linkage

- **Source:** promoted from inbox note [w6/146](../../done/146.md) on 2026-11-19 by `/loopx w6`. w6/m515 t005 filed it as the longer-bound variant that milestone's Design weighed.
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
- **ADR021 § 10 changes in the stash.** w6/m515's stash rewrites § 10 and links this work as `../.pm/w6/146.md`. That note is now [w6/146](../../done/146.md), so the link moves to this milestone when the stash is applied (`bash scripts/docs-links-validate.sh` reports it). t005 edits the section w6/m515 lands, not today's.

## Progress — 2026-11-19 (t001 and t002 shipped; parked on w6/m515)

- **t001.** `scripts/operator-pins.sh wait` now reads each SNI DaemonSet's pods, not `rollout status`. On the local cluster, with each old pod held for 90 s, the wait passed 1 s after the new pods were Ready, while `rollout status` waited 96 s ([t001](done/t001.md#result-2026-11-19)).
- **t002, option 1.**
  - The operator keeps `app.bex.co/new-pods-ready: "<generation>/<nodes>"` on both SNI DaemonSets while every node runs a Ready, non-terminating pod of the current template.
  - `deploy.yml` installs `deploy/argocd/daemonset-health.lua` as Argo's DaemonSet health check: the built-in rule, plus that annotation.
  - Option 2 and its keep-the-labels variant were rejected on the DaemonSet controller's code, and option 3 is not needed. The health signal sets no maximum, so t003 can take the 3,600 s target ([t002](done/t002.md#choice-2026-11-19-option-1-an-argo-health-check-that-reads-a-signal-the-operator-writes)).
- **Neither changes an SNI binary or a DaemonSet template.** `go list -deps` shows neither binary links the new code, and `config/prod` renders both DaemonSets byte-identical to `3d8835416`. So the shipping deploy keeps both SNI pins, as w6/m470 needs.
- **Gate.** t003 changes both DaemonSets. It ships after w6/m515 t006 records its production check, or once m515 is in `done/` (§ Sequencing constraint). m515 itself waits for w6/m470 t006. Next: apply w6/m515's stash, then t003 on top of it, then t004–t009.

## Unblock record — 2026-11-20 (verified evidence, `/loopx w6`)

- **Affected:** t003–t009.
- **Source: verified evidence.** w6/m515 is in `done/` (`17e8848c0`), and its [t006](../../done/m515/done/t006.md#step-2-the-sni-moving-deploy-2026-11-20-pin-a47204c7e) recorded the production check this milestone waited for: across pin `a47204c7e` (producer `3e53abbe3bc3`), held external clients on both proxies got their replies after `SIGTERM` and closed at the 30 s bound. Both gates in § Sequencing constraint hold.
- **The stash is gone.** w6/m515 shipped its drain as `495d04b86`, so t003 edits `main`'s DaemonSets directly, and ADR021 § 10's link to this milestone already points here. Its t008 (`17e8848c0`, `sniproxy.HardStopOnManagerExit`) changes both SNI binaries, so the first pin that carries it moves both SNI pins whatever t003 does.
- **Rollout consequence.** `bex-operator` tracks `main`, so t003's template change rolls both DaemonSets at push, on the running image, and those old pods still drain for 30 s (w6/m515 t006 § Step 1, Deviation). The first pin that carries t003 and `17e8848c0` then rolls them by image, and its old pods drain for the new bound: that is t006's held-client deploy.
- **Next:** t003 at 3,600 s, t004 on the local cluster, t005, ship, then t006 across that pin.

## Park — 2026-11-20 (t001–t005, t007 and t008 done; t006 waits for the next pin)

- **Shipped** as `081c847e1`. Its template rolled both DaemonSets at push (06:15Z), one surge per node, and those old pods drained for their own 30 s. The new pods run the 3,600 s bound with a 3,610 s grace period ([t006 § Step 1](done/t006.md#step-1-shipped-as-081c847e1-and-the-template-rolled-at-push-2026-11-20)).
- **Armed.** Eight probes in `bex-canary` hold busy, self-ending and idle Key Value and Postgres clients through the edge, with new-connection loops on each proxy ([t006 § Step 2](done/t006.md#step-2-armed-2026-11-20-05510617z)).
- **Gate.** The first pin that carries `081c847e1` rolls both DaemonSets by image with the new bound on the old pods. It moves both SNI pins anyway, for `17e8848c0`. Next: [t006 § Next](done/t006.md#next-the-worker-after-the-first-pin-that-carries-081c847e1), then t009.

## Unblock record — 2026-11-20 (verified evidence, `/loopx w6`)

- **Affected:** t006 and t009.
- **Source: verified evidence.** Production pin `4e37ec03f` (producer `12716d1d7097`, 08:47Z) carries `081c847e1`. Its write-back logged `moved` for both `/kv-sni-proxy` and `/pg-sni-proxy`, so the gate holds. The pin also carries w6/148's `9e792160a` but not w6/149's `9ea476b3b`, so the old pods ran with the 3,600 s drain.
- **Next:** t006 § Next on that pin's roll, then t009.

## Closed — 2026-11-20 (pin `4e37ec03f`)

- **Production ([t006 § Step 2](done/t006.md#step-2-the-sni-moving-deploy-2026-11-20-pin-4e37ec03f)).**
  - Both DaemonSets rolled by surge at 08:50:26–08:51:06Z.
  - A busy client on each proxy read 720 replies on its old connection after its old pod's `SIGTERM`, and was closed at `SIGTERM` + 3,600–3,601 s.
  - The idle clients, kept under the edge load balancer's 1 h idle close by a 10-min keepalive, closed at the same bound.
  - A 20-min session on each proxy ended on its own mid-drain with the same id, and its old pod exited with it.
  - The deploy's wait passed in 3 s while five old pods drained. `bex-operator` read Healthy for the rest of the hour, and `PlatformGitOpsUnhealthy` never fired.
  - The probes and `dpg-dbvtucpbgjqs73fpskhg` are deleted.
- **Found.** The bytes of a draining pod's last stream, and of every stream the bound closes, never reached the scrape. That gap predates this milestone and is filed as [w6/151](../../151.md). Rolling the proxies less often is [w6/150](../150.md) (t009).
