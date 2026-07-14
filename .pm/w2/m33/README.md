# w2 · m33 — Deploy Hook: secret-URL deploy trigger

**Worker:** worker2 **Goal:** let external CI (GitLab CI, CircleCI, Jenkins, cron, curl) trigger a deploy via a rotatable secret URL, without an API key **Status:** todo

## Tasks (in order)

| id   | title                                                                                              | est  | depends_on |
| ---- | ------------------------------------------------------------------------------------------------------ | ---- | ---------- |
| t001 | `App.Status`/store: rotatable opaque deploy-hook token (mint via `id`-style opaque token, not guessable) | 45m  | —          |
| t002 | Unauthenticated-but-token-gated `POST/GET /v1/deploy-hooks/{token}` endpoint — validates token, triggers a deploy on the matching App | 1.5h | t001       |
| t003 | Regenerate/rotate verb (old token immediately invalid) + REST/GraphQL/MCP exposure of the current hook URL | 1h   | t001       |
| t004 | Dashboard: reveal/copy/regenerate control in the service's Deploy settings                            | 1h   | t003       |
| t005 | Rate-limit the trigger endpoint independently of the caller-token bucket (it's unauthenticated by design) | 45m  | t002       |
| t006 | Render parity: verify field/endpoint shape consistent across REST/GraphQL/MCP + dashboard             | 30m  | t004, t005 |
| t007 | Simplify                                                                                               | 30m  | t006       |
| t008 | Test coverage                                                                                          | 1h   | t006       |
| t009 | Closeout                                                                                               | 15m  | t008       |

## Definition of done

A service has a stable deploy-hook URL visible in REST/GraphQL/MCP and the dashboard; `POST`ing it triggers a new deploy without any API key; regenerating invalidates the old URL immediately; the endpoint is rate-limited.

## Source + Goal linkage

- **Source:** `/pm-brainstorm more` 2026-07-13 (third pass) — `docs/ADR006-bex-api.md:124` (`initialDeployHook` field, "ignored"); Render's Deploy Hook feature, confirmed via `.pm/w5/done/m13/README.md:7,48`.
- **Goal linkage:** Render parity — the deploy-trigger surface is currently API-key-only or git-push-only; this is the third, CI-agnostic path Render ships.
- **Expected outcome:** non-GitHub CI systems can trigger deploys without minting/rotating an API key.
- **Why now:** self-contained, no CRD/operator changes — pure bex-api surface, reuses existing deploy-trigger internals. Render parity included — new field/endpoint must be consistent across REST/GraphQL/MCP + dashboard.
