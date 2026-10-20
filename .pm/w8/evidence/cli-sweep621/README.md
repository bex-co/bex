# CLI sweep 621 — `logs --type` dialects: strict server, one silent drop inside the pinned client

**Verdict: no bug filed.** Same isolated QA session, `bex-canary`, CLI at `c8717afdf`, 2026-10-19. Read-only log queries against the baseline service; no fixture, no cleanup.

- **Single values are a closed client enum:** `--type app|request|build` each return their stream (request logs are the Traefik JSON lines). `--type predeploy` and `--type bogus` fail client-side: `must be one of "app", "request", "build"`, exit 1. Note the server enum is one wider — REST accepts `predeploy` as a type; the pinned CLI simply cannot ask for it (upstream surface, ledger-consistent).
- **Combinations:** `--type app,build` reaches the server and gets its named refusal: `400 log type "build" must be requested on its own`. `--type APP` (uppercase) works — the client normalizes case.
- **The one silent drop is client-side:** `--type app,bogus` returned plain app logs, exit 0. The server is *not* the lenient party: the identical REST queries `type=bogusType` and `type=app,bogusType` both answer `400 unknown log type "…" (want app|request|build|predeploy)`. So the pinned CLI filters unrecognized list members before sending (its single-value enum check doesn't run per-member) — an upstream parsing quirk, invisible against a strict server, and not fixable from bex. Recorded so it is not mistaken for a server-side silent-ignore like w8/132.

## Unrun

- `--type predeploy` through a future pin that widens the client enum.
- `app,request` combined streams (build is the only solo-restricted type observed).
