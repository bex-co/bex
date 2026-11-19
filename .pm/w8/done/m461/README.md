# w8 · m461 — A service switched from a native runtime to a prebuilt image runs the image, and reads as an image service

**Worker:** worker8 **Goal:** `bex services update <native service> --image <ref>` leaves a service that deploys the image with its own entrypoint, reads `runtime: image` on every surface, and clones with the pinned CLI, the way an image-created service does. **Status:** done (t001–t006 2026-11-17; t007's production replay passed on pin `dff52c596d13` 2026-11-19)

## Tasks (in order)

| id   | title                                                                                                                  | est | depends_on |
| ---- | ---------------------------------------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Report an image-only source as runtime `image`, whatever native runtime the service retains for a switch back — **DONE** | 30m | —          |
| t002 | A switch from a native runtime to an image stops carrying the native start and build commands into the image's command — **DONE** | 45m | t001       |
| t003 | Blast radius: every source-switch caller, service type and source kind — **DONE** | 30m | t002       |
| t004 | Render parity — **DONE** | 20m | t003       |
| t005 | Simplify — **DONE** | 15m | t004       |
| t006 | Test coverage — **DONE** | 30m | t004       |
| t007 | Closeout — **DONE**                                                                                                               | 15m | t006       |

## Definition of done

Verified on production after a roll that carries the change, with bex CLI v0.7.0 or later, in `bex-canary`:

1. Create a free native Node web service: `bex services create --name <fresh> --type web_service --repo https://github.com/render-examples/express-hello-world --branch main --runtime node --plan free --build-command yarn --start-command 'node app.js' --confirm -o json`. Wait for Live.
2. `bex services update <id> --image docker.io/jmalloc/echo-server:v0.3.7 --confirm -o json` exits 0. `GET /v1/services/<id>` then reads `serviceDetails.runtime: image` and `env: image`, `imagePath` the new image, and no `buildCommand`, `startCommand` or `dockerCommand` in `envSpecificDetails`. GraphQL `service(id:) { runtime startCommand buildCommand }` agrees.
3. `bex deploys create <id> --wait --confirm -o json` exits 0, the deploy is `live`, and the public URL answers 200 with echo-server's `Request served by` body (it listens on `$PORT`, so the service's port 10000 still works).
4. `bex services create --from <id> --name <fresh clone> --plan free --region frankfurt --confirm -o json` exits 0 and creates an image service on the same image.
5. Controls:
   - `bex services update <id2> --image <ref> --start-command '<cmd>'` on a second fresh native service still saves `<cmd>` as the Docker Command and runs it (w4/m202's behavior).
   - An image-created echo-server service still deploys and reads as before.
   - Switching back with `--repo … --branch main` (plus `--start-command 'node app.js'` if t002 clears the native command) reads `runtime: node`, and its deploy serves `Hello from Render!`.
6. Delete every fixture by ID and confirm 404s.

## Results (2026-11-17)

The fix: `core.EffectiveRuntime` reads an image with no repo as runtime `image` whatever runtime it keeps (t001), and the one source writer, `sourceFields.applyTo` → `settleSourceSwitch`, drops the build command and a start command the build baked into its image on a switch to a prebuilt image (t002). [ADR004](../../../../docs/ADR004-app-deployment.md) § w8/m461 records the rule: a command that already was a Docker Command carries over, a same-PATCH command runs, and the dropped commands are not restored by a switch back.

### Blast radius (t003)

| Caller | Writes the source through | Result |
| --- | --- | --- |
| REST `PATCH /v1/services/{id}` `image` | `servicePatchTable` source row → `SetSourceAndRegistryCredential` → `applySourceChange` → `applyTo` (the preflight probe too) | rule holds; a same-PATCH `startCommand` or `dockerCommand` row runs after it and wins |
| GraphQL `setImage` (dashboard Settings → Source → Existing Image) | `SetSourceAndRegistryCredential` | rule holds |
| MCP `update_service` `image` | the same patch table as REST | rule holds |
| Blueprint sync to `runtime: image` | its own appliers (`probeBlueprintServiceSpec`); `runtime: image` is declared, since render.yaml requires a runtime | the same rule for a service that builds itself (w8/m70); a declared command wins. A sync does not read a linked service's carrier, so it keeps a linked command |
| Build source link → image | `applyTo` clears `spec.buildSource`; `checkSourcePatch` reads the carrier (`linksNativeBuildSource`) | a native source's start command (and a linked cron's `command`) is dropped; a Docker or image source's `dockerCommand` (w3/069) carries over |

