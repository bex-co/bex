# w6 · m470 — An operator deploy that leaves the SNI proxies' code alone keeps every external datastore connection

**Worker:** worker6 **Goal:** `bex-kv-sni-proxy` and `bex-pg-sni-proxy` roll only when their own binary (or their DaemonSet template) changes, not on every operator deploy, so external TLS clients of Key Value stores and Postgres databases survive the platform's routine releases, as on Render. **Status:** done

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [A command tells whether an image carries the same SNI proxy binary as another](done/t001.md) — **DONE** | 1h30m | — |
| t002 | [Each SNI DaemonSet takes its own image pin, which renders today's image](done/t002.md) — **DONE** | 1h | — |
| t003 | [deploy.yml keeps an SNI pin while its binary is byte-identical, and waits for the digest it pinned](done/t003.md) — **DONE** | 1h30m | t001, t002 |
| t004 | [Prove it against GHCR and the rendered manifests](done/t004.md) — **DONE** | 45m | t003 |
| t005 | [Record the rule in ADR021 and ADR009](done/t005.md) — **DONE** | 30m | t003 |
| t006 | [Ship and verify on production](done/t006.md) — **DONE** | 1h30m | t004, t005 |
| t007 | [Simplify](done/t007.md) — **DONE** | 15m | t006 |
| t008 | [Test coverage](done/t008.md) — **DONE** | 30m | t006 |
| t009 | [Closeout](done/t009.md) — **DONE** | 10m | t007, t008 |

## Definition of done

- The production deploy that ships this milestone leaves both SNI DaemonSets on their image, revision and pods, unless the producer's `/kv-sni-proxy` or `/pg-sni-proxy` differs from the running one. Then t006 names the commit that changed it and says why that roll could not be avoided.
- On production, one external TLS client held to a Key Value store (`PING`, same `CLIENT ID`) and one held to a Postgres database (`SELECT 1`, same `pg_backend_pid()`) still answer on the same connection after a later operator deploy whose SNI binaries are byte-identical, and each DaemonSet keeps its pods and its revision.
- A deploy whose SNI binary changed still moves that DaemonSet to the new digest, and `deploy.yml` waits for it to roll out. Any doubt (a registry error, a missing build step) also moves it.
- A test fails if the pin step moves a pin whose binary is byte-identical, and one fails if it keeps a pin whose binary changed.
- ADR021 § 10 and ADR009 § SNI proxy record the rule. w6/142 (draining) stays its own item.

## Source + Goal linkage

- **Source:** promoted from inbox note [w6/141](../../done/141.md) on 2026-11-18, filed by w6/m469 t004 as the external half of w6/140. The loopx dispatch picked it ahead of w6/138 as a live production defect.
- **Goal linkage:** ADR008 § Render parity first. Render breaks a datastore's connections when the customer changes it (a plan change, a restart), not when Render ships its own platform ([Key Value](https://render.com/docs/key-value)). No ADR018 row signs off this divergence.
- **Expected outcome:** a deploy that leaves the SNI binaries alone closes no external datastore connection. Over 2026-11-12 to 2026-11-18 at most 22 of 56 production pins touched a package the SNI binaries link (see Findings), so at least 34 of those deploys would have kept every external connection.
- **Why now:** each production deploy (at most one per 6 h since block-eden-mono w1/m25) closes every external Key Value and Postgres connection at once. w6/m469 already holds the internal proxy, so the SNI DaemonSets are the remaining routine drop. Builds on w6/m469's `registry.ImageFileSHA256`.
- **Render parity task omitted:** the change is platform-internal (a `deploy.yml` pin and two DaemonSet image references). No REST, GraphQL, MCP or dashboard field or message changes. The tenant-visible effect, an external connection surviving a platform deploy, is checked on production in t006.

## Findings at promotion (2026-11-18, production read-only and git)

- **Only the image moves.** On production both DaemonSets run `bex-operator@sha256:b4104643…` (the `controller=` pin, producer `14eaa71b9`), `RollingUpdate` with `maxSurge: 0`, `maxUnavailable: 1`, a 30 s grace period. `bex-kv-sni-proxy` is at revision 809 (generation 808), `bex-pg-sni-proxy` at 892. The last two ControllerRevisions of each (2026-11-17 18:37Z and 2026-11-18 01:31Z) differ only in `image` (`f7b06e9c…` → `b4104643…`). So holding the image stops the roll; nothing else in the template changes per deploy.
- **What the SNI binaries link.** `go list -deps ./cmd/kv-sni-proxy` (and `pg-sni-proxy`): the first-party packages `lego/types/{v1alpha1,netutil,proxyproto,k8sname,env,tiers}` and `lego/operator/internal/sniproxy`, plus `k8s.io` and controller-runtime from `lego/operator/go.mod`. They do not link `internal/registry`, so changing `ImageFileSHA256` leaves them byte-identical. They do link the whole CRD scheme (`v1alpha1`), so any CRD types edit rolls them.
- **How often they change.** Over the 56 pins from 2026-11-12 to 2026-11-18, 22 had a commit touching one of those paths since the previous producer. That is an upper bound: `fc4fbf28a` touched `lego/types/tiers` (0eed45bcb) yet carried the same bytes as `336b647b1` (w6/141's GHCR hashes), because test files and unlinked code do not change the binary.
- **The next deploy rolls them once anyway.** `936b61bab` (w6/m467) changed `lego/types/v1alpha1/database_types.go` after the running producer `14eaa71b9`. The next pin (due once the 01:28Z pin is 6 h old, carrying w6/m469 and w6/m467) changes both SNI binaries, whatever this milestone does. t006 must compare against the pin that is live when this milestone ships.
- **Where the image comes from.** Argo deploys `lego/operator/config/prod` with `kustomize.images: controller=ghcr.io/bex-co/bex-operator@sha256:…` (`deploy/gitops/base/bex.yaml`). Both DaemonSets use the placeholder `controller:latest`, as do the manager, bex-api, activator and the rest, so today only a separate placeholder can give them their own pin. `deploy.yml`'s "pin image digests in GitOps (write-back)" seds that one line, and "wait for PROXY-compatible datastore proxies" requires `${IMAGE}@${operator_digest}` on both DaemonSets.
- **Kustomize trap.** `config/prod` builds `../default` first. If `config/default` mapped a new placeholder to `controller`, the inner build would rename it before Argo's outer `kustomize edit set image` could match it, and the DaemonSets would follow `controller=` again. The new placeholders must reach `config/prod` untouched. Local `make deploy` (`config/deploy` + `kustomize edit set image controller=${IMG}`) must set all three.
- **Platform.** `ImageFileSHA256` reads `runtime.GOARCH`, as the operator compares an image with itself. The deploy job runs on `ubuntu-24.04-arm` and the operator image is `linux/amd64` only (`platforms: linux/amd64`), so the comparison needs an explicit platform.
- **Guards that pin today's shape:**
  - `scripts/gitops-validate.sh` greps `deploy.yml` for `BEX_EXPECTED_PROXY_IMAGE: ${{ env.IMAGE }}@${{ needs.build.outputs.operator_digest }}`, which must change in step.
  - `scripts/deploy-superseded.sh` (with its `.test.sh`) and `deploy.yml`'s inline write-back check each expect exactly one generated digest field in `deploy/gitops/base/bex.yaml`.
  - `scripts/image-pin-validate.sh` exempts each DaemonSet's `controller:latest` by file and reference.
- **Rejected direction: a content-addressed image per proxy.** It needs two new GHCR packages with signing, SBOM and the CVE gate, a reproducible image config (timestamps), and it would still roll on every distroless base bump. The pin hold reuses w6/m469's digest-verified reader and the existing pin write-back.
- **Not covered: w6/142.** A deploy that does change an SNI binary still replaces each pod (`hostPort`, no surge) and closes its streams at once. This milestone does not change that.

## Design constraints

- **Do not change the SNI binaries in this milestone.** Leave `lego/types/{v1alpha1,netutil,proxyproto,k8sname,env,tiers}`, `lego/types/go.mod`, `lego/operator/internal/sniproxy`, `lego/operator/cmd/{kv,pg}-sni-proxy`, `lego/operator/go.mod`/`go.sum`, and `lego/Dockerfile`'s builder image and SNI build lines alone. The new command adds no module dependency. If a change there becomes unavoidable, t006 and the result say so plainly.
- **Hold the image, not the template.** An env, probe or resource change in a DaemonSet manifest still rolls it, now on the held image, which runs a byte-identical binary.
- **Any doubt rolls.** When the comparison cannot prove the binaries identical (a registry error, a tag, a missing build step, a layer digest mismatch), the pin step pins the new digest, which is today's behavior. It never fails the deploy for it.
- **The first sync renders today's image.** The commit that introduces the two pins sets both to the `controller=` digest on `origin/main` at ship time, so Argo's first sync changes no DaemonSet. A pin that lands in between conflicts on the adjacent line during `/ship`'s rebase; re-read it then.
- **An in-flight deploy refuses the change.** `deploy.yml`'s inline check (in the workflow file of the run that is already building) refuses a write-back when `bex.yaml` changed beyond its generated field. Ship when `gh run list --workflow deploy.yml` shows no run past `changes`, or record the one refused run.
- The held digest can be weeks old. GHCR keeps every `bex-operator` version (w6/m469).

## Progress — 2026-11-18 (parked on its production deploys)

- **Shipped:** `registry.ImageFileSHA256` takes a platform and `cmd/proxy-pin` answers which digest to pin (t001); each SNI DaemonSet's own placeholder and `bex.yaml` pin, seeded at the live `controller=` digest so the first sync renders today's image (t002); `scripts/operator-pins.sh` holds or moves each pin in the write-back, `deploy.yml` builds `proxy-pin` and waits for each DaemonSet's own pin, and both generated-field checks know the three pins (t003); the GHCR proof, kept for `336b647b1` → `fc4fbf28a` and moved for `5e0fb5ad8` → `e36296ae8` on both proxies (t004); ADR021 § 10 and ADR009 (t005); `/simplify` (t007); and a red test for every reverted rule (t008).
- **Expected on production:** the first deploy after this commit moves both pins once, because `936b61bab` (w6/m467) changed `lego/types/v1alpha1` after the running producer `14eaa71b9` (t006 § Progress). The deploy after that holds both while no commit touches the SNI binaries' packages.
- **Gate (clears by itself):** two production pins whose producers contain this milestone's commit (`git log origin/main --grep "pin platform images"`, then `git merge-base --is-ancestor <commit> <producer>`): the first moves both pins (step 4), the second should keep both (steps 2 and 3, with a Key Value and a Postgres client held across it). Production deploys `main` once its pin is 6 h old. Then t006's record and t009.

## Unblock check — 2026-11-18 23:35Z (the gate cleared; the next one is a deploy that keeps both pins)

- **Affected:** t006, then t009. **Source:** verified evidence, no user decision. Three production pins contain `d41bbb9c2`: `054f29128f64` (pin `0070f86ab`, 08:13Z), `1ec5007be0fa` (`c7f44bd87`, 14:44Z) and `1db506777e17` (`e5ace7826`, 21:22Z).
- **Result, DoD bullets 1 and 3 (met):** each of the three write-backs moved both SNI pins, and each was right to. `proxy-pin` read different `/kv-sni-proxy` and `/pg-sni-proxy` bytes every time, because each producer carried a non-test change to `lego/types/v1alpha1`: `936b61bab` (w6/m467), then w2's OIDC commits and `1ec5007be`, then `caf484f6e` and `d6366d33e`. The wait step waited for the new digest and passed in runs 44322231072 and 44395683569. Details: [t006 § Progress](done/t006.md#progress-2026-11-18-2335z-three-deploys-after-the-ship-steps-1-2-and-4-done-step-3-holding).
- **The failed wait in run 44262293900 is not this milestone's step.** It found the pinned image at once. Then `rollout status` hit its unchanged 300 s limit, because platform node `jfc55` was under `DiskPressure` from 08:15Z to 08:19Z, and its kubelet evicted each new SNI pod until about 08:24Z. No fix is needed here. Node disk pressure is a separate, recurring cluster issue.
- **Deviation found:** the shipping commit's own Argo sync rendered the bare `kv-sni-proxy:latest` and `pg-sni-proxy:latest` for about 3 minutes (06:53:22Z–06:56:11Z). `bex-operator` refreshed `main` before the root app updated the Application spec that adds the two pins, so node `m82n5` had both SNI pods in `ImagePullBackOff`. This cannot recur for these pins. ADR021 § 10 now records the two-step rule for any future placeholder.
- **Still open:** DoD bullet 2. No deploy so far could hold, so the held clients are now in place in `bex-canary`. Pod `m470-t006-kv-client` holds `client_id=258295` to `red-dbpf5q5me2kc73f2l0og.kv.bex.co:6379`, and pod `m470-t006-pg-client` holds `pg_backend_pid=187` to the free probe `dpg-dbv37l06mruc7390oomg.db.bex.co:5432`. Both connected at about 23:27Z, over TLS through the edge LB.
- **Gate (clears by itself, then worker):** the next production pin after `e5ace7826`, with a write-back that logs `kept` for both proxies. Production deploys once the 21:22Z pin is 6 h old, so not before 03:22Z on 2026-11-19. It keeps both pins only if no commit before it touches a linked package; none has as of 23:35Z. The held streams end at their 24 h cap at 23:27Z on 2026-11-19. Then t006 § Progress "Next", and t009.

## Unblock check — 2026-11-19 13:00Z (one deploy kept both pins, but no client was held across it; re-armed)

- **Affected:** t006, then t009. **Source:** verified evidence, no user decision. Three production pins came after `e5ace7826`: `826d32e42` (`7af50a28168d`, 03:48Z), `19faf35f3` (`4b2cebd0bd03`, 09:13Z) and `a91e9aeb3` (`dff52c596d13`, 11:30Z).
- **Write-backs.**
  - `826d32e42` and `a91e9aeb3` moved both SNI pins, each for non-test `lego/types/v1alpha1` changes: `568b388ce` … `e1c7d08cc`, then `cf5f56884`, `a7cb94de2` and `4a08adc46`.
  - `19faf35f3` logged `kept` for both. Both DaemonSets kept their generation, revision and pods across it: the six pods from 03:50Z ran until 11:34Z. This is the first deploy to keep both pins, and it holds the DaemonSet half of DoD bullet 2. Details: [t006 § Progress](done/t006.md#progress-2026-11-19-1300z-three-more-deploys-one-kept-both-pins-but-no-client-was-held-across-it).
- **The held clients did not reach it.** Both exited with `Error` at 23:40:18Z and 23:40:24Z on 2026-11-18, 13 minutes after connecting and before any of these deploys.
  - No SNI pod restarted at that time. A tenant node joining at 23:38:46Z is the only coincident event found; it is a correlation, not a cause.
  - Their pods were deleted at about 03:14Z. So DoD bullet 2's held-client half is still open. The dispatcher's rule applies: re-park, do not force.
- **Re-armed at 12:57Z, now reconnecting.** The probes now reconnect after a closed connection and log each one as `conn=<n>`. So they outlive node churn and the 24 h stream cap, and the check reads the log, not the pod phase. They hold `client_id=280163` and `pg_backend_pid=23773` (via `bex-pg-sni-proxy-t64mc`).
- **Gate (clears by itself, then worker).** The held-client deploy is the first production pin after `a91e9aeb3` whose write-back logs `kept` for both SNI pins. It is not the next one: `9fac4888a` and `05dbc4a35` changed `lego/types/v1alpha1` after `dff52c596d13`, and local builds show both binaries differ, so the first pin after 17:30Z on 2026-11-19 moves both. The worker then reads both probe logs across that kept deploy (t006 § Progress "Next"), deletes the probes, and closes t009.
- **Ordering.** w6/m515 changes both SNI binaries, so it ships only after that record, or once m470 is in `done/` (m515 README § Sequencing constraint).

## Unblock check — 2026-11-19 19:40Z (the gate cleared; criterion 2 met; closed)

- **Affected:** t006, t009. **Source:** verified evidence, no user decision. Two production pins came after `a91e9aeb3`: `aefb144dc` (`c81d2d447bd8`, 18:04Z) and `99bbc4239` (`c624a4f217ff`, 18:51Z).
- **Write-backs.** `aefb144dc` moved both SNI pins for `05dbc4a35`, `9fac4888a` and `fdf910308` (`lego/types/v1alpha1`), as predicted. `99bbc4239` logged `kept` for both: no commit between the two producers touches a linked package.
- **Result, DoD bullet 2 (met).** Both reconnecting probes held one connection across `99bbc4239`: kv `conn=2` `client_id=288604` and pg `conn=2` `pg_backend_pid=32855` (through `bex-pg-sni-proxy-224r9`), from 18:10Z to 19:36Z, with no `FAIL` between the 18:51:25Z write-back and the 18:55:33Z wait. Both DaemonSets kept generation (kv 816, pg 899), revision (817, 900) and the six pods from 18:08:51–18:09:23Z. Details: [t006 § Progress](done/t006.md#progress-2026-11-19-1940z-a-kept-deploy-with-both-clients-held-across-it-criterion-2-met-probes-deleted).
- **Cleanup.** Both client pods, the ConfigMap and both Secrets are deleted. `dpg-dbv37l06mruc7390oomg` answered `DELETE` 204, then `GET` 404.
- **Closed.** Every DoD bullet holds (t009). w6/m515's ship precondition, this held-client record, now holds.
