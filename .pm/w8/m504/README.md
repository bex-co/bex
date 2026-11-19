# w8 · m504 — A service switched to a prebuilt image goes live on the port its image binds, private services included

**Worker:** worker8 **Goal:** a web or private service switched from a native build to a prebuilt image (`bex services update <id> --image <ref>`, the REST/GraphQL/MCP source writers, a Blueprint sync) goes live on the port its image binds, as Render serves it, and reads that port afterwards, as an image-created service does. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | A configured-port private service whose process binds another port goes live on it | 1h15m | — |
| [t002](t002.md) | A switched service reads the port it serves: its native release's before the deploy, its image's after | 45m | t001 |
| [t003](t003.md) | ADR089, ADR004 and ADR018: a source switch keeps the configured-port mode, and detection finds the image's port | 20m | t002 |
| [t004](t004.md) | Render parity | 20m | t003 |
| [t005](t005.md) | Simplify | 15m | t004 |
| [t006](t006.md) | Test coverage | 30m | t004, t005 |
| [t007](t007.md) | Closeout | 30m | t006 |

Total: **7 tasks, about 3h55m**.

## Definition of done

1. **Private rescue.** An operator test fails on today's code and passes after t001: a private service with no explicit port, no `PORT` and no image port mode, whose instance listens only on 6379, adopts 6379 for its probe and Services and goes Ready. An explicit port or `PORT` still waits on the one it names, and the web service detector is unchanged.
2. **Reads.** `TestSwitchedServiceReadsAsAnImageCreatedOne` (`lego/backend/internal/apps/native_image_switch_test.go`) compares the reads after the switched service's deploy has found the image's port, with `port` and `openPorts` no longer excluded. Before that deploy, the switched service reads the port its native release still serves, as Render's `openPorts` reports the running instance. A private variant covers t001's adopted port.
3. **Docs.** ADR089 § Port slice decisions, ADR004's source switch and ADR018's port rows say why a switch keeps the configured-port mode and how the switched image's port is found.
4. **Production**, after a roll that carries w8/m461 (`0d5ef6256`) and this milestone, in `bex-canary`:
   - A free native Node web service (`render-examples/express-hello-world`, `--runtime node --build-command yarn --start-command 'node app.js'`) switched with `bex services update <id> --image docker.io/library/nginx:1.27-alpine --confirm` and deployed goes `live`. Its URL answers 200 with the nginx welcome page, the deploy log names the detected port 80, and `GET /v1/services/<id>` reads `port: 80` and `openPorts: [{port: 80, protocol: TCP}]`.
   - A Starter native private service (private services are paid-only, as on Render) switched to `docker.io/library/redis:7-alpine` and deployed goes `live` on 6379, and `openPorts` lists 6379. It is deleted as soon as this is read.
   - Every fixture is deleted by ID, and each delete is confirmed with a 404.

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