| Source left | web, private, worker | cron |
| --- | --- | --- |
| native runtime | build + start dropped, runtime kept | the same; its native start command is dropped |
| buildpack | start dropped | start dropped |
| Dockerfile (`runtime: docker`) | start (the Docker Command) kept | `command` kept |
| native cron with an explicit `command` override | — | override kept (it already ran through the shell) |
| native build source link | start dropped | `command` dropped |
| Docker build source link | `dockerCommand` kept | `command` kept |
| another image | Docker Command kept | `command` kept |
| image → Git (w8/088) | unchanged: reads its Docker build | unchanged |

Pinned by `TestSourceSwitchToImageAcrossCallersTypesAndSources` (75 cases: every caller × type × source). Dashboard: Settings → Source → Existing Image sends only `setImage(id, image, registryCredentialId)`. Its Deploy card shows `startCommand` as the Docker Command, which read `node app.js` before the fix and is now empty. No dashboard change was needed.

**Production count: not read.** `scripts/fetch-app-kubeconfig.sh` refuses to run because this checkout's `.env` is mode 664 (it holds production credentials; the fix is `chmod 600 .env`, ADR079 finding 1), and this host has no `kubectl`. Changing the user's credential file was out of scope. A service switched before the fix keeps running its former native start command as a Docker Command. It now reads that way (`dockerCommand` on REST, the Docker Command field in the dashboard), so its owner can see and clear it. No one-off migration was made.

### Render parity (t004)

- `TestSwitchedServiceReadsAsAnImageCreatedOne` creates a native Node service and an image service through REST, switches the first, and diffs the reads without identity and history. Every field matches except the port: an image-created web service serves on its image's exposed port (`portMode: web-image-v1`, stamped at create only, w4/m243), while a switched one keeps its configured port 10000. Filed as w8/337. The definition of done's step 3 already relies on the kept port.
- GraphQL `service { runtime startCommand buildCommand }` reads `image`, `""`, `""`; MCP `get_service` reads runtime `image` with Render's image `envSpecificDetails` (an empty `dockerCommand`, no build or start command, w9/m473); the dashboard reads the same GraphQL fields (Source and Deploy sections, no Runtime or Build section without a repo).
- [ADR018](../../../../docs/ADR018-render-parity.md)'s runtime row records the switch as an extension and its Render basis: Render's docs don't say whether a Git-backed service can switch to an image, and an image-backed service reads `runtime: image` with an optional Docker Command.

### What t007 replays

The definition of done above on production, after the roll that carries this change. With the pinned CLI, switch back in two calls: an image service refuses `--start-command` client-side (`pkg/service/update.go`), so first send `--repo … --branch main`, then `--build-command yarn --start-command 'node app.js'`.

## Unblocked (2026-11-19, `/loopx w8` worker)

- **Affected:** t007. **Source:** verified evidence, no user decision.
- **Finding:** the newest pin on `origin/main`, `a91e9aeb3` (`chore(deploy): pin platform images to dff52c596d13`, 2026-11-19 11:30Z), carries this milestone's commit: `git merge-base --is-ancestor 0d5ef6256 dff52c596d13` holds. The previous pin, `4b2cebd0bd03`, already carried it.
- **w8/m504 on the same pin:** the pin also carries `cf5f56884`, which builds on this milestone (a switched service goes live on the port its image binds). echo-server listens on `$PORT`, so step 3's expectations do not change; m504's own replay stays with its own t007.
- **Status:** pending verification. A deployed bex-api is not proof that the definition of done holds on production.
- **Next:** t007 replays the definition of done in `bex-canary` with bex CLI v0.7.0, records the commands and answers here, deletes every fixture by ID, and closes the milestone.

## Production replay (t007, 2026-11-19 11:48–11:58 UTC)

