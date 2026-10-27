# w4 · m243 — A Docker web service listens on its image's exposed port, as on Render

**Worker:** worker4 **Goal:** a new Docker-built or prebuilt-image web service with no `PORT` env and no explicit port serves on its image's `EXPOSE`d port (hello-go 3000, whoami 80) instead of silently defaulting to 10000. **Status:** todo — t001–t005 done 2026-10-25, t007–t008 done 2026-10-26, t009–t011 done 2026-10-27 (loopx w4); t012 opened 2026-10-27 by live QA run e9ba (every rollout of an image-port service leaves its Service with no ready endpoints at promotion: raw `503 no available server` for 1–5 s); t006 then waits for the release pipeline to deploy the operator, activator, bex-api and dashboard, then QA replays the DoD live in `bex-canary`

## Tasks (in order)

| id   | title                                     | est | depends_on      |
| ---- | ----------------------------------------- | --- | --------------- |
| t001 | Operator: web image-port discovery serves — **DONE** | 45m | —               |
| t002 | Backend: stamp the web port mode at create — **DONE** | 20m | w4/m243/t001    |
| t003 | Render parity — **DONE** | 15m | w4/m243/t002    |
| t004 | Simplify — **DONE** | 10m | w4/m243/t003    |
| t005 | Test coverage — **DONE** | 20m | w4/m243/t004    |
| t007 | Ingress follows the serving release's port until promotion (port change / failed deploy never takes the host down) — **DONE** | 1h15m | w4/m243/t005 |
| t008 | Read Docker Hub once per release; a registry 429 retries instead of failing the deploy — **DONE** | 1h | w4/m243/t005 |
| t009 | Held wake request to a sleeping image-port service is proxied to port 10000 and fails 502 after 90 s — **DONE** | 45m | w4/m243/t005 |
| t010 | Dashboard create form and Settings treat Docker/image web services as image-port services (no pinned 10000) — **DONE** | 45m | w4/m243/t005 |
| t011 | Rollout stall hint names an explicit/PORT primary as configured, not as an image-declared port — **DONE** | 20m | w4/m243/t005 |
| t012 | An image-port service's rollout leaves its Service with no endpoints at promotion (raw 503 "no available server" for 1–5 s) | 1h15m | w4/m243/t005 |
| t006 | Closeout                                  | 10m | w4/m243/t005, t007, t008, t009, t010, t011, t012 |

## Definition of done

- t001–t005 complete with operator (envtest) and backend suites green: a new Docker/image web service's `Service`/`Ingress` and status port come from its image's TCP `EXPOSE` set; a `PORT` env or explicit port still wins; existing services (no web port mode) are untouched.
- **Live check (post-deploy, QA):** deploy hello-go (Docker, repo-built) and a prebuilt whoami image as new web services in `bex-canary` with no `PORT` set → `serviceDetails.port` and `openPorts` report 3000 / 80 and each URL answers its own response. (Held on pin `b4726243e74f` for whoami, run 661f; hello-go not yet replayed.)
- **t007 live check:** on an owned image web service polled every 1 s, a `PORT=9999` deploy (image listens on 80) fails while the host keeps answering `200`; an echo `HTTP_PORT`/`PORT` `8080 → 9090` change stays `200` throughout; `openPorts`/GraphQL `port` report the serving port until promotion; a rollback after a failure goes `live`.
- **t008 live check:** image deploys and redeploys never end `update_failed` with `image metadata request returned status 429`; a throttled read leaves the deploy open with a waiting reason.

- **t009 live check:** a sleeping Free `nginx-unprivileged` (EXPOSE 8080) service answers a bare `curl` and an `Accept: application/json` request with the app's `200` within the wake time; HTTP latency returns series for it.
- **t010 live check (run 828c):** dashboard New Web Service → Existing Image `docker.io/traefik/whoami:latest` with every default → the captured `CreateService` carries no `port`, the service reports `port 80`/`openPorts [80]`, the first deploy is `live` and the host answers `200`; typing `8080` still sends `port: 8080`; Settings → Port on that service never reads "(default 10000)".
- **t011 check:** an image web service with an explicit port its image does not serve stalls with a reason naming that port as configured and listing only the image's declared ports.

- **t012 live check (run e9ba):** on an owned image web service (and an owned Git-built Docker private service, polled internally), a 0.25 s poll across a Manual Deploy and an env-group "Save and deploy" answers `200` every time — never `503 no available server`.

## Source + Goal linkage

- **Source:** inbox note w4/388 (live QA run r7k2 evidence: port detection misses on first deploys, 2026-10-22), promoted in the loopx w4 drain 2026-10-25 because it spans types, operator and backend.
- **Goal linkage:** [ADR008](../../../docs/ADR008-vision.md) Render parity first — Render serves a Docker web service on its image's exposed port; bex's 10000 default makes first deploys 502.
- **Expected outcome:** repo-built and prebuilt Docker web services answer on their real port with no `PORT` workaround.
- **Why now:** every Docker first deploy without `PORT` hits this; the private-service image-port machinery (w6/m148) already exists to extend. Render parity is included as t003 because service reads (`serviceDetails.port`, `openPorts`) are user-facing.
