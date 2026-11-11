# w4 · m323 — A private service created in the dashboard from a prebuilt image is pinned to port 10000, so an image listening on its declared port never goes live, and the stall hint's "remove it" fix exists nowhere

**Worker:** worker4 **Goal:** New Private Service → Existing Image takes its ports from the image, as the backend has since w6/095 and as the hint on the same form promises. Every remediation the rollout stall reason names can actually be done. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | The create form treats a prebuilt-image private service as an image-port service: blank Port, no `port` sent | 30m | — |
| [t002](t002.md) | Every remediation the rollout stall reason names is reachable: clearing the port of an image-port service returns it to the image's port | 45m | — |
| [t003](t003.md) | Blast radius: the form shape's consumers, the stall-reason branches, and regression tests on the shapes that work today | 20m | t001, t002 |
| [t004](t004.md) | Live verification in `bex-canary` | 30m | t003 |
| [t005](t005.md) | Render parity | 15m | t004 |
| [t006](t006.md) | Simplify | 10m | t005 |
| [t007](t007.md) | Test coverage | 30m | t005 |
| [t008](t008.md) | Closeout | 10m | t006, t007 |

Eight tasks, 190m (3h10m).

**Major.** With every field but name and image left at its default, the dashboard's private-service path creates a first deploy that never goes live, and the service is unreachable from its peers meanwhile. The page told the user not to set a port.

## Finding

Live `/qa-find-bugs w4` loop, round 82, 2026-11-11 07:21–07:31 UTC (`muse.env`, workspace `bex-canary` on the Scale plan; production runs the code of `348bd77a5`).

Fixtures, all owned and deleted afterwards (DELETE 204, then GET 404):

- `qa-20261111-pv82` (`srv-dbq1f7qk10fs73dvot70`), created in the dashboard;
- `qa-20261111-pv82b` (`srv-dbq1hfqp92ls73cjjjsg`), created over REST as the control;
- `qa-20261111-pc82` (`srv-dbq1i3ik10fs73dvotp0`), a busybox cron job used as an in-network peer.

### Probes

All API reads ran from the signed-in dashboard page with `fetch(…, {credentials: "include"})`.

- **The form contradicts itself.** `/services/new?type=private_service` → Existing Image. Under the Docker Image box the form says "bex uses the port the image declares (EXPOSE). Set the port below only if the image listens on a port it doesn't declare." Below that, `textbox "Port"` is pre-filled `10000`, with "Your container must listen on port 10000 — bex routes traffic there and injects it as $PORT."
  - **Control:** the same tab with Web Service selected shows Port blank, placeholder "From the image", help "Optional. bex publishes every TCP port the image declares (EXPOSE) …".
- **Dashboard create, defaults kept.** Image `docker.io/traefik/whoami:v1.10.1` (`EXPOSE 80`), name `qa-20261111-pv82`, Port left at `10000`, Starter → Deploy Service at 07:22:07Z.
  - `GET /v1/services/srv-dbq1f7qk10fs73dvot70` returns `serviceDetails.port 10000`, `internalAddress "qa-20261111-pv82:10000"` and `openPorts [{80,TCP},{10000,TCP}]`.
  - The app log reads `Starting up on port 80`.
  - At 07:29:28Z, after 7 min 21 s, the deploy is still `update_in_progress`, with `stallReason`: "… a TCP connect to port 10000 … Nothing accepted a TCP connection on port 10000, set by the service's configured port; the image declares TCP ports (80). Change the service's port in Settings to the one it listens on, or remove it to use the image's port."
- **REST control, no `port`.** `POST /v1/services` `{"type":"private_service","name":"qa-20261111-pv82b","image":{"imagePath":"docker.io/traefik/whoami:v1.10.1",…},"serviceDetails":{"plan":"starter","runtime":"image"}}` → 201.
  - The deploy is `live` about 45 s later.
  - The service reads `port 80`, `internalAddress "qa-20261111-pv82b:80"`, `openPorts [{80,TCP}]`.
- **Reachability from a peer.** A manual run of the cron job, `wget -qO- -T 5` per URL:
  - `http://qa-20261111-pv82b:80/` and `http://qa-20261111-pv82b/` → whoami's body (`Hostname: tea-daif693dqjvc73e7as3g-qa-20261111-pv82b-76cc464cc7-hhmzl`);
  - `http://qa-20261111-pv82:80/` and `:10000/` → `wget: can't connect to remote host (10.107.8.127): Connection refused`.
