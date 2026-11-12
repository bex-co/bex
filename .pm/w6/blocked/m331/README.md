# w6 · m331 — Agent sandboxes run on a Hetzner bare-metal node under Kata

**Worker:** worker6 **Goal:** sandboxes run in microVMs on one Hetzner dedicated AX42-1 (ordered 2026-11-12; the planned EX44 was out of stock), so a build inside a sandbox runs at close to native speed and the scheduled routines finish, with the gVisor cloud node as overflow. **Status:** blocked — parked until 2026-12-12 (user, 2026-11-12: Hetzner's delivery is expected to take weeks). t001 done; t002 in progress: the AX42-1 was ordered 2026-11-12 and is not delivered yet. vSwitch #88064 is coupled to `bex` as `10.10.64.0/24`, CAPH accepts it, and the Robot webservice credentials are in `.env`.

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [Probe script and an idle gVisor baseline on today's node](done/t001.md) — **DONE** | 1h | — |
| t002 | [Order one AX42-1 and a vSwitch coupled to the `bex` network](t002.md) | 2h | — |
| t003 | [Bare-metal machine template with the node image's pinned artifacts](t003.md) | 3h | t002 |
| t004 | [The AX42-1 joins the cluster: CCM Robot support, vSwitch node IP, Cilium device and MTU, public-port firewall](t004.md) | 4h | t003 |
| t005 | [Kata and runsc installed at bootstrap, behind a trial taint](t005.md) | 3h | t004 |
| t006 | [Measure gVisor against each Kata backend and pick the VMM](t006.md) | 3h | t001, t005 |
| t007 | [One `sandbox` RuntimeClass whose handler each node maps to its runtime](t007.md) | 3h | t006 |
| t008 | [ADR042 sandbox controls and node exposure hold under Kata](t008.md) | 3h | t007 |
| t009 | [Go or no-go, recorded in ADR105](t009.md) | 30m | t006, t008 |
| t010 | [Sandboxes prefer the AX42-1 and overflow to the cloud node](t010.md) | 3h | t009 |
| t011 | [Production verification and ADR amendments](t011.md) | 1h | t010 |
| t012 | [Simplify](t012.md) | 15m | t011 |
| t013 | [Test coverage](t013.md) | 30m | t012 |
| t014 | [Closeout](t014.md) | 10m | t013 |

## Definition of done

- A scheduled `rt-*` routine session in production runs on the AX42-1 under Kata, completes, and opens its draft PR. Its session id and wall time are recorded in ADR105 beside the gVisor baseline.
- With the AX42-1 cordoned, a new agent session still starts on the gVisor cloud node, with no retry or code path in bex-api.
- From the internet, only SSH answers on the AX42-1's public address.
- ADR105 carries the measured comparison and the go or no-go verdict. On go, ADR042 D3, ADR053 and ADR002 § Network are amended.
- On no-go, the node, its pool and its CCM and Cilium settings are removed, the server's and vSwitch's cancellation is requested, and the milestone closes with that verdict.

## Source + Goal linkage

- **Source:** [ADR105](../../../../docs/ADR105-agent-sandbox-hetzner-bare-metal.md) (proposed 2026-11-11), written after all eight scheduled `rt-*` routines failed that day and a probe inside session `ags-dbqdl84o6rds73e87pj0` measured the gVisor cost. Revised the same day: the board now covers joining a dedicated server to a Cloud-network cluster, a runtime seam OpenSandbox can actually use, and a repeatable gate measure.
- **Goal linkage:** ADR008 pillar 5 (hosted agent sandboxes, re-opened 2026-07-27) and Render parity: Render runs each sandbox in its own Firecracker microVM. ADR042 D3 chose Kata first and fell back to gVisor only because the cloud nodes have no `/dev/kvm`.
- **Expected outcome:** process start and small-file I/O inside a sandbox drop from tens of milliseconds to near native, routines complete well inside the turn bound, and compute billed per run falls.
- **Why now:** the turn bound was raised from 30 minutes to 4 hours on 2026-11-11 as a stopgap. The slowness it hides is still there, and every agent session pays for it. `w22/blocked/m312` (Docker in a sandbox) also waits on the same node-pool choice.
- **Render parity task:** omitted. The change is the node pool and runtime under the sandbox, and no REST, GraphQL, MCP or dashboard field changes. One tenant-visible effect is checked in t008: `/tmp` surviving a pause under Kata, which narrows an ADR018 divergence.
- **External gates:**
  - t002 spends $119–$304 a month depending on the server chosen (ADR105 § What can be ordered today), plus a vSwitch, and needs Hetzner Robot access from the account owner.
  - t009 can end the milestone early with a no-go.
  - The single-host exception to the surge-first rule was accepted by the user on 2026-11-11 (ADR105 § Known limits).
  - The scripted replay as the gate measure is ADR105's default, and the user's to change (ADR105 § Decision step 3).