Production pin `dff52c596d13`, `https://api.bex.co/v1/`, bex CLI v0.7.0 (Render CLI v2.36.0, release checksum verified), QA user by device OAuth in an isolated config, workspace `bex-canary` (`tea-daif693dqjvc73e7as3g`). Every mutation ran with `--confirm </dev/null`. All steps pass.

1. **Create.** `bex services create --name qa-20261119-m461-a … --runtime node --plan free --build-command yarn --start-command 'node app.js' --confirm -o json` exited 0 with `srv-dbve3te014qc73acu7f0`. Its first deploy went `live` at 11:52:38 and the URL answered 200 with `<title>Hello from Render!`.
2. **Switch.** `bex services update srv-dbve3te014qc73acu7f0 --image docker.io/jmalloc/echo-server:v0.3.7 --confirm -o json` exited 0. `GET /v1/services/<id>` then read `imagePath: docker.io/jmalloc/echo-server:v0.3.7`, no `repo` or `branch`, `serviceDetails.runtime: image`, `env: image`, and `envSpecificDetails: {dockerCommand: "", dockerContext: "", dockerfilePath: ""}`, the same as the image-created control below (no `buildCommand` or `startCommand`, an empty `dockerCommand`). GraphQL `service(id:)` read `runtime: "image"`, `startCommand: ""`, `buildCommand: ""`. (Before the fix: `runtime: node`, `yarn`, `node app.js`.)
3. **Deploy.** `bex deploys create srv-dbve3te014qc73acu7f0 --wait --confirm -o json` exited 0 after 25 s: `dep-dbve6a6014qc73acu7kg` `live`, trigger `api`, image `docker.io/jmalloc/echo-server:v0.3.7` (`sha256:c87f80de8dbb…`). The URL answered 200 with `Request served by srv-dbve3te014qc73acu7f0-…` on the kept port 10000. (Before the fix: `update_failed`, `the startup program "node" does not exist in this image`.)
4. **Clone.** `bex services create --from srv-dbve3te014qc73acu7f0 --name qa-20261119-m461-clone --plan free --region frankfurt --confirm -o json` exited 0 with `srv-dbve6he014qc73acu7lg`: `imagePath` echo-server v0.3.7, `runtime: image`, the same empty image `envSpecificDetails`. Its deploy went `live` and its URL answered 200 with `Request served by srv-dbve6he014qc73acu7lg-…`. (Before the fix: rc 1, `--build-command and --start-command are only supported for native runtimes`.)
5. **Controls.**
   - A second fresh native service, `srv-dbve3ve014qc73acu7h0`, went live on `Hello from Render!`. `bex services update <id> --image docker.io/library/busybox:1.36.1 --start-command 'mkdir -p /tmp/w && echo m461-control-docker-command > /tmp/w/index.html && exec httpd -f -p $PORT -h /tmp/w' --confirm` exited 0. REST read `runtime: image` with that command as `dockerCommand`, and GraphQL `startCommand` the same. Its deploy `dep-dbve6oo1ks2s73a6pf60` went `live` and the URL answered 200 with `m461-control-docker-command`, so a same-call Docker Command still runs (w4/m202).
   - An image-created echo-server service, `srv-dbve3vg1ks2s73a6pf3g` (`--image docker.io/jmalloc/echo-server:v0.3.7 --plan free --region frankfurt`), went `live` in about 40 s, answered 200 with `Request served by …`, and read `runtime: image`, `env: image`, `envSpecificDetails: {dockerCommand: "", dockerContext: "", dockerfilePath: ""}`, `port: 10000`. GraphQL read `runtime: "image"` with empty commands.
   - Switching back: `--repo https://github.com/render-examples/express-hello-world --branch main` exited 0 and read `runtime: node`, `env: node` with empty `buildCommand` and `startCommand`. The switch dropped them and the switch back does not restore them, as ADR004 records. A second call, `--build-command yarn --start-command 'node app.js'`, exited 0 and read both on REST and GraphQL. `bex deploys create … --wait` exited 0: `dep-dbve72g1ks2s73a6pf70` `live` on commit `039c34770852`, and the URL answered 200 with `<title>Hello from Render!`.
