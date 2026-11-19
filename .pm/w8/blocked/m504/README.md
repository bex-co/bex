# w8 · m504 — A service switched to a prebuilt image goes live on the port its image binds, private services included

**Worker:** worker8 **Goal:** a web or private service switched from a native build to a prebuilt image (`bex services update <id> --image <ref>`, the REST/GraphQL/MCP source writers, a Blueprint sync) goes live on the port its image binds, as Render serves it, and reads that port afterwards, as an image-created service does. **Status:** blocked (t001–t006 and t008 done; t007's production replay on pin `dff52c596d13` found the switched deploy's log naming no port, which t008 fixes in `9fac4888a`; t007 waits for a roll that carries it)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | A configured-port private service whose process binds another port goes live on it — **DONE** | 1h15m | — |
| [t002](done/t002.md) | A switched service reads the port it serves: its native release's before the deploy, its image's after — **DONE** | 45m | t001 |
| [t003](done/t003.md) | ADR089, ADR004 and ADR018: a source switch keeps the configured-port mode, and detection finds the image's port — **DONE** | 20m | t002 |
| [t004](done/t004.md) | Render parity — **DONE** | 20m | t003 |
| [t005](done/t005.md) | Simplify — **DONE** | 15m | t004 |
| [t006](done/t006.md) | Test coverage — **DONE** | 30m | t004, t005 |
| [t008](done/t008.md) | A switched web service's deploy log names the port detection found — **DONE** | 30m | t006 |
| [t007](t007.md) | Closeout | 30m | t008 |

Total: **8 tasks, about 4h25m**.

## Definition of done

1. **Private rescue.** An operator test fails on today's code and passes after t001: a private service with no explicit port, no `PORT` and no image port mode, whose instance listens only on 6379, adopts 6379 for its probe and Services and goes Ready. An explicit port or `PORT` still waits on the one it names, and the web service detector is unchanged.
2. **Reads.** `TestSwitchedServiceReadsAsAnImageCreatedOne` (`lego/backend/internal/apps/native_image_switch_test.go`) compares the reads after the switched service's deploy has found the image's port, with `port` and `openPorts` no longer excluded. Before that deploy, the switched service reads the port its native release still serves, as Render's `openPorts` reports the running instance. A private variant covers t001's adopted port.
3. **Docs.** ADR089 § Port slice decisions, ADR004's source switch and ADR018's port rows say why a switch keeps the configured-port mode and how the switched image's port is found.
4. **Production**, after a roll that carries w8/m461 (`0d5ef6256`) and this milestone, in `bex-canary`:
   - A free native Node web service (`render-examples/express-hello-world`, `--runtime node --build-command yarn --start-command 'node app.js'`) switched with `bex services update <id> --image docker.io/library/nginx:1.27-alpine --confirm` and deployed goes `live`. Its URL answers 200 with the nginx welcome page, the deploy log names the detected port 80, and `GET /v1/services/<id>` reads `port: 80` and `openPorts: [{port: 80, protocol: TCP}]`.
   - A Starter native private service (private services are paid-only, as on Render) switched to `docker.io/library/redis:7-alpine` and deployed goes `live` on 6379, and `openPorts` lists 6379. It is deleted as soon as this is read.
   - Every fixture is deleted by ID, and each delete is confirmed with a 404.

## Results (2026-11-19)

