# CLI sweep 1266 — edge header fixes already live through GitOps; release-pipeline fixes pending (live, 2026-10-20)

Production image pin is still `9cace6b6e25d`. Deploy run 39470664194 for `e1973aece` passed all test jobs but failed in `build` twice: attempt 1 in the dashboard CVE-report step, attempt 2 when the runner timed out reaching `registry-1.docker.io` while booting buildx. `deploy` was skipped both times. The tenant edge's Traefik is synced by GitOps from `main`, so the Traefik halves of w8/m123 and w8/m125 can be checked before the image pin moves.

- Fixture (deleted at sweep end, 204 then GET 404): `qa-20261020-19655f-echo` (`srv-dbbsedhv5vjs73b9h9bg`), created with `bex services create --name … --type web_service --plan free --image docker.io/ealen/echo-server:0.9.2 -o json --confirm` in bex-canary. Live in 22 s. An earlier create attempt (`qa-20261020-65979b-echo`) failed with a client-side `TLS handshake timeout` before the request was sent; a by-name list confirmed nothing was created.
- The live deploy row has `image: {"ref": "docker.io/ealen/echo-server:0.9.2"}` and no `sha`, as expected on the pre-m122 build.

## w8/m123 — client-identity headers (web service): passes live

`GET /cf` with `CF-Connecting-IP: 203.0.113.66`, `True-Client-IP: 203.0.113.77`, `X-Client-IP: 203.0.113.88`, `CF-IPCountry: KP`: the container received `true-client-ip`, `x-real-ip` and `x-forwarded-for` all equal to the real client address (`<client-ip>`), and no `cf-connecting-ip`, `cf-ipcountry` or `x-client-ip`. A plain request carries the same `true-client-ip` = `x-real-ip`.

## w8/m125 — `Rndr-Id` (edge half): passes live

Both requests got an `Rndr-Id` response header (UUID form), and the app received the same value in `rndr-id`. The client-supplied `Rndr-Id: forged-rndr-1234` was replaced. The `requestID` in request logs needs the bex-api release; checked below once the pin moves.

## Docker `rootDir` + `dockerContext`: filed as w8/150 and w8/151

Public repo `render-examples/twingate-example` (`example-pserv/Dockerfile` copies `example-pserv/index.html`; the repo root has no `index.html`). Free Docker web services, created through REST because the pinned CLI has no `dockerContext` flag:

| Fixture | Body | Result |
| --- | --- | --- |
| `qa-20261020-a18296-dctx` `srv-dbbu8m65u6js73eso1pg` | `rootDir: example-pserv`, `dockerContext: "."`, `dockerfilePath: "./Dockerfile"` (Render's own export shape) | `build_failed` in 78 s. Dockerfile found under `example-pserv/` (115 B), context = repo root (`transferring context: 2B`), `COPY index.html` → `"/index.html": not found` |
| `qa-20261020-e89060-dctl` `srv-dbbuctu5u6js73eso1s0` | same without `dockerContext` | build succeeded (`COPY index.html` → `DONE`). Read-back reports `dockerContext: "example-pserv"` (Render: `.`). The container then crash-looped on the recorded `nginx:alpine` `chown` posture (sweep 454), so it ended `update_failed` at 428 s; that is not a new finding |
| PATCH on `…dctx` | `serviceDetails.envSpecificDetails.dockerContext: "example-pserv"` | 400 `request body contains unknown field "dockerContext"` (Render's `dockerDetailsPATCH` has the field) |

Render's semantics come from its pinned Blueprint schema (`buildSourceGit.dockerContext`: "relative to rootDir. Defaults to rootDir…"), its monorepo guide, a Render-generated export (`aliyevaladddin/AladdinAI`: `rootDir: backend`, `dockerContext: .`) and a user report of path doubling (`AjayMaan13/FinSight`). Details are in [150](../../150.md). Both Docker fixtures were deleted (204, then GET 404).

## Not filed

- Bex injects no Render default environment variables (`RENDER`, `RENDER_EXTERNAL_HOSTNAME`, `RENDER_SERVICE_ID`, `IS_PULL_REQUEST`, …), only `PORT`. ADR018 records this as deliberate. Consequence worth noting for the decision: Render's Django guide sets `DEBUG = 'RENDER' not in os.environ`, so that app runs with `DEBUG=True` on bex.
- HTTP→HTTPS: GET 301, POST 308; HTTP/2 negotiated. Fine.
- Env var key rule (letters, digits and underscores only): Render documents no key rule, so there is no contract to compare against.
- Slow uploads past 60 s: existing w8/m108 (blocked on the read-budget decision).
