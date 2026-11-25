# w6 · m639 — A WebSocket through the edge stays open past 100 minutes, as Render enforces no maximum duration

**Worker:** worker6 **Goal:** a WebSocket that pings through `wss://<service>.onbex.co` stays open for hours, as on Render, instead of being cut at the edge's 6,000 s read deadline, while ADR022 still says what bounds a slow plain request. **Status:** done

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [Measure a pinged WebSocket through the production edge for 2.5 hours](done/t001.md) — **DONE** | 1h | — |
| t002 | [Choose how an upgraded connection escapes the edge's 100-minute read deadline, and amend ADR022](done/t002.md) — **DONE** | 45m | t001 |
| t003 | [Change the edge and its gitops-validate guard](done/t003.md) — **DONE** | 45m | t002 |
| t004 | [Production check: a pinged WebSocket stays open 3 hours through the edge](done/t004.md) — **DONE** | 1h | t003 |
| t005 | [Render parity](done/t005.md) — **DONE** | 30m | t004 |
| t006 | [Simplify](done/t006.md) — **DONE** | 20m | t005 |
| t007 | [Test coverage](done/t007.md) — **DONE** | 45m | t005 |
| t008 | [Closeout](done/t008.md) — **DONE** | 10m | t006, t007 |

## Definition of done

- On production, two WebSocket connections to a paid web service through `wss://<slug>.onbex.co`, each sending a ping every 30 s, are still open after 3 hours (t004). t001 records whether and when they closed before the change.
- A slow plain request body is handled as amended ADR022 says.
- `scripts/gitops-validate.sh` requires the new edge setting on both entrypoints in base and production and refuses `readTimeout: 6000s`.
- ADR022 records the change and its residual, and ADR018 has a WebSockets row.

## Source + Goal linkage

- **Source:** `/pm-brainstorm one aspect of render parity to w6` 2026-11-25 (service networking: the private network and WebSockets, at `fe4d53addd`), item 1, filed by `/pmx them to w6`. Both tenant entrypoints set `respondingTimeouts.readTimeout: 6000s` (`deploy/gitops/base/values/traefik.values.yaml`, w8/207), and [w6/149](../blocked/149.md) measured that Traefik v3 sets that deadline on each accepted connection. An outside measurement found WebSockets through Traefik closed at exactly `readTimeout` (github.com/CryptoManufaktur-io/central-proxy-docker/pull/35). So bex's tenant WebSockets, and its own Shell if it uses the same entrypoints, are probably cut at 100 minutes. That is inferred, not measured: t001 measures first.
- **Render:** "Render does not enforce a maximum duration for WebSocket connections" (render.com/docs/websocket, read 2026-11-25). The page also says a connection can still be interrupted by a deploy or maintenance, and tells clients to reconnect and to ping.
- **Recommendation, not a user decision:** clear the deadline on upgraded connections if the pinned Traefik can; otherwise set `readTimeout: 0` and record the slow-client residual in ADR022. Both rest on ADR008 Render parity first. w8/207's finite bound was chosen for request bodies, and Render documents no WebSocket maximum.
- **Out of scope:** the Hetzner load balancer's 1 h close of a flow that carries no data either way is [w6/149](../blocked/149.md), parked on the user. A pinged connection is not affected.
- **Goal linkage:** Render parity first ([ADR008](../../../docs/ADR008-vision.md#render-parity-first)); real-time apps (chat, live boards, voice) on bex.
- **Expected outcome:** a long-lived WebSocket is no longer forced to reconnect every 100 minutes.
- **Why now:** ADR018 has no WebSockets row, and nobody has measured a long connection through the edge.
- **Render parity:** included: tenant-visible connection behavior.

## Outcome (2026-11-25): already true at the edge, now recorded and guarded

- **t001 measured it, and the premise was false.** Nothing at the edge closes a WebSocket at 100 minutes.
  - Edge: `docker.io/traefik:v3.7.17@sha256:30ff6249…`, both pods (`-2bvp4`, `-fb2b8`) started 2026-11-24T00:23Z and did not roll during the run, `readTimeout=6000s` on `web` and `websecure`. The pin was `d8d0713b48`, then `8b9d780f45` from 17:54Z, which rolled the ssh-gateway but not Traefik.
  - Fixture: `w6-m639-ws-echo` (`srv-dc3gs8dksp0s73b5ovqg`, Starter, `python:3.12-slim` running a standard-library WebSocket echo) in `bex-canary`, created 16:34Z, deleted 19:36:10Z (`DELETE` 204, then `GET` 404).
  - Every connection pinged every 30 s from a workstation outside the cluster. Each got 359 of its 360 pongs (the last ping crossed the close), at about 0.17 s round trip throughout.

  | connection | path | opened | ended | how it ended |
  | --- | --- | --- | --- | --- |
  | `pub-a` | `wss://w6-m639-ws-echo.onbex.co` through the load balancer, to Traefik `-fb2b8` | 16:35:42.540Z | 19:35:44.750Z | held the full 10,802.6 s; the client closed it (1000) |
  | `pub-b` | the same, to Traefik `-2bvp4` | 16:35:42.542Z | 19:35:44.719Z | held 10,802.6 s; the client closed it |
  | `pf-2bvp4` (control) | a port-forward straight to Traefik `-2bvp4:8443`, past the load balancer | 16:35:42.503Z | 19:35:44.721Z | held 10,802.4 s; the client closed it |
  | `shell` | Web Shell `wss://ssh.bex.co/shell` (same entrypoint), its shell printing a line every 60 s | 16:35:42.530Z | 18:58:59.355Z | `control {"type":"error","message":"the session reached its time limit","reason":"timeout"}` and close 1000 after 8,597.2 s: the gateway's drain bound, an hour after `8b9d780f45` rolled the ssh-gateway (ReplicaSet `5696764dbd` at 17:58:26Z), not the edge |

  - At 18:16:13Z, 6,032 s in and past the 6,000 s mark (18:15:42Z), every connection answered its ping. The app logged each close as the client's close frame at 10,802 s, and the Traefik access logs recorded `/a` on `-fb2b8` and `/b` and `/pf` on `-2bvp4` as lasting 10,802 s.
- **Why.** Go's `http.Server` clears a connection's deadlines when Traefik's proxy hijacks it after the backend's 101 (`net/http` `conn.hijackLocked`), and the edge offers no HTTP/2 WebSockets (no `SETTINGS_ENABLE_CONNECT_PROTOCOL` in its SETTINGS on production). Locally, in the pinned image with a 15 s `readTimeout`, two pinged WebSockets held 75 and 90 s while a slow body was cut at 16 s with 502. The outside report the source cited (CryptoManufaktur-io/central-proxy-docker#35) saw closes at 118 s, which is not its `readTimeout`, and traefik/traefik#13820 is about a POST body, not an upgraded connection.
- **A slow plain request body** is handled as ADR022 says: 1.33 MB at 10 KB/s to the fixture answered 200 with `got=1333337 of 1333337 in 129.9s`, and the fast control `in 1.1s`.
- **So** t002–t004 closed as already met, as t001's acceptance says. Shipped instead: [ADR022 § A WebSocket carries no read deadline](../../../../docs/ADR022-tenant-isolation.md#a-websocket-carries-no-read-deadline-w6m639-2026-11-25), the WebSockets row in [ADR018's Services & lifecycle](../../../../docs/ADR018/services-lifecycle.md), and `scripts/test_edge_websocket.py`, run by the gitops workflow against the pinned Traefik image.
- **Definition of done.** Item 1 holds (t001's 3 h on production). Item 2 holds (the upload above). Item 3 assumed a new edge setting. There is none, and `readTimeout: 6000s` stays required because it still bounds a request body, so the guard for the WebSocket is the new test instead. Item 4 holds.