6. **Cleanup.** `bex services delete <id> --confirm` exited 0 for all four fixtures. `GET /v1/services/<id>` answered 404 for each, and each `https://qa-20261119-m461-{a,b,img,clone}.onbex.co/` answered 404. `hello-go` and the other runs' services in `bex-canary` were not touched. The QA dashboard sessions were logged out with `qa-login.sh --logout`, and the isolated CLI config holding the device token was deleted (the grant was not revoked, since that ends every QA CLI session).

GraphQL `builder` still reads `native` on the switched service (`auto` on the image-created one). ADR004 keeps the builder, like the runtime, for a later switch back, and no read of an image service uses it.

## Finding

**Source:** `/qa-find-bugs-cli` sweep `0594`, 2026-11-17 06:52–07:00 UTC, production `https://api.bex.co/v1/`, pin `21762a1df1fb`, bex CLI v0.7.0 (Render CLI v2.36.0, module `v1.1.3-0.20261106232710-4fd34231b637`), human device OAuth, workspace `bex-canary` (`tea-daif693dqjvc73e7as3g`). **Severity:** major (a documented bex capability leaves a service that cannot deploy, and its reads mislead the pinned client). **Attribution:** server (source switch and runtime projection in `lego/backend/internal/apps`).

### Repro

Fixture `qa-20261117-0594-upd` (`srv-dbtvjcivk30s73cqtabg`, since deleted), created as in step 1 above. Its first deploy went live at 06:57 and served `<title>Hello from Render!` with 200.

```sh
bex services update srv-dbtvjcivk30s73cqtabg --image docker.io/library/nginx:1.27-alpine -o json --confirm   # rc 0, 1.2 s
```

The pinned client sends `PATCH /v1/services/srv-dbtvjcivk30s73cqtabg` with `{"image":{"imagePath":"docker.io/library/nginx:1.27-alpine","ownerId":""}}` (`pkg/service/update.go:39-49`). The REST read afterwards (owner fields dropped):

```text
imagePath: docker.io/library/nginx:1.27-alpine    repo: (absent)   branch: (absent)
autoDeploy: no   autoDeployTrigger: off
serviceDetails.runtime: node   serviceDetails.env: node
serviceDetails.envSpecificDetails: {buildCommand: yarn, preDeployCommand: "", startCommand: node app.js}
```

GraphQL reads the same: `{"buildCommand":"yarn","builder":"native","imagePath":"docker.io/library/nginx:1.27-alpine","repo":"","runtime":"node","servingImage":"","startCommand":"node app.js"}`.

Then the deploy:

```sh
bex deploys create srv-dbtvjcivk30s73cqtabg --wait --confirm -o json   # rc 1 after 37 s
# stdout: dep-dbtvlijvsrns73ebrkbg update_failed trigger api image {ref: docker.io/library/nginx:1.27-alpine}
```

GraphQL `deploy(serviceId:, deployId:) { failureReason }`:

```text
container could not start: the startup program "node" does not exist in this image. The program never ran, so there is no application output in the logs. Check the Docker Command and the image's entrypoint.
```

The previous release kept serving (200), as it should. The user never set a Docker Command: the native Start Command became one.

A plain clone of the switched service also fails, before any request is sent:

```sh
bex services create --from srv-dbtvjcivk30s73cqtabg --name qa-20261117-0594-clone --plan free --region frankfurt --confirm -o json
# rc 1, 0.7 s: Error: --build-command and --start-command are only supported for native runtimes
```

Two more reads from the same fixture:

