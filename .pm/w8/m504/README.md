# w8 · m504 — A service switched to a prebuilt image goes live on the port its image binds, private services included

**Worker:** worker8 **Goal:** a web or private service switched from a native build to a prebuilt image (`bex services update <id> --image <ref>`, the REST/GraphQL/MCP source writers, a Blueprint sync) goes live on the port its image binds, as Render serves it, and reads that port afterwards, as an image-created service does. **Status:** todo (t001–t006 and t008 done; t007's production replay on pin `dff52c596d13` found the switched deploy's log naming no port, which t008 fixes; the replay runs again after a roll that carries t008)

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

## Render parity

| Surface | Before the switched deploy | After it | Result |
| --- | --- | --- | --- |
| REST `GET /v1/services/{id}` `port` / `openPorts` | 10000 / `[{10000, TCP}]`, the native release | the image's port, as an image-created service reads | ✅ `TestSwitchedServiceReadsAsAnImageCreatedOne` (web and private) |
| MCP `get_service` | same as REST | equal to an image-created service's full read | ✅ same test |
| GraphQL `port`, `openPorts` | 10000 | the image's port | ✅ same test |
| Dashboard Settings → Port (`configuredPort`, `App.SavedPort`) | 10000 | the image's port | ✅ same value. The row shows a detected port as if configured: follow-up [w8/338](../done/338.md) (done 2026-11-19) |
| Blueprint sync, native entry → `runtime: image` | — | no port mode written; the port is detected as after any other switch | ✅ `TestBlueprintGitToImageKeepsTheConfiguredPortMode` (web and private); `ApplyBlueprintServiceSpec` has no port-mode applier |
| Operator, private service | probed on 10000 only, so it failed at 15 minutes | listener adopted after two scans | ✅ t001 |

Render serves a private service on any port it binds, and a web service on the port it detects; it does not read `EXPOSE` ([render.com/docs/private-services](https://render.com/docs/private-services), [render.com/docs/web-services#port-binding](https://render.com/docs/web-services#port-binding)). There is no new divergence. Detection costs about a minute on the switched release's first deploy (ADR089 item 7).

## Source + Goal linkage

- **Source:** inbox note [w8/337](../done/337.md), a follow-up from w8/m461 t004 on 2026-11-17. It was promoted by `/loopx w8` on 2026-11-19, when triage found that the note's premise holds only for private services and that its proposed fix is unsafe.
- **Triage evidence (code at `36463267c`, 2026-11-19):**
  - **Web: detection already serves the switched image.** A switched web service keeps no port mode and the default `spec.port` 10000. `explicitServingPort` (`lego/operator/internal/controller/port_detect.go`) is therefore false, and `detectBoundPort` adopts the image's lone HTTP listener while the startup check on 10000 fails (w8/177, w8/202). On production pin `838feec85fdd` (2026-10-28), a configured-port `nginx:1.27-alpine` service in exactly this state went live on 80 in under a minute ([w8/177](../done/177.md), [w4/393](../../w4/done/393.md)). The note's claim that nginx "still fails its first deploy after a switch" was inferred from a fake-client read diff. It was never measured.
  - **Private: the gap is real, and wider than the switch.** `detectBoundPort` returns early for any type but a web service. `detectPrivatePrimary` (`private_ports.go`) needs `UsesImagePorts()`. A configured-port private service is probed by TCP on its configured port only (`healthCheckHandler`, `deployment_projection.go`). So a private service switched to an image that binds anything else, such as redis on 6379, fails at the 15-minute start limit. So does a native private service that binds another port. Render: "Private services can listen on _almost_ any port" and "must bind to at least one port" ([render.com/docs/private-services](https://render.com/docs/private-services), read 2026-11-19).
  - **The note's fix (stamp `web-image-v1` / `image-v1` in the source writer) would break a save-only switch.** The operator gates `resolveImageNetwork` and the image-port Service projection on `app.Spec.HasImagePorts()`, which is the saved spec (`app_controller.go`, `image_network.go`). `PortMode` is release identity (`lego/types/v1alpha1/app_identity.go`). Update Source saves without deploying, as on Render, so flipping the mode at the save would re-project, and could roll, the native release that is still serving. `port_mode` is also a projector-owned row column (`store/reconciler.go` `applyOwnedSpec`), which `SetAppSource` does not write. Detection reaches Render's behavior without any of that. Render routes to the port a service binds; it does not read `EXPOSE`. Stamping the mode at the switched release's deploy remains a possible later variant if detection's ~1-minute first-deploy delay matters.
- **Goal linkage:** ADR008 § Render parity first. Switching a service's source to a prebuilt image is a Render dashboard, API and CLI flow, and the service must serve afterwards.
- **Expected outcome:** a private service switched to an image that does not bind 10000 goes live instead of failing at 15 minutes. A native private service that binds another port does too. Web services keep working as they do today, and every read shows the port that is actually served.
- **Why now:** w8/m461 makes the switch run the image's own entrypoint, so after its roll the port is the next thing a switched image can fail on.
- **Render parity included:** the change reaches `port`/`openPorts` on REST, GraphQL and MCP, and the dashboard's port row.