- **Not public.** `curl https://qa-20261111-pv82{,b}.onbex.co/` → 404 for both. That is correct.
- **The hint's "remove it" can't be done.** Settings → Port → Edit → clear the field → `spinbutton "Port" [invalid]`, `alert: Enter a port between 1 and 65535.`, and Save changes is disabled.
- **The other fix works.** Setting `80` and saving rolled `service_updated` deploy `…dvou2g` to `live` within ~40 s, and canceled the stuck first deploy. The service then read `port 80`, `openPorts [{80,TCP}]`.

### Root cause

- **1. A stale form predicate.**
  - `dashboard/src/features/services/lib/create-service-input.ts:94-107` (`imagePortShape`) returns `isDockerBuild` for a private service: "A private service on a prebuilt image keeps the configured port". So `portDraft` (`:145`) pre-fills `DEFAULT_SERVICE_PORT`, `portIsValid` (`:151`) rejects a blank port, and the payload (`:277-278`) sends `port: 10000`. `routes/services.new.tsx:510-537` shows the configured-port placeholder and hint.
  - That rule was true when w4/m243/t010 wrote it ("Private + prebuilt image is **not** an image-port shape on the backend … Do not widen it").
  - w6/095 (`d65c5638f`, 2026-10-29) then made the backend stamp `PortModeImageV1` for `TypePrivateService && (source.Repo != "" || source.Image != "")` (`lego/backend/internal/apps/service.go:2725-2733`). It changed no dashboard file and was verified over REST only.
  - With `port` sent, the backend stamps `PortModeImageConfiguredV1` with primary 10000, which the rollout health-checks.
- **2. No way to clear a port.**
  - The configured-port branch of the stall reason (`lego/operator/internal/controller/app_controller.go:5927-5929`) offers "remove it to use the image's port".
  - No surface can do that. `SetPort` (`lego/backend/internal/apps/service.go:5039-5059`) requires `validateServicePort` 1–65535 (`:5111-5115`). The service PATCH rule applies only when `Port != nil` (`lego/backend/internal/apps/settings.go:519-533`). `configuredPortMode` (`settings_check.go:289-298`) only ever moves to the configured variant. The dashboard's `ServicePortRow` (`dashboard/src/features/services/components/service-port-row.tsx:66-73`) validates a blank draft as an error.
  - Nothing returns a service from `image-configured-v1` or `web-image-configured-v1` to `image-v1` or `web-image-v1`.

### Fix (target behavior)

- **t001: the form mirrors the backend.**
  - `imagePortShape` returns true for a private service whose source is a prebuilt image or a Git Docker build, the same predicate as `configureNewImageCompatibility`. Prefer one shared helper.
  - The private + Existing Image form shows Port blank, placeholder "From the image" and `createFieldPortImageHint`, and sends no `port` when blank.
  - An explicit port is still sent, which stamps the configured variant.
  - A native-runtime private service keeps the pre-filled 10000.
  - Update the image hint and port strings' descriptions (en/zh) that name the types.
- **t002: the stall reason names only reachable fixes.** The recommended target is to make clearing real:
  - Settings → Port accepts a blank value on an image-port service (`UsesImagePorts`/`UsesWebImagePorts`).
  - The backend gets a clear path, for example a GraphQL `setPort(id, port: null)`/`clearPort` and a REST PATCH value that means "use the image's port". It returns the service to `image-v1`/`web-image-v1` with `port` 0 and starts a deploy, like a port change does. MCP gets whatever verb REST and GraphQL use.
  - A non-image-port service keeps rejecting blank.
  - If clearing is rejected as a design choice instead, drop "or remove it to use the image's port" from `app_controller.go:5928` and record why.
- **Pre-settle state.** The create form computes its shape synchronously from the selected type and tab, so there is no flash. Settings reads `imagePorts` from the loaded service (`status.ts:117`); until it loads, the row keeps today's behavior.

### Blast radius

