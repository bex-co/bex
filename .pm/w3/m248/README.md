# w3 · m248 — Git webhooks are acknowledged within GitHub's 10 s and redeploy out of band

**Worker:** worker3 **Goal:** every push webhook gets a `2xx` within GitHub's 10 s budget while the redeploys it triggers still happen exactly once; no delivery hangs for minutes or ends `502` on a rollout **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Time every step of the webhook path and find what waits up to an hour | 1h | — |
| t002 | Acknowledge after claiming and persisting the delivery; redeploy from a worker | 2h | t001 |
| t003 | Production check: delivery durations and outcomes after the rollout | 20m | t002 |
| t004 | Simplify | 15m | t002 |
| t005 | Test coverage | 30m | t002 |
| t006 | Closeout | 10m | t003, t004, t005 |

## Definition of done

- The step that held deliveries for up to an hour is identified and fixed (or made unreachable), with evidence.
- `POST /v1/webhooks/git` returns `2xx` within 10 s for every accepted delivery; the redeploy work runs out of band with the existing replay protection (a redelivery never redeploys twice; a failed persist returns `5xx` so GitHub redelivers).
- After rollout, 24 h of edge logs show no `/v1/webhooks/git` request over 10 s and no `502` on rollouts; a push still deploys (recorded).

## Evidence (from `w3/046`)


Why: GitHub gives up on a webhook delivery after 10 s, yet the edge logs show `POST /v1/webhooks/git` requests running 30 s – 3 639 s and ending `502` when a bex-api rollout drains the pod (Loki `{namespace="bex-system"}` Traefik access lines, 2026-10-27 – 10-29, found by `w3/044`). The handler (`lego/backend/internal/apps/webhook.go:285` `ServeHTTP` → `redeployMatching`/`redeployCandidates`) does the redeploy work synchronously inside the request, so a slow or blocked step holds the connection, GitHub records the delivery as failed, and whatever was still running is cut at the next rollout — a push-to-deploy can silently not happen.

- Find what blocks for up to an hour (per-candidate Kubernetes writes, the delivery-claim DB row, blueprint sync intents) using bex-api logs at those timestamps.
- Acknowledge fast (claim the delivery, enqueue, `202`) and do the redeploy work out of band, the way blueprint auto-sync already enqueues intents; keep replay protection.
- Verify: a webhook delivery returns in < 10 s in production (GitHub delivery log) and a push still deploys; a regression test proves the handler returns before the redeploy work finishes.

## Findings (2026-10-30, loopx w3)

- **Duration distribution** (Loki Traefik lines, 10-28 – 10-29): most deliveries finish in < 5 s, but many take 5–19 s (over GitHub's 10 s budget, so GitHub marks them failed and may redeliver), and some end `502` at 30.3 s, 34.8 s, 60.0 s and five at **3 601–3 639 s** (10-28 starts 09:08, 09:21, 09:37; 10-29 starts 16:23, 16:24) — the ~1 h ones are not aligned with rollouts, so something inside the handler waits about an hour.
- **Why 502 rather than a slow 200:** bex-api's `http.Server.WriteTimeout` is 30 s (`internal/api/graphql_cost.go:30` `HTTPWriteTimeout`). A handler still running at 30 s can no longer write its response, but it is not cancelled, so it keeps working while Traefik waits and eventually records 502. Any delivery whose work exceeds 30 s is therefore reported as failed even when the redeploy happened.
- **Synchronous work on the request path** (`webhook.go:285` `ServeHTTP`): a full `Apps` list per scope (`repoCandidates`), `prepareCheckedPush` → `awaitChecks` (optimistic-lock patch + GitHub `ResolveCommit`, 30 s client timeout per call), `enqueueBlueprintAutoSyncIntents`, then `redeployCandidates` (per candidate: billing gate, `dispatchCheckedCommit`/`redeployFetched`). bex-api logs nothing for webhooks, so the 1 h step is not yet identified.
- **Sizing:** acknowledging fast means a durable per-delivery queue (claim → persist intent → `202`, worker performs the redeploys with replay protection), reusing the blueprint auto-sync intent pattern — more than an hour across several tasks; promote to a milestone when this note's turn comes, with a first task that adds per-step timing logs to find the 1 h wait.


## Source + Goal linkage

- **Source:** promoted from `w3/046` (2026-10-30), found by `w3/044`'s edge-log correlation.
- **Goal linkage:** V0 roadmap 1 (deploy on push): push-to-deploy must not silently drop.
- **Expected outcome:** GitHub stops recording failed/timed-out deliveries; pushes deploy reliably across bex-api rollouts.
- **Why now:** deliveries routinely exceed GitHub's 10 s and some hang an hour; any push landing on a rollout can be lost.
- **Render parity omitted:** the inbound git webhook is a bex extension (ADR018 § bex ahead of Render); no REST/GraphQL/MCP/UI shape changes.
