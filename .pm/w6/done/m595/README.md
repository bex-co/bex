# w6 · m595 — A disk service's new image is on its node before Recreate stops the old instance

**Worker:** worker6 **Goal:** a code deploy of a disk-bearing service (`Recreate`) pulls its new image onto the volume's node while the old instance still serves, so the downtime stays at m547's same-node bound of about 4 s however large the new layers are, instead of adding a pull of 8–24 s for a typical built image. **Status:** done

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [The pre-pull Job](done/t001.md) — **DONE** | 1h | — |
| t002 | [A disk service's code deploy writes its template once the new image is on the node](done/t002.md) — **DONE** | 2h | t001 |
| t003 | [The pre-pull pod is never an instance, and admission treats it like one](done/t003.md) — **DONE** | 45m | t001 |
| t004 | [Record the pre-pull in ADR082, ADR004 and ADR018](done/t004.md) — **DONE** | 30m | t002 |
| t005 | [Ship and measure on production](done/t005.md) — **DONE** | 1h30m | t002, t003, t004 |
| t006 | [Simplify](done/t006.md) — **DONE** | 15m | t005 |
| t007 | [Test coverage](done/t007.md) — **DONE** | 30m | t005 |
| t008 | [Closeout](done/t008.md) — **DONE** | 10m | t006, t007 |

## Definition of done

- On production, a disk-bearing Starter fixture in `bex-canary` takes at least two code deploys of an image with at least 100 MB of layers new to the volume's node, measured as in [m547 t011](../m547/done/t011.md). In each, containerd fetches no layer of the new digest between the old instance's `SIGTERM` and the new instance's start: its `PullImage` there is the `imagePullPolicy: Always` re-check alone (under 1 s). Each window is within 1 s of the same fixture's restarts.
- A restart and a plan change start no pre-pull, and a cordoned-node run starts none.
- No pre-pull Job or pod is left once a rollout ends, and no customer surface shows one: the service's instances (`GET /v1/services/{id}/instances`), SSH targets, metrics, logs and events.
- ADR082 § D3 describes the pre-pull and its measured window. ADR004 and ADR018 no longer list the pull as an open excess term.

## Source + Goal linkage

- **Source:** inbox note [w6/160](../160.md), filed by [w6/m547](../m547/README.md) t001 on 2026-11-21 as the large-image variant of its t004, and promoted by `/loopx w6` on 2026-11-23. m547 t011 measured a code deploy of a tag new to the node at 6.0 s, against 3.99–4.26 s for a cached image, on a 5 MB image.
- **Goal linkage:** Render parity ([ADR008 § Render parity first](../../../../docs/ADR008-vision.md#render-parity-first)). [Render's persistent disks](https://render.com/docs/disks): "Render stops the old instance before starting the new one, which makes the service unavailable for a few seconds." Under `Recreate`, bex is down for the whole pull.
- **Expected outcome:** a disk service's code deploy is down about 4 s plus its own stop and start, whatever its image size, as its restarts are today.
- **Why now:** the measurement below shows the pull is the largest term left in a disk service's code deploy. m547 and [m559](../m559/README.md) brought the rest of the same-node window to about 4 s, and the holder they shipped already runs on the volume's node before the template changes, so the rollout hook exists.
- **Render parity task omitted:** operator mechanism only. No REST, GraphQL, MCP or dashboard field changes. t003 checks that the pre-pull pod appears on none of them.

## Measured at promotion (2026-11-23)

The note's own gate: "If they are tenths of a second, close this note as not needed." They are not.

- **Method.** containerd's `Pulled image "<ref>" … size "<bytes>" in <duration>` lines in the syslog of every tenant and sandbox node (`kubectl get --raw /api/v1/nodes/<node>/proxy/logs/syslog` and `syslog.1`), from 2026-11-21T10:32Z, when the `bex-tenant-0` nodes were rolled, to 2026-11-23T15:05Z. Images of the tenant registry (`zot.bex-registry.svc:5000/…`) only, without agent sandbox snapshots. The kube events keep `Pulled` for an hour only, and held one such pull.
- **All built-image pulls (176, all on `bex-tenant-0` nodes).**
  - 74 took 0.10–0.78 s (median 0.17 s): a digest already on the node, which `imagePullPolicy: Always` re-checks.
  - 102 were new to the node and took 1.0–48.9 s: median 8.2 s, p75 11.5 s, p90 23.7 s. Images were 31 MB–1.4 GB, most about 1 GB.
- **Same-node code deploys (22):** a new digest of a service whose earlier digest was already on that node, the case a disk service's held deploy is.
  - 15 took 0.48–1.64 s: only a small top layer changed.
  - 7 took 8.25–23.53 s. Four were successive deploys of one service in bex's own workspace (`tea-d98210cbbpdc73dcrkvg`, 421 MB: 9.90, 10.51, 11.40 and 23.53 s). The other three were QA fixtures in `bex-canary` (disposable; 8.25, 11.84 and 13.31 s).
- **So:** a disk service whose code deploy changes a dependency layer would be down for 8–24 s more than its restart.

## Design notes from triage (t001 and t002 settle them)

- **A Job of its own, not a container in the holder.** The kubelet pulls a pod's images inside one pod sync and posts the pod's status between syncs. A holder carrying the pull would likely report `PodReadyToStartContainers` only once the pull ends, past the holder's 15 s mount bound (`diskHoldMountBound`, `lego/operator/internal/controller/disk_holder.go`) for most built images, and the hold would fall back. t002 confirms this, or folds the pull into the holder if it is wrong. The kubelet serializes pulls (`serializeImagePulls: true` in every tenant kubelet's `/configz`). The holder's `pause` image is pinned and never pulled, so it does not queue behind the pre-pull.
- **No tenant code runs.** The container's `command` is one that no image can execute, such as `/`, a directory, which fails with `EACCES`. The kubelet has then pulled the image, and the container ends with `StartError`. This answers the note's "images with no binary that exits at once". It also means:
  - an explicit non-root `runAsUser`, or the kubelet's `runAsNonRoot` check stops the container in `CreateContainerConfigError`, and the pod never ends;
  - a CPU limit at which runc's init fails within about a second. The holder needed 500m, because at 10m it took 24.7 s to start (w6/m559 t005).
- **Admission and pull credentials as the instance's.** The pod carries the projected template's `app.bex.co/verify-image` label, so the signature webhook refuses an unsigned tenant image before any node pulls it, as it does for the instance. It also carries the template's `imagePullSecrets` (zot's, a registry credential, a linked source's). It never carries `app.bex.co/container-policy`, so `operator-workload-admission.yaml` holds it to the strict container shape.
- **Quota.** It requests no CPU, memory or ephemeral storage, as the holder does. Its limits count toward `limits.cpu`, which is twice `requests.cpu` and which instances (limits equal requests) cannot fill. `count/jobs.batch` has room (750 in `bex-canary`). A refusal is a rollout without a pre-pull.
- **When it runs.** Only on a pass whose projected template names an image the stored template does not, and only on the node the hold uses (the hold's `diskHoldNode`). A restart or plan change keeps the image, and a cordoned or other-pool node moves the volume, so neither pre-pulls.
- **The bound.** The old instance serves while the operator waits, and the pull would otherwise run after `SIGTERM`. So the bound costs deploy latency, not downtime, and it can exceed the largest pull measured (48.9 s). Once it passes, the template is written, and an unfinished pull goes on in the kubelet's queue. The new instance's pull waits for it rather than starting over.
- **Rejected: an image volume.** An `image` volume source pulls without running anything, but `ImageVolume` is off on production's v1.34.9 (`kubernetes_feature_enabled{name="ImageVolume",stage="BETA"} 0`).

## Unblocked (2026-11-23)

The gate cleared by itself (verified evidence, no user decision): the newest pin on `origin/main`, `cded300699` (`chore(deploy): pin platform images to 9fdf7e0a1171`, 2026-11-23 17:33Z), carries `a61572221d` (`git merge-base --is-ancestor a61572221d 9fdf7e0a1171`), and every `bex-system` Deployment ran its digest `sha256:df6e69bd31bf` (`bex-controller-manager` from 17:37:19Z). Next: t005's production measure, then t004 step 2 and t008, as the workstream line said.

## Closed (2026-11-23)

The definition of done holds on production pin `9fdf7e0a1171` ([t005](done/t005.md)):

- **Code deploys.** On a Starter fixture in `bex-canary` (disposable) on `bex-tenant-0-zdp6v-85xrs`, three code deploys brought 223.5, 238.8 and 305.1 MB of layers new to the node. In each, the pre-pull's pull took 11.5–14.1 s and ended 0.9–1.7 s before the old instance's `SIGTERM`, and the new instance's `PullImage` of the same digest read 0 bytes in 0.6 s. The two whose server exits on `SIGTERM` were down 4.0 and 4.25 s, against 4.0–4.26 s for three restarts of the same fixture. The other replaced an instance whose start command ignored `SIGTERM`, so its window was the kubelet's 30 s grace, as was a same-tag control deploy's. `bex_disk_prepulls_total{result="pulled"}` read 4.
- **No pre-pull where none belongs.** Three restarts, a same-tag deploy and a Starter → Standard plan change started none. The cordoned run counted `skipped_unschedulable` and made neither a holder nor a pre-pull.
- **Nothing left, nothing shown.** No `dskpul-` Job or pod outlived its rollout. The instances list never showed more than one instance, and no event or log line named a pre-pull.
- **The ADRs.** ADR082 § D3 records the measured window, and ADR004 and ADR018 no longer list the pull as an excess term (t004).

t005 changed no code, so t006 and t007 stand. Filed on the way: [w6/173](../173.md) (a move pulls a large image only after the volume reaches the new node: 47.76–48.0 s).
