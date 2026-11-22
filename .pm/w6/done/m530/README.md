# w6 · m530 — The operator reads a pod's listeners on its node instead of connecting to every port, so its sweeps stop filling the Cilium conntrack tables

**Worker:** worker6 **Goal:** private-port discovery and bound-port detection learn which ports a tenant pod listens on from the pod's own socket table, read on its node, instead of a TCP connect to all 65,535 ports. No sweep then puts tens of thousands of entries into the Cilium conntrack table of the operator's node or of the swept pod's node, so the idle connections of every pod on those nodes keep their entries. **Status:** done

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [Read a pod's listening ports from its node's `/proc`](done/t001.md) — **DONE** | 1h | — |
| t002 | [A listener-reader node agent and its DaemonSet](done/t002.md) — **DONE** | 2h | t001 |
| t003 | [The operator asks the pod's node and never sweeps a pod whose node has a reader](done/t003.md) — **DONE** | 2h | t002 |
| t004 | [Prove it on the local cluster](done/t004.md) — **DONE** | 1h30m | t003 |
| t005 | [Record it in ADR089, ADR018 and the operator guide](done/t005.md) — **DONE** | 45m | t003 |
| t006 | [Ship and verify on production](done/t006.md) — **DONE** | 1h30m | t004, t005 |
| t007 | [Read a day of conntrack GC on the operator's node and every tenant node](done/t007.md) — **DONE** | 30m | t006 |
| t008 | [Simplify](done/t008.md) — **DONE** | 15m | t006 |
| t009 | [Test coverage](done/t009.md) — **DONE** | 30m | t006 |
| t010 | [Closeout](done/t010.md) — **DONE** | 10m | t007, t008, t009 |

## Definition of done

- On the operator's node **and on every node that runs tenant App pods** (the `tenant-0` pool, any serving pool, and the sandbox pool, the nodes `bex-egress-meter` runs on), no Cilium conntrack GC pass across a day deletes half the table (`deleteRatio` below 0.5 in every `Conntrack garbage collector interval recalculated` line), and `cilium_bpf_map_pressure{map_name="ct4_global"}` stays below 0.5 in every sample of that day.
- Private-port discovery and bound-port detection still find listeners as before: their unit and envtest suites pass; on production every existing private service lists the same `openPorts` after the switch as before it, and a web service whose process binds another port than `PORT` goes live on it and lists an undeclared port above 10000 it also binds in `openPorts`. A loopback-only listener is still never found.
- The operator logs no full connect sweep of a pod whose node runs a Ready reader during the day read, and every fallback it counts is explained in t007.
- ADR089 § phase 2 item 1, ADR018 and the operator guide describe the reader and its privilege set, and `scripts/gitops-validate.sh` pins that set.

## Source + Goal linkage

- **Source:** inbox note [w6/156](../../done/156.md), filed by w6/153 2026-11-20, promoted by `/loopx w6` the same day. w6/149's probes ([blocked/149.md](../../blocked/149.md), "Finding: the operator's port sweeps evict a tenant pod's idle conntrack entries") showed the same fill on the swept pod's node, so the definition of done covers tenant nodes as well as the operator's.
- **Goal linkage:** Render parity and platform reliability. Render "scans the running container for an HTTP listener on 0.0.0.0" (w8/177); reading the container's own socket table is that scan. Every pod on an affected node pays for the sweeps today: on the operator's node bex-api, bex-db, Kratos, Hydra, OpenFGA, the SSH gateway and Argo CD; on a tenant node every tenant pod. An idle pooled connection to a Service with several backends is reset at its next use (w6/153), and a tenant client that only receives, a `SUBSCRIBE`, a `LISTEN` or server-sent events without keepalive, hears nothing more and gets no error (w6/149).
- **Expected outcome:** sweeps add a few conntrack entries per scan (one HTTP request to the node's reader, plus the HTTP check of the ports found) instead of up to 65,535. Detection gets a complete answer at once, where a sweep of a pod that drops probes to closed ports never finishes in its 15 s budget.
- **Why now:** measured on production at 18:29Z 2026-11-20, over the previous 6 h of each Cilium agent's log: the operator's node `dpjxh` ran 408 GC passes, 105 of them deleting at least half of its 131,072-entry TCP table; tenant nodes `hbkdk` 212 and 49, `59ksr` 181 and 31; `jfc55`, a platform node with no operator, none. One `hbkdk` pass at 18:28:08Z deleted 0.997 of the table after a 6m45s interval. Only 22 web and 4 private services (plus 3 of the default type) are swept. The gateway's retry (w6/153, `dfcd4e23c`) covers one client; every other client on those nodes is still exposed.
- **Render parity task omitted:** no REST, GraphQL, MCP or dashboard surface changes. What the user sees, `openPorts` and the detected port, should read the same or more complete; t006 checks both live.

## Direction chosen, and why the others were rejected (triage, 2026-11-20)

The note offered three directions. This milestone takes direction 1.

- **1. Read listeners without connecting: chosen.** A pod's sockets are in `/proc/<pid>/net/tcp` and `tcp6` of any of its processes, readable from the node's PID namespace. The table names each listener's local address, so "binds the pod address, not loopback" is answered as the connect answered it: `0.0.0.0`, `::` or the pod IP count, `127.0.0.0/8`, `::1` and `::ffff:127.0.0.0/104` do not. `internal/healthprobe/openport.go` already parses this table inside the pod for Render's default health check (w4/m350).
  - **Feasibility, checked locally:** on Docker's Linux 7.0.12 kernel, a container with only `--pid=host`, uid 65534, `--cap-drop ALL` and a read-only root read another root container's `/proc/<pid>/cgroup` (its container id) and `/proc/<pid>/net/tcp` (its `0.0.0.0:8123` and `127.0.0.1:8124` listeners, state `0A`). Its `/proc/<pid>/ns/net` and `/proc/<pid>/environ` were refused (ptrace-guarded). So the reader needs `hostPID` and nothing else: no capability, no root, no host mount, no service-account token.
  - **Not yet checked:** production's Ubuntu 6.8 kernel under containerd's AppArmor profile, and a pod with `hostUsers: false` (every render-v1 runtime pod on production, w6/m151). t001 and t006 check both.
- **2. Pace the sweep under a budget: rejected.** Expired entries stay in the table until the next GC pass, and Cilium lengthens its GC interval to minutes while the table is quiet (`hbkdk`: a 6m45s interval before the 0.997 pass). One sweep of a pod that refuses its closed ports is 65,535 entries within seconds, half the table. Holding every pass under half would spread one sweep over many minutes per service, so a listener opened after readiness would take far longer to publish than the 2 to 16 minutes it takes now, and the 15 s detection budget would end every full sweep partial.
- **3. Raise `bpf-ct-global-tcp-max`: rejected, and not to be used as a stopgap before 2026-11-23.** It moves the cliff, it does not remove the fill, and it restarts every Cilium agent and recreates the maps. That would cut the windows the w6/149, w6/152 and w6/153 probes in `bex-canary` are recording (they stop by themselves by 2026-11-23T00:00Z).

## Parked 2026-11-20: pin must carry `84e2bfe08`

- **Shipped in `84e2bfe08`** (t001–t005): the shared `/proc/net/tcp{,6}` parser (`internal/procnet`), the reader (`internal/listeners`, `cmd/listener-reader`, in the image), its manifests (`config/listener-reader/`: DaemonSet, token-less ServiceAccount, operator-only NetworkPolicy), `deny-tenant-ingress` leaving the reader out, the operator's client and the reader-first selection with its counter `bex_port_listener_reads_total{source,result}`, the `gitops-validate.sh` and `image-pin-validate.sh` guards, the `mock-cluster.sh` namespace-label fix, and the docs. Local proof in [t004](done/t004.md).
- **Not yet on production:** the DaemonSet is not in `config/default` (t002 § Deviation). Argo applies `config/` at the push while the image moves only with the pin, so listed now it would crash-loop on every tenant node until a pin carried `/listener-reader`. Until then the new operator finds no reader pod on any node and sweeps exactly as before, each sweep logged once per node and interval and counted as `source="sweep"`.
- **Next, once production's pin carries `84e2bfe08`:**
  1. Add `- ../listener-reader` to `lego/operator/config/default/kustomization.yaml` (beside `- ../egress-meter`, with a one-line comment), run `bash scripts/gitops-validate.sh`, and ship it alone. Argo applies it at the push on the already-pinned image, so no second pin is needed. Check `kubectl -n bex-system get ds bex-listener-reader` shows one Ready pod per node egress-meter runs on, and `kubectl get ns bex-system --show-labels` keeps `pod-security.kubernetes.io/enforce=privileged` (the local cluster had lost it, t002).
  2. Then t006 steps 2–5 (re-read the four private services' `status.runtimePorts` first, read-only), and park on t007's `date gate:` 24 h after t006's start. t008 and t009 follow t006 (t009 lists what ship 1 already covers).
- **Effect on the probes in `bex-canary`:** shipping `84e2bfe08` triggers a normal deploy. The operator rolls, bex-api and the SSH gateway restart as on every deploy (w6/152 waits for exactly that gateway release; w6/153 counts a release's 3,600 s drain bound as a pass), and neither SNI proxy moves: `go list -deps` of `cmd/pg-sni-proxy` and `cmd/kv-sni-proxy` names none of the changed packages, so their held pins stay and w6/149's streams are untouched. Nothing restarts Cilium. The `deny-tenant-ingress` selector change keeps every existing bex-system pod selected with the same rules. Step 1's DaemonSet adds one pod per tenant node and restarts no tenant pod.

## Unblocked 2026-11-20: production carries `84e2bfe08`

- **Affects:** t006 (and, through it, t007's day read).
- **Source: verified evidence, no user decision.** The newest pin on `origin/main`, `7474e1c6a` (`chore(deploy): pin platform images to 820edf28d22d`, 22:08Z), contains `84e2bfe08` (`git merge-base --is-ancestor 84e2bfe08 820edf28d22d`). On production `bex-controller-manager` runs that image (`ghcr.io/bex-co/bex-operator@sha256:ca059b40…`, on `bex-platform-k9wb4-dpjxh` since about 22:10Z), and with no reader pod anywhere it sweeps every node, as designed: `bex_port_listener_reads_total{source="sweep",result="ok"}` read 59 after 16 minutes, and "no listener reader on the pod's node" was logged 25 times in 30 minutes.
- **Production pre-flight of the reader, 22:26Z (what t001 could not check):** a throwaway pod in `bex-system` with the DaemonSet's exact privilege set (`hostPID`, uid 65532, every capability dropped, read-only root, seccomp `RuntimeDefault`, no token) and the pinned image's `/listener-reader`, on `bex-tenant-0-wldb9-hbkdk` (Ubuntu 24.04, kernel 6.8.0-149, containerd 2.3.3 under its AppArmor profile), labeled apart from the reader so the operator never used it. `/readyz` answered 200; three `hostUsers: false` tenant pods read `[3000]`, `[3000]` and `[8000]`, the `status.runtimePorts` of `beancount-cms-v2`, `block-eden-mono` and a customer service; a wrong address answered 409 and an unknown pod 404. The pod was deleted at 22:26:27Z.
- **Before the line, read-only at 22:26Z:** the four private services list the 18:31Z ports unchanged (`crypto-price-clickhouse` `[8123,9000,9004,9005,9009]`, `metrics-loki` `[3100,9096]`, `metrics-prometheus` `[9090]`, `verify-iso-b` `[80]`); two newer private services in `tea-da2isimlm39c739m4ofg`, `beancount-gitea` and `beancount-ledger`, are still `Building` with no ports. `bex-system` keeps `pod-security.kubernetes.io/enforce=privileged`. `ct4_global` pressure read 0.758 on the operator's node `dpjxh` and 0.003 on `hbkdk` (22:28Z, `cilium-dbg metrics list -p bpf_map_pressure`). The SNI proxies are mid-roll from the 22:08Z pin itself (old `b123308a` pods still draining beside new `ca059b40` ones); the listener-reader line changes neither proxy's binary, so the next write-back holds both pins (w6/m470).
- **Observability facts for t007:** Prometheus's `bex-operator` keep-list (`deploy/gitops/base/prometheus.yaml`) does not keep `bex_port_listener_reads_total`, and the Cilium agents serve no Prometheus endpoint (`cilium-config` has `enable-metrics=true` but no `prometheus-serve-addr`; adding one restarts every agent, ruled out before 2026-11-23). So the day's sweeps are read from the operator's log line, which names the node, and from direct scrapes of each operator pod; `ct4_global` pressure is read in each agent (`cilium-dbg metrics list`), and the agent's GC lines are the day's record.
- **Next:** add `- ../listener-reader` to `lego/operator/config/default/kustomization.yaml`, run `bash scripts/gitops-validate.sh`, ship it alone, then t006 steps 2–5 once Argo has applied it.

## Parked again 2026-11-20: date gate 2026-11-21T22:46Z

- **t006 done on production** ([record](done/t006.md#record-production-2026-11-20)). The reader has run on all eight nodes egress-meter runs on since 22:45Z. Every private service and three web services read through it the same ports their `openPorts` list. A `bex-canary` web fixture went live on its bound 8000 with `PORT` 10000 and listed its undeclared 12345, never its loopback 7000. Over the hour from 22:46Z the operator read only through readers, with no sweep and no failed read. The operator's node `ct4_global` pressure went from 0.758 at 22:28Z to 0.018.
- **t008 and t009 done:** `0e7d0dec6` (the `/simplify` pass), and the loopback rule's mutation run ([t009](done/t009.md)).
- **Gate:** t007 is a day's read from 2026-11-21T22:46Z back to 22:46Z the day before, so `date gate: 2026-11-21T22:46Z`. Then t007 reads every agent's GC lines, spot pressure, and the operator's fallbacks in Loki, as its context says. The next pin, at about 04:08Z or later, rolls the operator and the readers inside that day and carries `0e7d0dec6`. Then t010 closes the milestone if the definition of done holds.

## Unblocked 2026-11-21: the date gate passed

- **Affects:** t007 and t010.
- **Source: verified evidence, no user decision.** `date gate: 2026-11-21T22:46Z` had passed when the read began at 22:52Z, and the gate names no pin. Nothing else gated the two tasks.
- **Next:** t007 reads the day, and t010 closes the milestone if the definition of done holds.

## Closed 2026-11-21: the definition of done holds

- **Conntrack ([t007](done/t007.md#record-production-read-2026-11-21-2252z-to-2301z)).**
  - **Every node read was clean.** Across the day from 2026-11-20T22:46Z, no GC pass on the operator's node or on any tenant node that could be read deleted half a table, and no pass there deleted even a quarter.
  - **Operator's node.** `dpjxh` held the whole day: 153 passes, at most 0.049 logged, against 105 of 408 passes at or above 0.5 in the 6 h before the switch.
  - **Tenant nodes.** The six `tenant-0` nodes left after w6/m547's pool roll were read from their joins, about 12 h each, with at most 0.044. So was the new sandbox node.
  - **Pressure.** `ct4_global` read at most 0.095 on any node at 22:57Z, and 0.031 on the operator's node.
  - **Nodes that left with their logs.** The old pool's first 12 h, nine short-lived nodes and the replaced sandbox node took their agents' logs with them. For those, the operator's own log stands in: Loki shows no sweep of any pod on any node all day.
  - **The one pass at or above 0.5.** It was on the control plane `ff5b6`, which runs no operator and no tenant pod. It is most likely CoreDNS lookups expiring after the pool roll, and it is recorded in t007.
- **Fallbacks.** There were no sweeps. Two reads failed, both at a pin's roll, through a reader the DaemonSet was deleting. Each kept the last record and swept nothing. They are filed as w6/166.
- **Listeners still found as before.** This was checked on production in t006: every private service listed the same ports through the reader, and the web fixture went live on its bound port, listed its undeclared port, and never its loopback-only one. The suites pass. ADR089, ADR018, the operator guide and `scripts/gitops-validate.sh` were updated in t005.