- **`imagePortShape`** has one caller, `buildShape` (`create-service-input.ts:135`). Its result reaches:
  - `portDraft`;
  - `portIsValid`;
  - the create payload;
  - `services.new.tsx`'s placeholder and hint.
- **Unaffected:** Blueprint, REST and MCP creates go through the backend default.
- **The stall-reason helper has four branches** (`app_controller.go:5915-5929`). Only the last names "remove it". The other three must not change.

### Unverified

- Whether a private service from a prebuilt image with no `EXPOSE` (the 10000 default) behaves the same when created from the dashboard after t001. It should, and t004 covers it.
- The zh copy of the form's hints. Not opened this round.
- Render's private-service create form for an image. Render detects open ports and asks for none (render.com/docs/private-services); its exact dashboard fields were not opened this round.

### Render

- Render's private services receive traffic on whatever ports they bind; Render detects them, and an image needs no port setting. bex's own Settings help ("This service takes its ports from its image's EXPOSE instructions") already describes that model.

### Dedupe

- **w6/095 (done)** is the backend change this residual follows. Its fix bullet and live check ("ClickHouse prebuilt pserv published `[8123, 9000, 9009]`") ran over REST, not the dashboard.
- **w4/m243/t010 (done)** fixed the same symptom for web services and deliberately left this predicate alone; the rule it relied on was later changed.
- **w4/m243/t011 (done)** wrote the stall reason's configured-port branch.
- **w6/m148 (done)** covers Git-built Docker private services, which are correct today.
- **w8/202** owns native-runtime port detection, which is out of scope.
- `grep -rl "imagePortShape\|remove it to use the image" .pm` finds only m243/t010.
- **Not deploy lag:** `git log 348bd77a5..origin/main -- dashboard/src/features/services/lib/create-service-input.ts` is empty.

## Definition of done

Signed in to `bex-canary` after the release pins this build, with owned fixtures deleted afterwards:

1. **Dashboard create takes the image's port.** New Private Service → Existing Image `docker.io/traefik/whoami:v1.10.1`, every other field at its default.
   - The Port field is blank with "From the image", and the captured `CreateService` variables carry no `port`.
   - The first deploy goes `live`, and `GET /v1/services/{id}` reads `port 80`, `internalAddress "<name>:80"`, `openPorts [{80,TCP}]`.
   - A peer's `wget -qO- http://<name>/` returns whoami's body.
2. **An explicit port is still honored.** The same form with Port `8080` sends `port: 8080`, the deploy stalls naming port 8080 as configured, and Settings → Port `80` brings it live.
3. **The stall reason's fix exists.** On that stalled service, Settings → Port → clear → Save starts a deploy that goes `live` with `port 80`; the equivalent REST or GraphQL call does the same. Alternatively, if t002 dropped the clause, the stall reason no longer says "remove it".
4. **Unchanged.** A Git-built Docker private service (`examples/multi-port`) and an image web service (whoami) still create with no `port` and serve their image's ports. A native web service still pre-fills 10000.
5. **Checks pass.** Dashboard `yarn lint` and `yarn test`; backend `go test ./...` with `BEX_TEST_REQUIRE_DEPS=1` if t002 changes the backend; operator `make test` if the stall text changes.

## Source + Goal linkage

- **Source:** the finding above (round 82 of the `/qa-find-bugs w4` loop, 2026-11-11). The durable evidence is the probe record; no screenshot was kept.
- **Goal linkage:**
  - [ADR089](../../../docs/ADR089-render-container-compatibility.md) § Port slice decisions and [ADR004](../../../docs/ADR004-app-deployment.md) (deploys and ports).
  - [ADR008 § Render parity first](../../../docs/ADR008-vision.md#render-parity-first) and [ADR018](../../../docs/ADR018-render-parity.md).
  - [ADR006](../../../docs/ADR006-bex-api.md): if t002 adds a clear path, REST, GraphQL and MCP move together.
- **Expected outcome:** the dashboard's default private-service-from-image path goes live on the image's port, as the API path already does. A stalled service can be fixed the way its stall reason says.
- **Why now:** this is the default path through the form, and the form's own hint steers users into it. w6/095 made the dashboard the only create surface still on the old rule.
- **Render parity included (t005):** the create form and Settings change, and t002 may add a REST, GraphQL and MCP verb.
