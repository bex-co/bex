# w9 · m39 — CLI-surface evidence & dialect chores: 429 escapee + revoke dialect + checklist evidence

**Worker:** worker9 **Goal:** no non-deliberate bex-api error path emits a body without Render's `message` shape, and the CLI checklist's login / `workspace set` / `psql` rows carry live evidence instead of ◐ guesses. **Status:** **DONE** — all tasks complete; live probes (t003/t004) captured against a freshly-provisioned dev-9 with the unmodified official CLI v2.21.0 once the `bex` cluster recovered.

## Tasks (in order)

| id   | title                                                                                  | est | depends_on                   | status |
| ---- | -------------------------------------------------------------------------------------- | --- | ---------------------------- | ------ |
| t001 | Fix the logs-subscribe 429 to Render's error shape + regression-test it                 | 20m | —                            | — **DONE** |
| t002 | Unify the `/v1/oauth/revoke` failure dialect (absorbs `008`)                            | 30m | —                            | — **DONE** |
| t003 | Live `render login` bad-key probe — attach evidence to the checklist login row          | 30m | t001                         | — **DONE** |
| t004 | Verify the official CLI's interactive `workspace set` flow — flip the ◐ row or file gap | 30m | —                            | — **DONE** |
| t005 | Assess CLI `psql` out-of-box friction — record the answer on the checklist row          | 30m | —                            | — **DONE** |
| t006 | Render parity — sweep every surface the dialect fixes touch                             | 30m | t001, t002, t003, t004, t005 | — **DONE** |
| t007 | Simplify — `/simplify` over the milestone's diff                                        | 20m | t006                         | — **DONE** |
| t008 | Test coverage — meaningful tests for the shipped behavior                               | 30m | t006                         | — **DONE** |
| t009 | Closeout — verify DoD, sync status, move to `done/`                                     | 15m | t008                         | — **DONE** |

## Closeout (2026-07-15)

DoD met and verified live. Both code fixes shipped and the checklist rows carry captured live evidence from a freshly-provisioned dev-9 (Kratos+Hydra+bex-api from HEAD) driven by the unmodified official CLI v2.21.0:

- **t001** — the `GET /v1/logs/subscribe` SSE-cap 429 now routes through `core.WriteErrStatus` (`{error,message,id}`, `id:"too_many_requests"`); white-box regression test `TestLogsSubscribeCapSpeaksRenderErrorDialect` (in `internal/logs`, where `sseConns` is settable — the accessible home for this coverage; asserts the Render shape + cap-release-on-reject).
- **t002** — `/v1/oauth/revoke`'s two `temporarily_unavailable` branches unified behind `core.ErrLogoutUnavailable`; the RFC 8628 device endpoints' OAuth bodies annotated as deliberate survivors (`writeOAuthError`). Absorbs note `008`.
- **t003** — bad `RENDER_API_KEY` → `render whoami` surfaces `Error: failed to get current user: unauthorized` (the m38-unified 401 `message` verbatim). Login row evidence attached. Absorbs note `009`.
- **t004** — `render workspace set <tea-…>` → set→persist→`workspace current` round-trip works live; the no-arg interactive picker is TTY-only client UI over the already-✅ `workspaces` list + local config write, **no wire gap** → `workspace set` row flipped to ✅. Absorbs note `006`.
- **t005** — `psql` friction assessment recorded: IP allow-list is Render parity, `BEX_DB_DOMAIN` is platform-install config, no bex-api work to promote. Absorbs note `007`.
- **t006 (parity)** — the two fixes are REST-transport-specific error paths (an HTTP connection-cap 429 and a REST-only logout endpoint) with no GraphQL/MCP counterparts; GraphQL/MCP carry their own error envelopes, so there is no cross-surface fan-out. The one remaining bare `{"error"}` in `logs/rest.go` is a post-upgrade **WebSocket** frame (not an HTTP body) — outside the HTTP-dialect scope, left intact so the CLI's WS frame parser is undisturbed.
- **t007 (simplify)** — the diff is two minimal, mirror-of-existing changes (a sentinel + a `WriteErrStatus` reroute); manual review found no behavior-preserving reduction.
- **t008 (test coverage)** — the two regression tests above plus the strengthened `TestLogoutFailsClosed/admin unavailable` assertion (revoke 503 is Render-shaped). `go test ./...` + `make lint-backend` green.

Absorbed notes `006`–`009` sit in `w9/done/`. DoD grep: the only bare-`{"error"}` HTTP emitters left are the two documented survivors (device endpoints, `:8091` internal store API).

## Definition of done

`grep`-verifiable: no non-2xx JSON error body in `lego/backend` (outside the annotated deliberate survivors — the RFC 8628 device endpoints and the :8091 internal store API) lacks Render's `message` field, and `internal/api`'s error-dialect regression test covers the logs-subscribe 429. `docs/cli-compatibility-checklist.md`'s login and `workspace set` rows carry captured live evidence (✅ or a filed bex-api wire gap — never CLI work, per DO_NOT_DO); the `psql` row records the friction assessment. Absorbed notes `w9/006`–`009` sit in `w9/done/`.

## Source + Goal linkage

- **Source:** `/pm-brainstorm` round 16, 2026-07-15 — absorbs open notes `w9/006` (workspace set), `w9/007` (psql friction), `w9/008` (revoke dialect), `w9/009` (login probe) plus the round-16 shipped-diff mine's one genuine m38 escapee: `lego/backend/internal/logs/rest.go:134` still emits bare `{"error":…}` on the SSE-connection-limit 429 (no `message`, no `id`). Grouped on the w8/m16 note-absorption precedent.
- **Goal linkage:** the fifth surface — Render's official CLI run unmodified against bex-api (`docs/cli-compatibility-checklist.md`, DO_NOT_DO: the CLI is a client to verify, never a product to build) — plus the one-error-dialect contract `w9/m38` shipped (`docs/ADR006-bex-api.md`).
- **Expected outcome:** a Render client parsing any bex-api failure — including the `BEX_MAX_SSE_CONNS` 429 and logout failures — sees Render's shape; two more checklist rows are evidence-backed instead of assumed.
- **Why now:** all five items are day-old residue stranded when m38 closed mid-round-15; the mine-while-fresh well decays as the code ages. Render parity task included: t001/t002 change the REST error surface.
