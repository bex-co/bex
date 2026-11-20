# w6 · m530 — The operator reads a pod's listeners on its node instead of connecting to every port, so its sweeps stop filling the Cilium conntrack tables

**Worker:** worker6 **Goal:** private-port discovery and bound-port detection learn which ports a tenant pod listens on from the pod's own socket table, read on its node, instead of a TCP connect to all 65,535 ports. No sweep then puts tens of thousands of entries into the Cilium conntrack table of the operator's node or of the swept pod's node, so the idle connections of every pod on those nodes keep their entries. **Status:** todo (t001–t005 done)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [Read a pod's listening ports from its node's `/proc`](done/t001.md) — **DONE** | 1h | — |
| t002 | [A listener-reader node agent and its DaemonSet](done/t002.md) — **DONE** | 2h | t001 |
| t003 | [The operator asks the pod's node and never sweeps a pod whose node has a reader](done/t003.md) — **DONE** | 2h | t002 |
| t004 | [Prove it on the local cluster](done/t004.md) — **DONE** | 1h30m | t003 |
| t005 | [Record it in ADR089, ADR018 and the operator guide](done/t005.md) — **DONE** | 45m | t003 |
| t006 | [Ship and verify on production](t006.md) | 1h30m | t004, t005 |
| t007 | [Read a day of conntrack GC on the operator's node and every tenant node](t007.md) | 30m | t006 |
| t008 | [Simplify](t008.md) | 15m | t006 |
| t009 | [Test coverage](t009.md) | 30m | t006 |
| t010 | [Closeout](t010.md) | 10m | t007, t008, t009 |

## Definition of done

- On the operator's node **and on every node that runs tenant App pods** (the `tenant-0` pool, any serving pool, and the sandbox pool, the nodes `bex-egress-meter` runs on), no Cilium conntrack GC pass across a day deletes half the table (`deleteRatio` below 0.5 in every `Conntrack garbage collector interval recalculated` line), and `cilium_bpf_map_pressure{map_name="ct4_global"}` stays below 0.5 in every sample of that day.
- Private-port discovery and bound-port detection still find listeners as before: their unit and envtest suites pass; on production every existing private service lists the same `openPorts` after the switch as before it, and a web service whose process binds another port than `PORT` goes live on it and lists an undeclared port above 10000 it also binds in `openPorts`. A loopback-only listener is still never found.
- The operator logs no full connect sweep of a pod whose node runs a Ready reader during the day read, and every fallback it counts is explained in t007.
- ADR089 § phase 2 item 1, ADR018 and the operator guide describe the reader and its privilege set, and `scripts/gitops-validate.sh` pins that set.

## Source + Goal linkage

- **Source:** inbox note [w6/156](../done/156.md), filed by w6/153 2026-11-20, promoted by `/loopx w6` the same day. w6/149's probes ([blocked/149.md](../blocked/149.md), "Finding: the operator's port sweeps evict a tenant pod's idle conntrack entries") showed the same fill on the swept pod's node, so the definition of done covers tenant nodes as well as the operator's.
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
