# w4 · m350 — A new image web service whose process listens on a port its image doesn't declare starts twice on its first deploy: the first instance is replaced about a minute in, and Live comes about 45 s later than for the same image on its declared port

**Worker:** worker4 **Goal:** a first deploy rolls one ReplicaSet whichever port the image's HTTP listener turns out to be on. HTTP port discovery changes only the Services and the route, never the pod template, as Render's port scan routes to the port it finds without restarting the instance. **Status:** todo (t001–t004 done 2026-11-14; t005 replays the DoD after the next production deploy)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Reproduce in envtest and pin which pod-template fields discovery changes on a first release — **DONE** | 30m | — |
| [t002](done/t002.md) | Keep a first release's pod template independent of HTTP port discovery — **DONE** | 60m | t001 |
| [t003](done/t003.md) | Simplify — **DONE** | 15m | t002 |
| [t004](done/t004.md) | Test coverage — **DONE** | 30m | t002 |
| [t005](t005.md) | Closeout (live replay on production) | 30m | t003, t004 |

## Definition of done

All on production. Create two Free image web services from `docker.io/traefik/whoami:v1.10`, which declares `EXPOSE 80`, with `POST /v1/services` at the same moment:

- `…-p10k` with env `WHOAMI_PORT_NUMBER=10000`;
- `…-p80` with no env.

Poll `GET /v1/services/{id}/instances` and `GET /v1/services/{id}/deploys?limit=1` every 3 s until both deploys are `live`.

1. **One instance on a first deploy.** `…-p10k` lists exactly one instance id from its first instance until Live, as `…-p80` does. Its app log has one `Starting up on port 10000` line.
2. **Live in the same time.** `…-p10k`'s deploy reaches `live` within 15 s of `…-p80`'s, measured from each deploy's `createdAt`.
3. **Discovery still works.** `…-p10k`'s narration reads `==> Detected HTTP on port 10000; serving it`, and `curl -sS https://<p10k>.onbex.co/` answers 200 with `Hostname:` equal to that one instance's pod.
4. **The serving-release guard is unchanged.** On `…-p10k`, change the image to `docker.io/traefik/whoami:v1.11` and keep polling every 2 s with `curl` against the URL. Every response is 200, and no response comes from the new release before its deploy reads `live` (w4/m320).
5. **Fixtures.** Both services are deleted, and each GET answers 404.

## Source + Goal linkage

- **Source:** live `/qa-find-bugs w4` loop, round 209, 2026-11-12 16:39–16:53 UTC (`muse.env`, workspace `bex-canary`; pin `ca7fbbe606c8`). It was found during the scaling journey and confirmed with a side-by-side control. The probes are under § Finding.
- **Governing docs:**
  - [w2/078](../../w2/done/078.md) (done, 2026-11-09) made a first deploy roll once by forcing the HTTP probe only while an earlier release serves (`webImageHTTPSelectionGuardsServing`). Its dev-2 repro used `traefik/whoami` on its declared port 80, so the listener matched the declared port.
  - [w4/m320](../blocked/m320/README.md) (blocked) covers a later deploy over a serving release.
  - w8/m141 covers web-image HTTP port discovery.
- **Goal linkage:** Render parity for the first deploy of a prebuilt image (ADR018 "Create service"; Render documents a scan for the port a service binds to). This is also the zero-downtime deploy story's first step.
- **Expected outcome:**
  - an image whose server listens on any port starts once;
  - its first deploy is as fast as one on its declared port;
  - its logs, instance list and instance-count chart show one instance.
- **Why now:** minor, but it touches every prebuilt image that reads its port from its own setting, not `EXPOSE`. That covers many off-the-shelf images, and Render's own docs tell users to listen on `PORT` or let the scan find their port. A start command with side effects (a migration, a cache warm, a "started" notification) runs twice, and the first run is killed about a minute in.
- **Render parity task omitted:** no REST, GraphQL, MCP or dashboard behavior changes. This is the operator's pod template during port discovery. The deploy row, events and narration are already right.

## Finding (round 209)

### The control, created together at 16:49:53–54 UTC (Free, `traefik/whoami:v1.10`)

| service | env | instances seen (id suffix @ createdAt) | app log | deploy `createdAt` → `finishedAt` |
| --- | --- | --- | --- | --- |
| `qa-20261112-r209-p10k` (`srv-dbquscc5b3as73bg3cr0`) | `WHOAMI_PORT_NUMBER=10000` | `954pc` @ 16:50:01, replaced by `2ukbu` @ 16:51:06 | `16:50:05 Starting up on port 10000` (954pc), `16:51:10 Starting up on port 10000` (2ukbu) | 16:49:53 → **16:51:52** |
| `qa-20261112-r209-p80` (`srv-dbquscns9cvc73b61tvg`) | none | `qk7lg` @ 16:50:01 only | `16:50:07 Starting up on port 80` | 16:49:54 → **16:51:09** |

- Narration (GraphQL `logs(type:"build")`):
  - p10k: `==> Deploy queued` 16:49:53, `==> Deploying image …` 16:50:09, `==> Detected HTTP on port 10000; serving it` and `==> Your service is live 🎉` 16:51:52.
  - p80: the same lines, with `Detected HTTP on port 80` at 16:51:09.