- **t001, private rescue.** `detectBoundPort` (`lego/operator/internal/controller/port_detect.go`) now runs for a private service as well as a web one, when the service names no port of its own: no explicit port, no `PORT`, no image port mode. A private service's listener is not filtered to HTTP, and among several it takes the one an image-port private service's primary would (`preferredPort`: 3000 if open, else the lowest; reserved ports never count). The same two-scan confirmation (`candidateConfirmed`) and grace apply. The web path is unchanged apart from sharing those helpers, and the scan's bounded phase now also dials the probed port. The stall texts read correctly for a private service: "The process is listening on port 6379, which the service now uses", and Render's "no open port … background worker" hint when nothing answers. The multi-HTTP-port text is never used, since a private service always chooses. `DetectedPort`'s CRD description now names private services.
  - Guards: `TestDetectBoundPortAdoptsAPrivateServicesListener`, `TestDetectBoundPortChoosesAPrivateServicesListener`, `TestDetectBoundPortLeavesConfiguredPrivateServicesAlone`.
  - Envtest `serves a private service that names no port on the port its process binds (w8/m504)`: a `redis:7-alpine` private service with the stored default 10000 and a 6379-only unready instance adopts 6379. The probes, the `bex-http` container port and both Services move to 6379, `PORT` stays 10000, and once the new instance is Ready the App is Ready with `openPorts` `[6379]` and address `…:6379`.
  - With the old type gate restored, both the unit tests and the envtest fail.
