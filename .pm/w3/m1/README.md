# w3 · m1 — Logs API: query + stream App logs

**Worker:** worker3 **Goal:** Render-compatible read-only logs over bex-api — query an App's application logs by time/text and stream new lines live. **Status:** done

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Logs `Core` verb + client-go pod-log backend (tail/limit/since, aggregate replicas) + RBAC | 45m | — |
| t002 | REST adapter `GET /v1/logs` (Render-compat): resource/type/search/time filters + envelope | 40m | t001 |
| t003 | Live tail: SSE/WebSocket `GET /v1/logs/subscribe` (follow) | 40m | t001 |
| t004 | Log-type split: label & filter `application` vs `request` (Traefik access logs) | 30m | t001 |
| t005 | GraphQL `logs(...)` resolver + `api_test.go` unit coverage | 30m | t001, t002 |

## Definition of done

REST + GraphQL return an App's application logs filtered by time and text; `GET /v1/logs/subscribe` streams new lines live; the `type` filter distinguishes `application` (at minimum) and labels `request`; `make test` green; endpoints documented in `docs/bex-api.md` + `docs/observability.md`. Verified with `curl -H "Authorization: Bearer $BEX_API_TOKEN" .../v1/logs?resource=<app>&type=application` against the mock cluster.

## Source + Goal linkage

- **Source:** `/pm-brainstorm` 2026-07-05 (Render `/logs` page) + inbox note `w3/001`.
- **Goal linkage:** `GOAL.md` #2 ("Basic obs for operation"); `docs/vision.md` AI-native pillar — MCP/agents need to read logs to debug deploys.
- **Expected outcome:** an operator or agent can pull and live-tail an App's logs through the Render-compatible bex-api without `kubectl`.
- **Why now:** highest-value obs primitive for running the platform (a failing deploy can't be debugged without logs), and the simplest backend (pod logs, no metrics-server dependency), so it ships before metrics.
