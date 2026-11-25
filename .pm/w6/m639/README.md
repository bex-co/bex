# w6 · m639 — A WebSocket through the edge stays open past 100 minutes, as Render enforces no maximum duration

**Worker:** worker6 **Goal:** a WebSocket that pings through `wss://<service>.onbex.co` stays open for hours, as on Render, instead of being cut at the edge's 6,000 s read deadline, while ADR022 still says what bounds a slow plain request. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [Measure a pinged WebSocket through the production edge for 2.5 hours](t001.md) | 1h | — |
| t002 | [Choose how an upgraded connection escapes the edge's 100-minute read deadline, and amend ADR022](t002.md) | 45m | t001 |
| t003 | [Change the edge and its gitops-validate guard](t003.md) | 45m | t002 |
| t004 | [Production check: a pinged WebSocket stays open 3 hours through the edge](t004.md) | 1h | t003 |
| t005 | [Render parity](t005.md) | 30m | t004 |
| t006 | [Simplify](t006.md) | 20m | t005 |
| t007 | [Test coverage](t007.md) | 45m | t005 |
| t008 | [Closeout](t008.md) | 10m | t006, t007 |

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
