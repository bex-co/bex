# w6 · m640 — Background workers, cron jobs and pre-deploy jobs receive no private network traffic, as on Render

**Worker:** worker6 **Goal:** the pods of background workers, cron runs, pre-deploy and one-off jobs refuse connections from the workspace's other services, as Render's private network does, while they still send private traffic and platform access (SSH, metrics, logs) keeps working. **Status:** todo (t001–t003 and t007 done)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [Inventory production traffic into worker, cron and pre-deploy pods](done/t001.md) — **DONE** | 45m | — |
| t002 | [The operator labels every pod that receives no private network traffic](done/t002.md) — **DONE** | 1h | t001 |
| t003 | [A Cilium policy refuses tenant traffic to those pods; ADR022 and ADR041 say so](done/t003.md) — **DONE** | 45m | t002 |
| t004 | [Production check in bex-canary: a sibling cannot reach a worker or a cron run, and they still send](t004.md) | 45m | t003 |
| t005 | [Render parity](t005.md) | 30m | t004 |
| t006 | [Simplify](t006.md) | 20m | t005 |
| t007 | [Test coverage](done/t007.md) — **DONE** | 45m | t005 |
| t008 | [Closeout](t008.md) | 10m | t006, t007 |

## Definition of done

- On production, a paid web service in the same workspace and environment cannot open a TCP connection to a background worker's pod or its `<slug>-discovery` address, on any port, nor to a cron run's pod.
- The worker and the cron run still reach other services' internal addresses and datastores, and SSH into the worker, its metrics and its logs still work.
- A paid web or private service still receives private traffic (w9/m57), and a free one is still refused (w8/m130).
- The label has unit tests and the policy a gitops-validate guard; ADR022 and ADR041 describe the rule; ADR018's private-network row reads ✅ for receiving.

## Source + Goal linkage

- **Source:** `/pm-brainstorm one aspect of render parity to w6` 2026-11-25 (service networking: the private network and WebSockets, at `fe4d53addd`), item 2, filed by `/pmx them to w6`. `allow-same-namespace` (`sameNamespaceNetworkPolicy`, `lego/backend/internal/store/namespaces.go`) lets every pod of an environment reach every other on any port, and only free-plan pods are refused (`deny-free-plan-private-ingress`, w8/m130). A worker also has a reachable headless `<slug>-discovery` Service (`renderenv.HasDiscoveryService`, w6/m148). bex's own code states the rule: `ReceivesPrivateTraffic` in `lego/types/v1alpha1/app_types.go`, and `build_datastore_reach.go`'s "Workers, cron jobs, static sites and free web services receive none, as on Render". But only build reach and Blueprint checks use it, not the network.
- **Render:** background workers, cron jobs and workflows "can _send_ private network requests, but they can't _receive_ them" (render.com/docs/private-network, read 2026-11-25); background workers "don't receive any incoming network traffic" (render.com/docs/background-workers).
- **Recommendation, not a user decision:** keep a worker's `RENDER_DISCOVERY_SERVICE` and `<slug>-discovery` name. render.com/docs/environment-variables describes the variable as the way to discover all running instances of a scaled service, and the private-network page names only web and private services as receivers. Under ADR008 Render parity first the network follows the private-network page; whether Render sets the variable on a worker at all is unverified and stays open in ADR018.
- **Goal linkage:** Render parity first ([ADR008](../../../docs/ADR008-vision.md#render-parity-first)); tenant isolation ([ADR022](../../../docs/ADR022-tenant-isolation.md)).
- **Expected outcome:** a worker's debug, metrics or admin port is no longer open to the rest of its workspace, and bex's network does what its code and ADRs say.
- **Why now:** the rule is written in bex's code but not enforced in the network, and ADR018 has no row for it.
- **Render parity:** included: tenant-visible network behavior.

## Production inventory (t001, 2026-11-25)

Read-only, on the production app cluster (`scripts/fetch-app-kubeconfig.sh`), at about 21:00Z.

- **Destinations.** Production ran **no** pod this milestone denies: 0 `background_worker` and 0 `cron_job` Apps, and 0 Apps with a pre-deploy command, among its 27 Apps. bex runs no one-off job pods (Create Job answers 410, `jobs.ErrOneOffJobsUnsupported`). So no flow into a worker, cron run or pre-deploy pod existed to find, and **no production tenant relies on reaching one**.
- **Flows into workspace-labeled pods.** Hubble is on (`enable-hubble=true`), but each agent's ring buffer held only the last ~4 minutes (20:53:42–20:57:45Z). New TCP connections (SYN without ACK) across all 16 agents, by source:

  | Source | `app.bex.co/workspace` | Destination | Connections |
  | --- | --- | --- | --- |
  | kubelet probes (`reserved:host`) | no | App pods (80, 3000, 3100, 8000, 9000, 9090, 10000), Postgres :8000 | 994 |
  | Traefik (`traefik`) | no | web service pods :3000, :10000 | 459 |
  | tenant pods | yes | Key Value :6379 | 219 |
  | tenant pods | yes | Postgres :5432 | 34 |
  | Prometheus (`monitoring`) | no | Postgres exporter :9187 | 36 |
  | bex-api (`bex-system`) | no | Key Value :6379 | 18 |
  | CloudNativePG operator (`cnpg-system`) | no | Postgres :8000 | 10 |
  | tenant pods | yes | a paid web service in bex's own workspace :3100 | 2 |

- **Platform sources.** No pod outside a `tea-*` namespace carries `app.bex.co/workspace` (0 found), so the deny leaves the operator (port reads), the activator, Traefik, Prometheus, ssh-gateway, the listener reader, the egress meter and the log shipper alone. Of those, only probes, Traefik, the activator and the operator open connections to tenant pods at all: SSH is `pods/exec` through the API server and the kubelet (`sshgateway/exec.go`), the listener reader reads `/proc` and the log shipper the node's files. Inside tenant namespaces, the disk snapshot (`dskbak-*`), disk holder and Key Value backup Jobs **do** carry the workspace label, but they mount the volume (`/disk-snapshot backup` mounts the service's claim) or dial the Key Value store, never an App pod.
- **Cilium identity.** `cilium-config` sets no `labels` filter, so a new `app.bex.co/*` pod label is identity-relevant, as `app.bex.co/free-plan` already is.
