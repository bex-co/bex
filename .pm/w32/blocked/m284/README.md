# w32 · m284 — Render SDK v1.8.0 sandbox compatibility: the unmodified SDKs drive bex sandboxes, each method graded

**Worker:** worker32 **Goal:** a user's unmodified Render Python or TypeScript SDK script, pointed at bex with `base_url`, creates, runs commands in, copies files to and from, and terminates a bex sandbox, and every SDK sandbox method has a recorded grade **Status:** blocked — t001 done; t003 static fixes shipped and deployed (`2d453686f`, 2026-11-08 12:41 UTC); t002 live walk waits on a working QA sign-in

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Harness: unmodified Render SDK v1.8.0 (Python and TypeScript) pointed at bex-api — **DONE** | 1h | — |
| t002 | Walk every SDK sandbox method against bex and grade it | 1.5h | t001 |
| t003 | Fix the bex-api wire gaps the SDK walk found | 1.5h | t002 |
| t004 | Render parity | 15m | t003 |
| t005 | Simplify | 15m | t004 |
| t006 | Test coverage | 30m | t004 |
| t007 | Closeout | 10m | t005, t006 |

## Definition of done

- `scripts/render-sdk-compat.sh` runs the pinned unmodified Python and TypeScript SDK v1.8.0 against a bex API base with only environment configuration.
- On production, both SDKs complete create → exec → file upload/download → terminate.
- `docs/cli-compatibility-checklist.md` has an SDK section grading every sandbox method of both SDKs, with each non-working method attributed and either a non-goal reason or a filed follow-up.

## Source + Goal linkage

- **Source:** `/pm-brainstorm` for w32, 2026-11-07, item 4. The SDK accepts a `base_url` (`render-oss/sdk` `python/render/render.py:40`); SDK v1.8.0 released 2026-11-06. No board item or doc tracks SDK compatibility (ADR033 covers only the workflows protocol).
- **Goal linkage:** Pillar 5 and [ADR008 § Render parity first](../../../docs/ADR008-vision.md#render-parity-first): Render's sandbox docs present the SDK as the main programmatic path, so a Render-compatible bex must work with it, verified as a client the way DO_NOT_DO treats the CLI.
- **Expected outcome:** Users can move SDK-based sandbox code to bex by changing the base URL, and the board knows exactly which methods do not work yet.
- **Why now:** v1.8.0 is fresh, sandbox suspend (w2/m278) and network policy (w32/m283) are landing, and the SDK walk is the cheapest way to find remaining wire gaps before users do.
- **Render parity:** Included: t003 changes bex-api sandbox behavior on REST (and must keep GraphQL/MCP aligned). Dashboard N/A.

## Progress — 2026-11-08

- **t001:** `scripts/render-sdk-compat.sh` + `scripts/render-sdk-compat/scenario.{py,mjs}` run the pinned unmodified `render-sdk==1.8.0` and `@renderinc/sdk@1.8.0` against a bex origin. The static wire map (every SDK sandbox method → route → bex answer) is summarized in `docs/cli-compatibility-checklist.md` § Render SDK.
- **t003 (static part, ahead of the walk):** sandbox reads now carry Render's required `region`, `createdAt` and list `cursor` (the Python SDK raised on their absence even after a successful create); opt-in `cursor`/`limit` paging; the file gateway answers Render's JSON error body with `file_not_found`; the exec gateway sends a 15 s SSE keep-alive (the Python SDK times out at 45 s of silence); `name` is refused by name on create and list.
- **Follow-ups found:** relative remote paths (Render resolves them under the user's home; bex refuses) and a resume that outlasts the Python SDK's 5 s HTTP timeout — filed as inbox notes once the walk confirms them.

## Blocked — 2026-11-08 (`/loopx w32`)

- **Gate (user):** the live SDK walk (t002) needs a bex bearer token for the QA user on production. `bash scripts/qa-login.sh` failed twice on 2026-11-08 ~12:45 UTC with `error: login failed — no session established` (Kratos password flow submitted, but `GET /sessions/whoami` was not 200). It is not exit 2, so `QA_EMAIL`/`QA_PASSWORD` are present; the likely causes are a stale password or a second factor now required on the QA account. The credential files were not read.
- **Release condition:** `bash scripts/qa-login.sh` prints `ok …` again (fix the QA credentials in `.env`/`muse.env`, or remove the second factor from the QA account), **or** an already-authorized QA OAuth token is supplied privately as `BEX_ACCESS_TOKEN`. Then `/unblock w32` and run t002.
- **Walk recipe once unblocked:** isolated `BEX_CLI_CONFIG_PATH` + `BEX_HOST=https://api.bex.co/v1/` → `bex login` (device URL) → with the QA session jar, open the device URL and POST the dashboard's `/auth/device` confirmation, then `qa-login.sh --login-challenge` for the login challenge → read the token into `RENDER_API_KEY` without printing it → `BEX_API_URL=https://api.bex.co RENDER_WORKSPACE_ID=<QA workspace> bash scripts/render-sdk-compat.sh`. Record the PASS/FAIL lines here and regrade `docs/cli-compatibility-checklist.md` § Render SDK.
- **Already true without the walk:** the t003 fixes are covered by unit tests (`internal/sandbox/sdk_compat_test.go`, the `internal/sshgateway/sandboxsse` keep-alive test, the `internal/sshgateway/sandboxfiles` `file_not_found` test) and are deployed.