- **t002, reads.** `TestSwitchedServiceReadsAsAnImageCreatedOne` (`lego/backend/internal/apps/native_image_switch_test.go`) has web (nginx-like, 80) and private (redis-like, 6379) cases. Before the switched source deploys, the switched service reads `port` 10000 and `openPorts` `[{10000, TCP}]`, the native release's. After its deploy records `status.detectedPort`, the full REST read and the MCP `get_service` read equal an image-created service's, `port` and `openPorts` included. The GraphQL `port`, `configuredPort` and `openPorts` are equal too. Pointing the detected port elsewhere fails every surface's check.
- **t003, docs.** ADR089 § Port slice decisions item 7 (and a pointer from D3 phase 2 item 10), ADR004 ("A switched service serves the port its image binds") and ADR018 (the Open ports row's detection item, and the switch note in the Runtime row).
- **t005, simplify.** The three detectors share one choice rule (`preferredPort`, also used for an image's declared ports) and one two-scan confirmation, and the web and private bound-port paths share `adoptListener`. The scan calls stay separate because their failure texts differ: the configured-port service gets the `PORT` text, an image-port one the service-port text.
- **t006, coverage.** `TestDetectBoundPortNamesAPrivateServiceWithNoOpenPort` covers a loopback-only listener, an unreachable instance and a reserved port alone: no adoption, and the no-open-port text. `TestDetectBoundPortWaitsOutAPrivateServicesEarlyMetricsPort` covers a metrics port that opens before the server. `TestAPrivateServicesExplicitPortClearsADetectedOne` covers a switch back to a native build that names its port. `TestARolledBackPrivateServiceServesItsNativePortAgain` covers a rollback to the native release, which binds 10000 and is re-detected there. Each test failed when its guarded line was reverted: the no-open-port return, the confirmation, the clearing of a stale detection, and the type gate.

## Unblocked (2026-11-19, `/loopx w8` worker)

- **Affected:** t007. **Source:** verified evidence, no user decision.
- **Finding:** the newest pin on `origin/main`, `a91e9aeb3` (`chore(deploy): pin platform images to dff52c596d13`, 2026-11-19 11:30Z), carries both w8/m461 (`0d5ef6256`) and this milestone (`cf5f56884`): `git merge-base --is-ancestor` holds for each.
- **Status:** pending verification. A deployed pin is not proof that the definition of done holds on production.
- **Next:** t007 replays step 4 in `bex-canary`, records the commands and answers here, deletes every fixture by ID with a 404 check, and closes the milestone.

## Production replay (t007, 2026-11-19 12:05–12:16 UTC)

Production pin `dff52c596d13`, `https://api.bex.co/v1/`, bex CLI v0.7.0 (Render CLI v2.36.0), the QA user by device OAuth in an isolated config, workspace `bex-canary` (`tea-daif693dqjvc73e7as3g`). Every mutation ran with `--confirm </dev/null`. **Not met:** the switched deploy's log named no port, and the private case could not run on a free plan.

1. **Web switch: passes but for the log line.**
   - `bex services create --name qa-20261119-m504-web --type web_service --repo https://github.com/render-examples/express-hello-world --branch main --runtime node --plan free --build-command yarn --start-command 'node app.js' --confirm -o json` exited 0 with `srv-dbvebrg1ks2s73a6pfs0`. Its first deploy went `live` at 12:09:41, and the URL answered 200 with `<title>Hello from Render!`. REST read `port: 10000` and `openPorts: [{port: 10000, protocol: TCP}]`.
   - `bex services update <id> --image docker.io/library/nginx:1.27-alpine --confirm -o json` exited 0 and started no deploy. REST then read `runtime: image` and `env: image`, and still `port: 10000` and `openPorts: [{10000, TCP}]`, the native release's (t002).
   - `bex deploys create <id> --wait --confirm -o json` exited 0 after 79 s: `dep-dbvee9u014qc73acu830` went `live` on `nginx:1.27-alpine` (`sha256:65645c7bb6a0…`). The first nginx instance started at 12:10:28 and was replaced at 12:11:24 by one serving the detected port. The URL answered 200 `text/html` with `<title>Welcome to nginx!`. REST read `port: 80` and `openPorts: [{port: 80, protocol: TCP}]`.
   - **Fails:** the deploy log (`bex logs --resources <id> --type build,deploy`) read `==> Deploy queued`, `==> Deploying image docker.io/library/nginx:1.27-alpine`, `==> Your service is live 🎉`, and named no port. The control, an image-created `nginx:1.27-alpine` service (`qa-20261119-m504-img`, `srv-dbvefeo1ks2s73a6pgsg`), went `live` in about 50 s. Its log read `==> Detected HTTP on port 80; serving it` before the live line, and it read `port: 80` and `openPorts: [{80, TCP}]` and served the nginx page. The deploy row recorded a detected port only for an image-port service's HTTP discovery. t008 fixes that in `9fac4888a`.
2. **Private service: unverified on production.** `bex services create --type private_service --plan free …` answered 400 `a private_service requires a paid plan; plan must be one of starter|standard|pro|pro_plus|pro_max`, as on Render. No paid fixture was created: the `/loopx` dispatch for this run allowed free plans only, and said to record a private-service case that needs a paid plan as unverified. t001's envtest (`serves a private service that names no port on the port its process binds (w8/m504)`, `redis:7-alpine` adopting 6379) is the evidence so far.
3. **Cleanup.** `bex services delete <id> --confirm` exited 0 for both fixtures. `GET /v1/services/<id>` answered 404 for each, and so did both `https://qa-20261119-m504-{web,img}.onbex.co/`. `hello-go` and another run's `m484-t005-cache` were not touched. The QA session was logged out with `qa-login.sh --logout`, and the isolated CLI config was deleted.

## Parked (2026-11-19)

- **Gate 1, clears by itself:** a production roll that carries `9fac4888a`. To check, find the newest `chore(deploy): pin platform images to <sha>` on `origin/main`, then run `git merge-base --is-ancestor 9fac4888a <sha>`.
- **Gate 2, the private case:** a Starter private service in `bex-canary` costs money. A replay under the free-plans-only rule records it as unverified again. To verify it live, the user has to allow one Starter private service for a few minutes (`/unblock w8`).
- **Next:** replay step 4's web case and check that the switched deploy's log reads `==> Detected HTTP on port 80; serving it`. Then run or record the private case as Gate 2 allows, and close the milestone.

## Render parity

| Surface | Before the switched deploy | After it | Result |
| --- | --- | --- | --- |
| REST `GET /v1/services/{id}` `port` / `openPorts` | 10000 / `[{10000, TCP}]`, the native release | the image's port, as an image-created service reads | ✅ `TestSwitchedServiceReadsAsAnImageCreatedOne` (web and private) |
| MCP `get_service` | same as REST | equal to an image-created service's full read | ✅ same test |
| GraphQL `port`, `openPorts` | 10000 | the image's port | ✅ same test |
| Dashboard Settings → Port (`configuredPort`, `App.SavedPort`) | 10000 | the image's port | ✅ same value. The row shows a detected port as if configured: follow-up [w8/338](../../done/338.md) (done 2026-11-19) |
| Blueprint sync, native entry → `runtime: image` | — | no port mode written; the port is detected as after any other switch | ✅ `TestBlueprintGitToImageKeepsTheConfiguredPortMode` (web and private); `ApplyBlueprintServiceSpec` has no port-mode applier |
| Operator, private service | probed on 10000 only, so it failed at 15 minutes | listener adopted after two scans | ✅ t001 |

Render serves a private service on any port it binds, and a web service on the port it detects; it does not read `EXPOSE` ([render.com/docs/private-services](https://render.com/docs/private-services), [render.com/docs/web-services#port-binding](https://render.com/docs/web-services#port-binding)). There is no new divergence. Detection costs about a minute on the switched release's first deploy (ADR089 item 7).

## Source + Goal linkage

- **Source:** inbox note [w8/337](../../done/337.md), a follow-up from w8/m461 t004 on 2026-11-17. It was promoted by `/loopx w8` on 2026-11-19, when triage found that the note's premise holds only for private services and that its proposed fix is unsafe.
- **Triage evidence (code at `36463267c`, 2026-11-19):**
  - **Web: detection already serves the switched image.** A switched web service keeps no port mode and the default `spec.port` 10000. `explicitServingPort` (`lego/operator/internal/controller/port_detect.go`) is therefore false, and `detectBoundPort` adopts the image's lone HTTP listener while the startup check on 10000 fails (w8/177, w8/202). On production pin `838feec85fdd` (2026-10-28), a configured-port `nginx:1.27-alpine` service in exactly this state went live on 80 in under a minute ([w8/177](../../done/177.md), [w4/393](../../../w4/done/393.md)). The note's claim that nginx "still fails its first deploy after a switch" was inferred from a fake-client read diff. It was never measured.
  - **Private: the gap is real, and wider than the switch.** `detectBoundPort` returns early for any type but a web service. `detectPrivatePrimary` (`private_ports.go`) needs `UsesImagePorts()`. A configured-port private service is probed by TCP on its configured port only (`healthCheckHandler`, `deployment_projection.go`). So a private service switched to an image that binds anything else, such as redis on 6379, fails at the 15-minute start limit. So does a native private service that binds another port. Render: "Private services can listen on _almost_ any port" and "must bind to at least one port" ([render.com/docs/private-services](https://render.com/docs/private-services), read 2026-11-19).
  - **The note's fix (stamp `web-image-v1` / `image-v1` in the source writer) would break a save-only switch.** The operator gates `resolveImageNetwork` and the image-port Service projection on `app.Spec.HasImagePorts()`, which is the saved spec (`app_controller.go`, `image_network.go`). `PortMode` is release identity (`lego/types/v1alpha1/app_identity.go`). Update Source saves without deploying, as on Render, so flipping the mode at the save would re-project, and could roll, the native release that is still serving. `port_mode` is also a projector-owned row column (`store/reconciler.go` `applyOwnedSpec`), which `SetAppSource` does not write. Detection reaches Render's behavior without any of that. Render routes to the port a service binds; it does not read `EXPOSE`. Stamping the mode at the switched release's deploy remains a possible later variant if detection's ~1-minute first-deploy delay matters.
- **Goal linkage:** ADR008 § Render parity first. Switching a service's source to a prebuilt image is a Render dashboard, API and CLI flow, and the service must serve afterwards.
- **Expected outcome:** a private service switched to an image that does not bind 10000 goes live instead of failing at 15 minutes. A native private service that binds another port does too. Web services keep working as they do today, and every read shows the port that is actually served.
- **Why now:** w8/m461 makes the switch run the image's own entrypoint, so after its roll the port is the next thing a switched image can fail on.
- **Render parity included:** the change reaches `port`/`openPorts` on REST, GraphQL and MCP, and the dashboard's port row.