- Events: only `deploy_started` and `deploy_ended` for both.
- **First seen** on Starter `qa-20261112-r209-scale` (`srv-dbqungkdhgcs73ao048g`, same image and env):
  - instance `mvvn` logged `Starting up on port 10000` at 16:39:40;
  - instance `a5tu` logged it again at 16:40:53;
  - the deploy (created 16:39:30) went live at 16:41:37.
  - `GET /v1/metrics/instance-count` read `16:41:35 → 2` and `16:42:35 → 3` while the service ran 1, then 2. That is the 90 s liveness window (`metrics/source.go:272-277`, w4/m110 t002) still counting the replaced instance.

### Mechanism (code read at `origin/main`, unchanged since the pin)

- `selectWebImageHTTP` (`lego/operator/internal/controller/web_image_http.go:126-190`) first probes `network.Ports`, the image's declared ports (`[80]` for whoami). It finds no HTTP there, scans, and finds 10000.
  - It then appends 10000 to `network.Ports` and sets `network.PrimaryPort` (`:166-181`). It returns "reproject probes and routing on the chosen port first".
- The pod template is projected from those values:
  - the container ports come from `app.Status.ImageNetwork.Ports` (`deployment_projection.go:293-302`);
  - all three probes use `healthCheckHandler(path, p.port, …)`, a TCP socket on `p.port` when there is no health check path (`:353-381`, `:409-421`);
  - `PORT` is `processPort(app, p.port)` (`app_controller.go:5860-5878`, `port_detect.go:100-105`).
  - So the template changes as soon as the primary port moves from 80 to 10000, and the Deployment rolls a second ReplicaSet.
- The first instance could never have become Ready: its probes dial port 80, where nothing listens.
- w2/078's guard (`webImageHTTPSelectionGuardsServing`, `web_image_http.go:59-61`) only stops the forced `GET /` probe on a first release. It does not cover a primary port that discovery moves.

### Target behavior

While a first release's HTTP port is being discovered, the pod template carries nothing discovery can change. Only the Services' target port and the route move to the chosen port.

- The default health check (no Health Check Path) is Render's own: "By default, health checks are TCP socket probes to one of your service's open ports". So it must pass when any non-reserved port in the pod is listening, not one fixed port.
  - The exec health-probe binary already ships in every pod that has a Health Check Path (`health_probe.go:49-57`, `usesHealthProbeBinary`). An `--any-open-port` mode that reads the pod's own listening sockets would make the probe port-agnostic.
- The container `ports` list stays as it was at dispatch (it is informational to Kubernetes), or is left out while discovery is pending.
- `PORT` stays what it was at dispatch: Render's default 10000 unless the user set `PORT`. t001 confirms what the first instance received.
- Unchanged:
  - a service with a Health Check Path keeps its HTTP probe on the chosen port;
  - a later deploy over a serving release keeps w4/m320's guard and hold;
  - a first release whose listener is its declared port still rolls once (w2/078).

### Unverified

- The exact pod-template diff between the two ReplicaSets (probe port, container ports, `PORT`). No cluster access this round; t001 pins it in envtest.
- Repo-built services. They detect ports through `port_detect.go` (`Status.DetectedPort`), whose `processPort` keeps `PORT` at the default; they may roll once today.
- Private services from an image. They use `private_ports.go`, a sibling discovery path (w4/m323).
- Render's own instance behavior for an image on an undeclared port is reasoned from its port-binding docs, not captured.

### Dedupe

- [w2/078](../../w2/done/078.md) (done) fixed the first-deploy second roll for a listener on the declared port only. This is the case it didn't cover, not a regression of what it verified.
- [w4/m320](../blocked/m320/README.md) (blocked) is about a deploy over a serving release.
- [w4/m323](../blocked/m323/README.md) (blocked) is private services pinned to 10000.
- `grep -rlE "rolls? (a second|two) ReplicaSet|second ReplicaSet|rolls twice|rolled twice|two ReplicaSets" .pm` finds only those three, w5/m149, w6/m104 and w1/m52, none open for a first deploy.
- Not deploy lag: `git log ca7fbbe606c8..origin/main -- lego/operator/internal/controller/web_image_http.go lego/operator/internal/controller/deployment_projection.go lego/operator/internal/controller/image_network.go` is empty.

### Also verified this round (all pass)

- **Manual scaling** on Starter `qa-20261112-r209-scale`. Scaling → Instances 2 → Save Changes:
  - `numInstances` read 2 and `/instances` listed two ids within a minute;
  - `instance_count_changed {"fromInstances":1,"toInstances":2}` was recorded at the save;
  - outside the browser, `curl` every 0.5 s alternated `Hostname:` between two pods.
  - Back to 1: the toast "Scaling to 1 instance…" appeared at once, and `instance_count_changed {"fromInstances":2,"toInstances":1}` was recorded.
- **Zero downtime.** 396 requests across the scale-up, a manual deploy at 2 instances (two new pods rolled in, both old ones out by 16:45:17) and the scale-down, all 200.
- **Seen, not filed:**
  - The deploy read `update_in_progress` until 16:46:39, 80 s after traffic had moved, inside the 90 s discovery window (`webHTTPDiscoveryWindow`, w8/m141).
  - The instance-count window's lag is recorded above.
