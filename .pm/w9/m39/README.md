# w9 · m39 — CLI-surface evidence & dialect chores: 429 escapee + revoke dialect + checklist evidence

**Worker:** worker9 **Goal:** no non-deliberate bex-api error path emits a body without Render's `message` shape, and the CLI checklist's login / `workspace set` / `psql` rows carry live evidence instead of ◐ guesses. **Status:** todo

## Tasks (in order)

| id   | title                                                                                  | est | depends_on                   |
| ---- | -------------------------------------------------------------------------------------- | --- | ---------------------------- |
| t001 | Fix the logs-subscribe 429 to Render's error shape + regression-test it                 | 20m | —                            |
| t002 | Unify the `/v1/oauth/revoke` failure dialect (absorbs `008`)                            | 30m | —                            |
| t003 | Live `render login` bad-key probe — attach evidence to the checklist login row          | 30m | t001                         |
| t004 | Verify the official CLI's interactive `workspace set` flow — flip the ◐ row or file gap | 30m | —                            |
| t005 | Assess CLI `psql` out-of-box friction — record the answer on the checklist row          | 30m | —                            |
| t006 | Render parity — sweep every surface the dialect fixes touch                             | 30m | t001, t002, t003, t004, t005 |
| t007 | Simplify — `/simplify` over the milestone's diff                                        | 20m | t006                         |
| t008 | Test coverage — meaningful tests for the shipped behavior                               | 30m | t006                         |
| t009 | Closeout — verify DoD, sync status, move to `done/`                                     | 15m | t008                         |

## Definition of done

`grep`-verifiable: no non-2xx JSON error body in `lego/backend` (outside the annotated deliberate survivors — the RFC 8628 device endpoints and the :8091 internal store API) lacks Render's `message` field, and `internal/api`'s error-dialect regression test covers the logs-subscribe 429. `docs/cli-compatibility-checklist.md`'s login and `workspace set` rows carry captured live evidence (✅ or a filed bex-api wire gap — never CLI work, per DO_NOT_DO); the `psql` row records the friction assessment. Absorbed notes `w9/006`–`009` sit in `w9/done/`.

## Source + Goal linkage

- **Source:** `/pm-brainstorm` round 16, 2026-07-15 — absorbs open notes `w9/006` (workspace set), `w9/007` (psql friction), `w9/008` (revoke dialect), `w9/009` (login probe) plus the round-16 shipped-diff mine's one genuine m38 escapee: `lego/backend/internal/logs/rest.go:134` still emits bare `{"error":…}` on the SSE-connection-limit 429 (no `message`, no `id`). Grouped on the w8/m16 note-absorption precedent.
- **Goal linkage:** the fifth surface — Render's official CLI run unmodified against bex-api (`docs/cli-compatibility-checklist.md`, DO_NOT_DO: the CLI is a client to verify, never a product to build) — plus the one-error-dialect contract `w9/m38` shipped (`docs/ADR006-bex-api.md`).
- **Expected outcome:** a Render client parsing any bex-api failure — including the `BEX_MAX_SSE_CONNS` 429 and logout failures — sees Render's shape; two more checklist rows are evidence-backed instead of assumed.
- **Why now:** all five items are day-old residue stranded when m38 closed mid-round-15; the mine-while-fresh well decays as the code ages. Render parity task included: t001/t002 change the REST error surface.
