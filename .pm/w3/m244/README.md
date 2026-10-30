# w3 · m244 — Dashboard requests survive a dropped browser connection to api.bex.co

**Worker:** worker3 **Goal:** find why the browser occasionally sees `ERR_CONNECTION_CLOSED` on `api.bex.co/graphql` with no error at the edge or the origin, fix it at the edge if it is an edge behavior, and make sure a mutation lost that way is never shown to the user as "did nothing" **Status:** in progress — t001–t007 done; t008 closeout waits on the 25 s poll reaching production, then a 2 h net-log re-run and three clean w4 QA loop runs

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Capture a failing request at the socket level (Chrome net-log during a long dashboard session) — **DONE** | 1h | — |
| t002 | HTTP/2 idle, GOAWAY and connection-churn probe against the edge; compare Traefik entrypoint timeouts and LB behavior — **DONE** | 45m | — |
| t003 | Fix the edge behavior t001/t002 identify (or record why it is not edge-side) — **DONE** | 1h | t001, t002 |
| t004 | A mutation that fails on the network shows an error the user can act on — **DONE** | 45m | — |
| t005 | Render parity — **DONE** | 20m | t004 |
| t006 | Simplify — **DONE** | 15m | t003, t004 |
| t007 | Test coverage — **DONE** | 30m | t003, t004 |
| t008 | Closeout | 10m | t005, t006, t007 |

## Definition of done

- The socket-level cause of at least one captured `ERR_CONNECTION_CLOSED` on `api.bex.co` is recorded (net-log excerpt + matching edge/origin evidence), or ≥2h of captured dashboard traffic records none and the residual is documented.
- If edge-side: the fix ships (Traefik entrypoint/transport config or LB setting) with a probe that reproduces the old failure and passes after; three consecutive w4 QA loop runs record no `ERR_CONNECTION_CLOSED`/CORS console errors on `api.bex.co/graphql`.
- A mutation whose request fails with a network error surfaces a visible, retryable error in the dashboard (never silent), covered by a component/link test; whether automatic retry is safe (idempotency keys) is recorded as a decision, not implemented by assumption.

## Evidence (2026-10-29, from `w3/044`)

- **Window:** 2026-10-26T21Z – 10-29T23Z (Prometheus 3-day retention), the `IdleTimeout` fix live throughout.
- **Edge vs origin (Prometheus):** `traefik_service_requests_total{service="bex-system-bex-api-80@kubernetes"}` ≈129k requests: 8 × `502`, 4 × `503`, 8 × `code=0`, 25 × `499`; origin `bex_api_http_requests_total{surface="graphql",status=~"5.."}` = 0.
- **Edge access logs (Loki, `{namespace="bex-system"}` Traefik JSON lines):** **zero** `/graphql` requests ended 502/504/0. All 11 × 502 are long-running requests cut by bex-api rollouts — `POST /v1/webhooks/git` (30 s–3 639 s; see `w3/046`), `POST /v1/blueprints/deploy` (34 s), `GET /v1/services/…/deploys` (49 s). All `code=0` are `GET /v1/logs/subscribe` streams the client closed.
- **Sightings after the fix with no edge record:** `w4/done/m159/done/t003.md` 2026-10-29 06:46–06:48Z (no edge error, no bex-api rollout, no Traefik rollout, no node change in that window); `w4/done/433.md` ~08:02Z (coincides with a bex-api rollout 07:56–08:06 and node churn, but no `/graphql` error at the edge); `w4/done/m164/stream-request-start-a664.md` (3 polls, 2026-10-16, before the fix).
- **Idle-reuse probe (2026-10-29):** HTTP/1.1 keep-alive connections to `api.bex.co` reused after 5/30/61/91/121/151/181/241/301/361 s idle all answered on the same socket — no idle cutoff ≤ 6 min on that path, so the edge keep-alive race the FUTURE-MAYBE entry predicted does not reproduce for HTTP/1.1.
- **Conclusion:** the origin side is clean; the remaining `ERR_CONNECTION_CLOSED` happens between the browser and the edge (Hetzner LB TCP + proxy protocol → Traefik `websecure`), never reaching Traefik's access log as an error. Cause still unidentified (HTTP/2 GOAWAY/idle handling, LB flow handling, Traefik connection churn are the candidates).

## Source + Goal linkage

- **Source:** promoted from `w3/044` (2026-10-29, step 4: the correlation found neither an edge 502 nor an origin gap for the sightings) ← [FUTURE-MAYBE](../../FUTURE-MAYBE.md) "Intermittent `ERR_CONNECTION_CLOSED` on api.bex.co mutations" (first seen `w1/071`).
- **Goal linkage:** V0 roadmap 2 (operability) and the dashboard's reliability as the primary Render-parity surface.
- **Expected outcome:** QA and users stop seeing dropped GraphQL requests; any that still drop are visible and retryable, never a silent "did nothing".
- **Why now:** sightings continued on 2026-10-29 after the origin-side fix; a silent dropped mutation reads as a broken product.
- **Render parity included:** t004 changes dashboard error UX — compare with how Render's dashboard reports a failed action.
