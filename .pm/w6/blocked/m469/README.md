# w6 · m469 — An operator deploy that leaves the Key Value proxy's code alone keeps every tenant connection

**Worker:** worker6 **Goal:** a store's `kv-internal-proxy` rolls only when its own binary changes, not on every operator deploy, so tenant connections through a store's internal (and external TLS) URL survive the platform's routine releases, as on Render. **Status:** blocked (t001–t004, t006, t007 done; t005's shipping-deploy check passed on production, and its held probe client waits for the next production deploy, then t008)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [The operator keeps a proxy's image while the proxy binary is unchanged](done/t001.md) — **DONE** | 1h30m | — |
| t002 | [A proxy the operator has no record for keeps its image when that image carries the same binary](done/t002.md) — **DONE** | 1h30m | t001 |
| t003 | [Prove it on the local cluster](done/t003.md) — **DONE** | 45m | t002 |
| t004 | [Record it in ADR021 and file the SNI-proxy half](done/t004.md) — **DONE** | 30m | t002 |
| t005 | [Ship and verify on production](t005.md) | 1h | t003, t004 |
| t006 | [Simplify](done/t006.md) — **DONE** | 15m | t005 |
| t007 | [Test coverage](done/t007.md) — **DONE** | 30m | t005 |
| t008 | [Closeout](t008.md) | 10m | t006, t007 |

## Definition of done

- The production deploy that ships this milestone creates no new ReplicaSet for any store's `<id>-internal` Deployment, and every proxy pod keeps its name and start time. The one exception is a producer whose `/kv-internal-proxy` differs from the running one. Then t005 names the commit that changed it and says why that one roll could not be avoided.
- On production, a client holds one connection through a store's internal URL across a later operator deploy whose `/kv-internal-proxy` is byte-identical. Afterwards that connection still answers `PING` with the same `CLIENT ID`, and the proxy Deployment still runs the earlier digest.
- A test fails if a manager with a different image but the same proxy binary changes the proxy's pod template. A test also fails if a changed binary does not roll the proxy.
- Enabling internal authentication still rolls the proxy and closes its unauthenticated connections (w6/m463). A plan or credential change still rolls the proxy, now on the held image.
- The SNI-proxy half (`bex-kv-sni-proxy`, `bex-pg-sni-proxy`) is filed as its own item with the evidence below.
- ADR021 records the rule.

## Source + Goal linkage

- **Source:** promoted from inbox note [w6/140](../../done/140.md) on 2026-11-18. w6/m463 t005 filed it after it saw every store's proxy roll on the 01:32Z deploy with only `image` changed. The loopx dispatch picked it ahead of w6/138 as a live production defect.
- **Goal linkage:** ADR008 § Render parity first. Render breaks a Key Value store's connections when the customer changes the store (a plan change, enabling internal authentication: "Render restarts your Key Value instance", [Key Value](https://render.com/docs/key-value)), not when Render ships its own platform. No ADR018 row signs off this divergence.
- **Expected outcome:** of the 11 production deploys on 2026-11-17, at most the 2 that changed the proxy binary would have rolled the proxies (see Findings). Long-lived Redis clients (pub/sub, blocking commands, pools) stop seeing in-flight commands fail on every release.
- **Why now:** production deploys several times a day (11 on 2026-11-17). Each deploy closes every tenant connection to every store, including the Discourse service `beancount-forum` (`srv-d9nqg9dcavls73fp8m2g`, namespace `tea-d98210cbbpdc73dcrkvg`) on `red-d9p49kdrtmes73c34ovg:6379`. Builds on w6/m456 (the proxy fronts every store), w6/m463 (it stays in front of internal-auth stores) and w6/139 (backup pods are never an endpoint), all closed.
- **Render parity task omitted:** the change is operator-internal. No REST, GraphQL, MCP or dashboard field or message changes. The tenant-visible effect, a connection surviving a platform deploy, is checked on production in t005.

## Findings at promotion (2026-11-18, production read-only and GHCR)

- **The build is already reproducible.** `/kv-internal-proxy` was hashed out of six production images (`skopeo inspect` plus a `curl` of layer 19, the `COPY /workspace/kv-internal-proxy` layer, from GHCR). Pins `9c79201b4`, `5e0fb5ad8` and `336b647b1` (07:47Z, 09:41Z, 10:34Z) carry `88fdde681f3f0610…`. `fc4fbf28a` and `e36296ae8` (12:08Z, 18:33Z) carry `43e24e9a1cabfdf2…`. `359aee37d` (01:28Z on 11-18, `bex-operator@sha256:b4104643…`, running now) carries `d1a7d7d0e8ecc328…`. The hash changes exactly at the two commits that changed the proxy, `0eed45bcb` (m456) and `ef091b640` (m463). The binary links only the standard library and `kvinternalauth`, and the `lego/` build context has no VCS stamp, so no build change is needed. Of the 11 proxy ReplicaSets each store got on 2026-11-17 (04:41Z … 01:32Z), only 2 carried a different binary (12:10Z and 01:32Z). Five of the other nine pins were not sampled, but no proxy, toolchain or `go`-line input changed between them.
- **The Discourse crash was a Valkey restart, not a proxy roll.** `beancount-forum` reads `DISCOURSE_REDIS_URL` = `red-d9p49kdrtmes73c34ovg:6379`, through the proxy. Its only restart in 5d20h, exit 1 at 12:11:07Z, followed the Valkey pod's own restart. StatefulSet revision `red-d9p49kdrtmes73c34ovg-595994744` was created at 12:10:07Z, pod `-0` at 12:10:10Z, and `valkey` started at 12:10:46Z. That is m456's one-time first-rollout restart (m456 t006, fixed by `cba5b9b5a`). The proxy rolled at the same moment and at ten other deploys, and Discourse survived those ten. So the recurring roll closes every connection and fails in-flight commands, but did not crash this service by itself. The 12:10Z deploy also changed the proxy's binary and template (m456), so this milestone would not have prevented that roll.
- **SNI proxies.** `bex-kv-sni-proxy` and `bex-pg-sni-proxy` run 3 replicas each: `RollingUpdate`, `maxUnavailable: 1`, a 30 s grace period, and `hostPort`, so a pod cannot surge. `bex-kv-sni-proxy` is at revision 809. On `SIGTERM` both close the listener and return from `main` (`cmd/kv-sni-proxy/main.go`, `cmd/pg-sni-proxy/main.go`), which closes every stream. Streams are already capped at 24 h lifetime and 1 h idle. Their binaries link `k8s.io` and controller-runtime and change more often: identical in pins `336b647b1`, `5e0fb5ad8` and `fc4fbf28a`, different in `9c79201b4`, `e36296ae8` and `359aee37d`. Kustomize gives them their image from the `controller=` pin (`deploy/gitops/base/bex.yaml`), so only a hold in `deploy.yml` or a separate content-addressed image can keep it. `deploy.yml`'s "wait for PROXY-compatible datastore proxies" step requires the new operator digest on both DaemonSets and must change with it.

## Design constraints

- **Do not change the proxy binary in this milestone.** Leave `lego/operator/internal/kvinternalauth/`, `lego/operator/cmd/kv-internal-proxy/`, the `go` line of `lego/operator/go.mod`, and the builder image and kv-internal-proxy build line of `lego/Dockerfile` alone. Otherwise this milestone's own deploy rolls every proxy once. If a change there becomes unavoidable, t005 and the result say so plainly.
- **Hold the image, not the template.** Env, annotation and resource changes (internal auth, credential revision, plan limit) still roll the proxy, now with the held image, which runs a byte-identical binary.
- **Record the hash on the Deployment's own `metadata.annotations`, never on the pod template.** A template annotation would roll every proxy once when this ships.
- The held digest can be weeks old. GHCR keeps every `bex-operator` version (no workflow in `.github/workflows` deletes package versions).

## Progress — 2026-11-18 (parked on its production deploy)

- **Shipped:** the hold (t001), registry adoption (t002), the local proof (t003: an unrelated operator build kept the proxy's pod and a held client's `CLIENT ID`, a proxy code change rolled it once, and internal auth rolled it on the held image), ADR021 § 10 with follow-ups [w6/141](../../done/141.md) and [w6/142](../../done/142.md) (t004), `/simplify` (t006) and the reverted-rule tests (t007).
- **Expected on production:** the running proxies' image `bex-operator@sha256:b4104643…` carries `/kv-internal-proxy` `d1a7d7d0e8ecc328…` (read by `registry.ImageFileSHA256` from GHCR), and this tree builds the same bytes for linux/amd64 in the pinned builder. So the shipping deploy should record the hash on all three stores' `<id>-internal` Deployments and create no ReplicaSet. Recording the hash bumps each Deployment's generation, which is not a roll.
- **Gate (clears by itself):** a production pin whose producer contains this milestone's commit (`git log origin/main --grep "pin platform images"`, then `git merge-base --is-ancestor <commit> <producer>`). Production deploys `main` once its pin is 6 h old. Then t005: the before/after record, a held client across a later deploy, and t008.

## Unblock check — 2026-11-18 23:14Z (the gate cleared; the next one is the held client's deploy)

- **Affected:** t005, then t008. **Source:** verified evidence, no user decision. Every production pin since 08:13Z contains `be2914a57`: `054f29128f64` (pin `0070f86ab`, 08:13Z), `1ec5007be0fa` (`c7f44bd87`, 14:44Z) and `1db506777e17` (`e5ace7826`, 21:22Z) all pass `git merge-base --is-ancestor be2914a57 <producer>`.
- **Result, DoD bullet 1 (met):** the shipping deploy (`054f29128f64`) and both later ones created no ReplicaSet for any of the three stores' `<id>-internal` Deployments. Each proxy pod kept its name and its 01:32Z start time, and each still runs `bex-operator@sha256:b4104643…`. All three producers carry the same `/kv-internal-proxy` (`d1a7d7d0e8ecc328…`), so no roll was due. The manager recorded that hash on each Deployment's own annotations at 08:15Z. Details: [t005 § Progress](t005.md#progress--2026-11-18-2314z-steps-12-done-step-3-holding).
- **Still open:** DoD bullet 2 needs a client already held when a deploy happens, and every deploy so far came before the probe existed. Probe `red-dbv30j86mruc7390onu0` in `bex-canary` and pod `m469-t005-client` have held `client_id=25` since 23:11:16Z. t005 step 4 adds a production check of DoD bullet 4: internal auth rolls the probe onto the held digest.
- **Gate (clears by itself, then worker):** the next production pin, the first `pin platform images` commit after `e5ace7826`. Production deploys once the 21:22Z pin is 6 h old, so not before 03:22Z on 2026-11-19, and `dc41a1a90` and `ab56ddb2a` are already waiting. Its producer must keep `/kv-internal-proxy` at `d1a7d7d0…`, which `proxy-pin -file /kv-internal-proxy` checks. Then t005 § Progress "Next" and t008.
