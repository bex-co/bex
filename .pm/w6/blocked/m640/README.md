# w6 · m640 — Background workers, cron jobs and pre-deploy jobs receive no private network traffic, as on Render

**Worker:** worker6 **Goal:** the pods of background workers, cron runs, pre-deploy and one-off jobs refuse connections from the workspace's other services, as Render's private network does, while they still send private traffic and platform access (SSH, metrics, logs) keeps working. **Status:** blocked (t001–t004, t007 and t009 done; t010 waits for a production pin carrying t009's `201a068b86`)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [Inventory production traffic into worker, cron and pre-deploy pods](done/t001.md) — **DONE** | 45m | — |
| t002 | [The operator labels every pod that receives no private network traffic](done/t002.md) — **DONE** | 1h | t001 |
| t003 | [A Cilium policy refuses tenant traffic to those pods; ADR022 and ADR041 say so](done/t003.md) — **DONE** | 45m | t002 |
| t004 | [Production check in bex-canary: a sibling cannot reach a worker or a cron run, and they still send](done/t004.md) — **DONE** | 45m | t003 |
| t009 | [A cron run carries the workspace label, and a restored template its plan's network labels](done/t009.md) — **DONE** | 45m | t004 |
| t010 | [Production check: a cron run cannot reach a worker or a free web service, and no free-plan pod lacks the send-only label](t010.md) | 30m | t009 |
| t005 | [Render parity](t005.md) | 30m | t004, t010 |
| t006 | [Simplify](t006.md) | 20m | t005 |
| t007 | [Test coverage](done/t007.md) — **DONE** | 45m | t005 |
| t008 | [Closeout](t008.md) | 10m | t006, t007 |

## Definition of done

- On production, a paid web service in the same workspace and environment cannot open a TCP connection to a background worker's pod or its `<slug>-discovery` address, on any port, nor to a cron run's pod.
- The worker and the cron run still reach other services' internal addresses and datastores, and SSH into the worker, its metrics and its logs still work.
- A paid web or private service still receives private traffic (w9/m57), and a free one is still refused (w8/m130).
- A cron run is refused like any other sibling: it cannot reach a worker's pod or a free web service (t004 found it could, t009).
- The label has unit tests and the policy a gitops-validate guard; ADR022 and ADR041 describe the rule; ADR018's private-network row reads ✅ for receiving.

## Unblock — 2026-11-25 (`/loopx w6`)

- **Affected:** t004, then t005, t006 and t008.
- **Source:** verified evidence. The newest pin on `origin/main`, `043827a8e8` (`chore(deploy): pin platform images to 2bd6e1f415d3`), carries `8a53318532`: `git merge-base --is-ancestor 8a53318532 2bd6e1f415d3` holds.
- **Next:** t004's production check in `bex-canary` (disposable fixtures), then t005, t006 and t008 as their tasks say. No user gate remains.

## Production check (t004, 2026-11-26)

On pin `2bd6e1f415d3` (operator `bex-operator@sha256:f317e63a…`, the digest the pin names), in `bex-canary` with the QA user's session, a run-minted API key and a run-registered SSH key, all three removed afterwards. `deny-send-only-private-ingress` was `Valid=True` beside `deny-free-plan-private-ingress`. Fixtures, all disposable, `busybox:1.37` image services: Starter web `w6-m640-web` (`srv-dc3o8u4uikjc73dfgsq0`, httpd on 10000), Starter background worker `w6-m640-worker` (`srv-dc3o8v4uikjc73dfgss0`, httpd on 8080, a pre-deploy command that serves 8080 for 150 s, and a loop that sends every 30 s), Starter cron job `w6-m640-cron` (`srv-dc3o8v83teic73bl0ri0`, two manual runs that serve 8080 and send for 6 minutes), and a free Postgres `w6-m640-pg` (`dpg-dc3oaog3teic73bl0sq0`). The free web control was `hello-go`, bex's own fixture.

- **Labels.** The worker's, the cron run's and the pre-deploy pod carried `app.bex.co/private-network: send-only`; the web pod carried none. Every free-plan pod on production carried it too (3 of 3 running).
- **Refused, from the web service's Shell (`ssh srv-…@ssh.bex.co`).** Each `nc -z -w 5` timed out after 5 s, and each target answered on its own loopback: the pre-deploy pod's IP on 8080 and 22; the worker's pod IP on 8080, 22 and 9090; `w6-m640-worker-discovery:8080` (it resolved to the worker's pod IP) and `wget` to it; the cron run's pod IP on 8080, 22 and 9090, and `wget` to it. `hello-go:10000` (free) timed out too.
- **Refused, from the worker (SSH).** `hello-go:10000` and the cron run's pod on 8080 timed out.
- **They still send.** The worker reached `w6-m640-web:10000` and the web pod's IP (a paid web service still receives, w9/m57), the Postgres at its internal host (`SSLRequest` answered `S`) and a Key Value (`PING` answered `+PONG`). The cron runs reached the web service's internal address, the Postgres and the Key Value.
- **Platform access.** SSH into the worker opened a shell and ran the probes above. REST `GET /v1/logs` returned the worker's lines and `GET /v1/metrics/cpu` and `/memory` its series. Node addresses (kubelet 10250, 9100) and 169.254.169.254 timed out from both the worker and a cron run.
- **Found: a cron run is not a tenant source to the denies.** Its pods carried no `app.bex.co/workspace` label (the cron template sets the App and environment labels only), so both ingress denies, which know a tenant source by that label, let it through: from a cron run, `wget` to the worker's pod IP and to `w6-m640-worker-discovery:8080` returned `worker-8080`, and to `hello-go:10000` (free) returned 200 at once. No other tenant workload in a `tea-*` namespace lacked the label (an ACME solver and agent sandboxes aside). t009 fixes it.
- **Found: a restored template keeps a record's missing label.** `qa-20261123-w8-412-node` (disposable, a leftover free web service of another run) had a deploy canceled over its served release, so the operator restores that release's recorded template, verbatim, on every pass. Its record predates the send-only label: woken through its public URL at 01:14Z, it ran a pod with `app.bex.co/free-plan: "true"` and no `send-only`. The free-plan deny still refuses it, so `deny-free-plan-private-ingress` cannot retire yet (t006). t009 fixes the restore.
- **Cleanup.** `DELETE` answered 204 for the three services and the Postgres, each `GET` then 404, and no object of theirs remained in the namespace by 01:25Z. The SSH key was deleted, the API key revoked and the QA session logged out.

## Source + Goal linkage

- **Source:** `/pm-brainstorm one aspect of render parity to w6` 2026-11-25 (service networking: the private network and WebSockets, at `fe4d53addd`), item 2, filed by `/pmx them to w6`. `allow-same-namespace` (`sameNamespaceNetworkPolicy`, `lego/backend/internal/store/namespaces.go`) lets every pod of an environment reach every other on any port, and only free-plan pods are refused (`deny-free-plan-private-ingress`, w8/m130). A worker also has a reachable headless `<slug>-discovery` Service (`renderenv.HasDiscoveryService`, w5/m148, `3cc1d7bd70`). bex's own code states the rule: `ReceivesPrivateTraffic` in `lego/types/v1alpha1/app_types.go`, and `build_datastore_reach.go`'s "Workers, cron jobs, static sites and free web services receive none, as on Render". But only build reach and Blueprint checks use it, not the network.
- **Render:** background workers, cron jobs and workflows "can _send_ private network requests, but they can't _receive_ them" (render.com/docs/private-network, read 2026-11-25); background workers "don't receive any incoming network traffic" (render.com/docs/background-workers).
- **Recommendation, not a user decision:** keep a worker's `RENDER_DISCOVERY_SERVICE` and `<slug>-discovery` name. render.com/docs/environment-variables describes the variable as the way to discover all running instances of a scaled service, and the private-network page names only web and private services as receivers. Under ADR008 Render parity first the network follows the private-network page; whether Render sets the variable on a worker at all is unverified and stays open in ADR018.
- **Goal linkage:** Render parity first ([ADR008](../../../../docs/ADR008-vision.md#render-parity-first)); tenant isolation ([ADR022](../../../../docs/ADR022-tenant-isolation.md)).
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
