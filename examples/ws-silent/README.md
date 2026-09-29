# ws-silent — a WebSocket server that only listens

Acceptance fixture for [w1/m161](../../.pm/w1/blocked/m161/README.md): does client→server WebSocket traffic alone keep a free service awake?

An echo server cannot answer that. Every client frame produces a server frame, so the old behavior (only server→client traffic counted) and the fixed one look identical. This server keeps the directions apart:

| Mode | How | Traffic |
| --- | --- | --- |
| **silent** (the subject) | connect to `/ws` and send | client → server only; the server writes nothing, not even a control frame, unless the client pings |
| **server-sending control** | connect to `/ws?mode=send&every=30s` with `-every 0` (the client listens only) | the server sends `server tick` on its own schedule |
| **idle control** | deploy it and connect nothing | none |

`GET /stats` reports counters since the process started: `connections`, `clientMessages`, `serverMessages`, `clientPings`, `serverPongs`, `lastClientMessage`. Control frames are counted apart from application frames. A pong the server sends in reply to a client ping is server→client traffic, and it shows up in `serverPongs` instead of hiding inside a "silent" result. The bundled client never pings.

## Run it locally

```sh
cd examples/ws-silent
go test ./...                                   # each mode, asserted
PORT=3000 go run . &                            # the server
go run . client -url ws://127.0.0.1:3000/ws -every 1s -duration 3s
curl -s localhost:3000/stats                    # clientMessages 3, serverMessages 0, serverPongs 0
go run . client -url 'ws://127.0.0.1:3000/ws?mode=send&every=1s' -every 1s -duration 3s
```

A recorded local run (2026-09-29):

```text
--- silent: client sends 3 frames, 1s apart
sent client 1 (received so far 0)
sent client 2 (received so far 0)
sent client 3 (received so far 0)
{"clientMessages":3,"clientPings":0,"connections":1,...,"serverMessages":0,"serverPongs":0,...}
--- server-sending control (?mode=send&every=1s), client sends 3
sent client 1 (received so far 0)
sent client 2 (received so far 0)
received "server tick" (total 1)
received "server tick" (total 2)
sent client 3 (received so far 2)
received "server tick" (total 3)
{"clientMessages":6,"clientPings":0,"connections":2,...,"serverMessages":3,"serverPongs":0,...}
```

## Live check (m161 t003 owns it)

1. Create a **free** web service from this directory, e.g. `POST /v1/services` with `repo`, `branch: main`, `rootDir: examples/ws-silent`, `runtime: docker`, `plan: free`. Note its id and URL.
2. Silent subject: `go run . client -url wss://<service>.onbex.co/ws -every 60s -duration 40m`. Keep `every` below the idle window and `duration` well past it.
3. During the run, `curl https://<service>.onbex.co/stats`. `clientMessages` should climb while `serverMessages` and `serverPongs` stay 0. **A `/stats` request is itself HTTP activity**, so poll it only after the verdict, or from a second service's logs, not during the idle window you are measuring.
4. Controls: repeat with `?mode=send&every=60s` (it should stay awake for the server-sending reason), and leave a second instance with no client (it should hibernate on schedule).
5. **Scanner traffic.** Internet scanners hit `*.onbex.co` and reset idle clocks. Read the service's request log for the measured window, and discard or rerun any window with requests you did not send (as `w1/m151` and `w1/m157` had to).
6. **Cleanup.** `DELETE /v1/services/<id>` for every fixture, then confirm `GET` returns `404`.