- **Switching back** with `--repo https://github.com/render-examples/express-hello-world --branch main` reads `runtime: node`, `yarn` and `node app.js` again (ADR004's retain-for-switch-back works).
- **Re-switching** with `--image … --start-command 'nginx -g "daemon off;"'` replaces the retained native start command, because one field holds both commands.

### Cause

- `lego/backend/internal/apps/service.go:5163-5175` (`sourceFields.applyTo`) moves repo, image, branch and credential, and clears a runtime only through `staleImageRuntime` (`build_strategy.go:105-111`, w8/088). That covers only image → Git. A native `runtime`, `buildCommand` and `startCommand` stay, which ADR004's w4/m202 paragraph intends for the runtime, builder and root directory ("for a later switch back").
- `lego/backend/internal/core/source_runtime.go:45-48` (`EffectiveRuntime`) returns any explicit runtime before looking at the source. So an image-only App reads `runtime: node` (`apps/service.go:1196`). `render.go:618-654` then emits the native `envSpecificDetails` with `buildCommand` and `startCommand`, where an image-created service gets no build block.
- The operator runs `spec.StartCommand` as the image's `/bin/sh -c` override whenever the saved source is a prebuilt image (`lego/operator/internal/controller/deployment_projection.go:321`, `shellRunsStartCommand`, w4/m202). The native Start Command and the Docker Command share `spec.StartCommand`, so the switch turns `node app.js` into the image's command.
- **Pinned consumer:** `pkg/service/update.go:74-93` picks the update body builder from `serviceDetails.runtime`, so later `--start-command` and `--build-command` saves go out as native fields on an image service. `pkg/service/clone.go:242-256` treats a source with `imagePath` as runtime `image` but inherits `buildCommand`/`startCommand` (`clone.go:274-275`), which `pkg/types/service/servicecreate.go:116` refuses.

### Render

Render's docs don't say whether a Git-backed service can switch to an image ([deploying-an-image](https://render.com/docs/deploying-an-image), [update-service](https://api-docs.render.com/reference/update-service), read 2026-11-17). They do describe an "image-backed service, as opposed to a Git-backed service", whose settings are the image URL, the credential and an optional Docker Command. An image-backed service reads `runtime: image`. bex supports the switch by design (MCP `update_service` `image`: "Setting it switches a Git-backed service to an image and clears the repository", `apps/mcp.go:94`), so the result must behave like an image-backed service.

### Dedupe

- **w8/088** (done) fixed the opposite direction: an image runtime kept after a switch to Git. Its `staleImageRuntime` is the pattern to mirror.
- **w4/m202** (done) made a Docker Command saved after a native → image switch run. This milestone keeps that working and stops an unsaved native command from being treated as one.
- **w4/418** (done): an image service refuses a build command on update. The switched service still holds `yarn`, a state w4/418 refuses.
- **w8/075** (blocked, upstream): clone overrides that change source kind. Not this: here the source's own read is wrong, and a clone with no overrides fails.
- No open item mentions the native → image direction (`rg -i 'existing image|switch.*image'` over `.pm` outside `done/`). There is no newer fix in `git log` since pin `21762a1df1fb`.

### Unverified

- Whether Render's API accepts `image` on a Git-backed service at all. No Render account is available, and the docs are silent.
- Behavior for a Dockerfile (`runtime: docker`) service, a cron job (whose native command is its start command), and private services and workers. These are expected to share the same path; t003 checks them.
- Whether the dashboard's Settings → Source → Existing Image save fills the Docker Command field with the retained native command. Its GraphQL read shows `startCommand: node app.js`.
- How many production services are already in this state. Each would need its command cleared, or a one-off fix.

## Source + Goal linkage

- **Source:** `/qa-find-bugs-cli` sweep `0594` (2026-11-17), a `bex services update` flag round trip on one free native web service. The other flags round-tripped cleanly:
  - `--auto-deploy` on and off;
  - `--max-shutdown-delay` 1 and 300, with 0 and 301 refused "must be 1-300";
  - `--health-check-path` set and cleared, with `healthz` refused "must start with /";
  - include-only, ignore-only and combined build filters;
  - `--root-directory` set and cleared, with `../x` refused;
  - `--pre-deploy-command` on free refused with Render's paid-only wording;
  - an unknown `--plan` refused.
- **Goal linkage:** ADR008 core hosting and Render parity first; ADR004 application deployment (w4/m202 source switches); ADR018 Docker Command parity.
- **Expected outcome:** moving a service from Git builds to a CI-built image is one `services update --image` and one deploy. That service reads, updates and clones like an image-created one.
- **Why now:** the switch is offered on REST, GraphQL and MCP and reachable from the pinned CLI. Today it ends in a failed deploy with a message blaming a Docker Command the user never set. The read also misleads the pinned client's update and clone.
- **Render parity included:** the fix changes REST, GraphQL and MCP reads and the dashboard's Settings → Source save.
