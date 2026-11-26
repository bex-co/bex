# w6 · m640 — Background workers, cron jobs and pre-deploy jobs receive no private network traffic, as on Render

**Worker:** worker6 **Goal:** the pods of background workers, cron runs, pre-deploy and one-off jobs refuse connections from the workspace's other services, as Render's private network does, while they still send private traffic and platform access (SSH, metrics, logs) keeps working. **Status:** done

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [Inventory production traffic into worker, cron and pre-deploy pods](done/t001.md) — **DONE** | 45m | — |
| t002 | [The operator labels every pod that receives no private network traffic](done/t002.md) — **DONE** | 1h | t001 |
| t003 | [A Cilium policy refuses tenant traffic to those pods; ADR022 and ADR041 say so](done/t003.md) — **DONE** | 45m | t002 |
| t004 | [Production check in bex-canary: a sibling cannot reach a worker or a cron run, and they still send](done/t004.md) — **DONE** | 45m | t003 |
| t009 | [A cron run carries the workspace label, and a restored template its plan's network labels](done/t009.md) — **DONE** | 45m | t004 |
| t010 | [Production check: a cron run cannot reach a worker or a free web service, and no free-plan pod lacks the send-only label](done/t010.md) — **DONE** | 30m | t009 |
| t005 | [Render parity](done/t005.md) — **DONE** | 30m | t004, t010 |
| t006 | [Simplify](done/t006.md) — **DONE** | 20m | t005 |
| t007 | [Test coverage](done/t007.md) — **DONE** | 45m | t005 |
| t008 | [Closeout](done/t008.md) — **DONE** | 10m | t006, t007 |

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

## Unblock — 2026-11-26 (`/loopx w6`, second gate)

- **Affected:** t010, then t005, t006 and t008.
- **Source:** verified evidence. The newest pin on `origin/main`, `c767c1bbd9` (`chore(deploy): pin platform images to 9b91284a4a2e`), carries t009's `201a068b86`: `git merge-base --is-ancestor 201a068b86 9b91284a4a2e` holds.
- **Next:** t010's production check in `bex-canary` (disposable fixtures), then t005, t006 and t008 as their tasks say. No user gate remains.

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

## Production check (t010, 2026-11-26)

On pin `9b91284a4a2e` (`c767c1bbd9`; operator `bex-operator@sha256:3838967a…`, the digest the pin names, running since 07:04Z), in `bex-canary` with the QA user's session and a run-minted API key, both removed afterwards. `deny-send-only-private-ingress` and `deny-free-plan-private-ingress` were both `Valid=True`. Fixtures, all disposable, `busybox:1.37` image services: Starter web `w6-m640-web` (`srv-dc3u1kesfruc73ft1ql0`, internal host `w6-m640-web-ljrf`, httpd on 10000), Starter background worker `w6-m640-worker` (`srv-dc3u1khm1rus738gvibg`, httpd on 8080, `w6-m640-worker-13dh-discovery`) and Starter cron job `w6-m640-cron` (`srv-dc3u21msfruc73ft1qn0`, one manual run at 07:37Z that served 8080 and probed three times a minute and a half apart). The free control was `hello-go`, bex's own fixture.

- **Labels.** The CronJob's template and the run's pod carried `app.bex.co/workspace: tea-daif693dqjvc73e7as3g` and `app.bex.co/private-network: send-only` (t004's run carried `send-only` but no workspace label, so the denies did not count it as a tenant source). The worker's pod carried both; the paid web pod carried no `send-only`.
- **Refused, from the cron run's own command, in each of the three rounds.** `nc -z -w 5` and `wget -T 5` timed out after 5 s to the worker's pod IP on 8080, to `w6-m640-worker-13dh-discovery:8080` (the name resolved: `wget` timed out on the download, not the address), and to `hello-go` by pod IP and by `hello-go:10000`. Both targets served meanwhile: the worker answered `worker-8080` on its own loopback, and `hello-go.onbex.co` answered 200.
- **It still sends.** In each round the run reached `w6-m640-web-ljrf:10000` at once (`nc` open, `wget` returned `web-10000`): a paid web service still receives (w9/m57).
- **No free-plan pod or template lacks the send-only label.** Across production at 07:30Z, 4 of 4 running `app.bex.co/free-plan: "true"` pods (`hello-go`, a disposable QA web service, and two in a customer workspace), 6 of 6 Deployment templates and 0 CronJob templates with the free-plan label all carried `send-only`. `qa-20261123-w8-412-node` (disposable, the restored-template case t004 found) was still there, hibernated: its restored template now carried `send-only`, and woken through its public URL at 07:31Z (200 after 42 s) it ran a pod with `app.bex.co/free-plan: "true"` and `send-only`. The only pod in a `tea-*` namespace without `app.bex.co/workspace` was an ACME HTTP-01 solver in bex's own workspace (cert-manager's, not a tenant workload).
- **Cleanup.** The run was cancelled (204), `DELETE` answered 204 for the three services and each `GET` then 404, and no object of theirs remained in the namespace by 07:43:55Z. The API key was revoked and the QA session logged out.

## Production check after the free-plan ingress deny retired (t006, 2026-11-26)

`aef43af3ad` removed `deny-free-plan-private-ingress`; Argo's `bex-platform-prod` synced `4a5a4556a5`, which carries it, and the policy was gone by 08:50:08Z, leaving `deny-send-only-private-ingress` and `deny-free-plan-smtp-egress`. In `bex-canary` with a new QA session and run-minted key, both removed afterwards, a Starter web service `w6-m640-web` (`srv-dc3v5nusfruc73ft1v80`, internal host `w6-m640-web-che0`) and a Starter cron job `w6-m640-cron` (`srv-dc3v5umsfruc73ft1va0`), both disposable. Its manual run at 08:51Z carried the workspace and send-only labels, and in each of three rounds `hello-go` (free, its pod `send-only`) timed out after 5 s by pod IP and at `hello-go:10000`, while `w6-m640-web-che0:10000` answered `web-10000` at once. `hello-go.onbex.co` answered 200. Both services answered 204 to `DELETE` and 404 after, and no object of theirs remained by 08:55:43Z.

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
