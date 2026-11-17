# w32 · m284 — Render SDK v1.8.0 sandbox compatibility: the unmodified SDKs drive bex sandboxes, each method graded

**Worker:** worker32 **Goal:** a user's unmodified Render Python or TypeScript SDK script, pointed at bex with `base_url`, creates, runs commands in, copies files to and from, and terminates a bex sandbox, and every SDK sandbox method has a recorded grade **Status:** done — production walk 2026-11-17 (both SDKs, every method graded)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Harness: unmodified Render SDK v1.8.0 (Python and TypeScript) pointed at bex-api — **DONE** | 1h | — |
| t002 | Walk every SDK sandbox method against bex and grade it — **DONE** | 1.5h | t001 |
| t003 | Fix the bex-api wire gaps the SDK walk found — **DONE** | 1.5h | t002 |
| t004 | Render parity — **DONE** | 15m | t003 |
| t005 | Simplify — **DONE** | 15m | t004 |
| t006 | Test coverage — **DONE** | 30m | t004 |
| t007 | Closeout — **DONE** | 10m | t005, t006 |

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

## Unblocked — 2026-11-16 (`/loopx w32`, `/pm unblock`)

- **Affected:** w32/m284 t002–t007.
- **Source:** verified evidence, no new user decision. The `/loopx w32` orchestrator ran `bash scripts/qa-login.sh <scratch path>` at 2026-11-16 18:50 PST (2026-11-17 02:50 UTC): it exited 0 with `ok <path>`, so a Kratos session was established, and it was logged out right after. The same day, `/qa-find-bugs-cli` sweeps 158 and 182 (w32/035, w32/036) signed in on production through the QA human device login with released `bex` v0.7.0.
- **Gate:** the 2026-11-08 release condition ("`bash scripts/qa-login.sh` prints `ok …` again") holds. No gate remains.
- **Scope:** unchanged. The definition of done still names the pinned v1.8.0 SDKs. Render released SDK v1.9.0 on 2026-11-16 (breaking: typed error classes); w22/m445 (todo) owns its contract and may retarget this walk once it lands.
- **Next:** t002 runs the walk recipe above on production. This host has no Playwright browser (`npx` is missing), so the device approval goes over curl with the QA session jar, and `qa-login.sh --login-challenge` answers the login challenge. The session and the device token are revoked when the walk ends.

## Closed — 2026-11-17 (`/loopx w32`)

- **Live walk (t002):** `scripts/render-sdk-compat.sh` ran against `https://api.bex.co` from 02:55 to 03:23 UTC, with main's deploy pin at `c48bc47fc59d`.
  - It ran as the QA user in `bex-canary`, with a human OAuth token from a headless device login of released `bex` v0.7.0 (qa-find-bugs § Approving a CLI device login without a browser). The device session and both Kratos sessions were revoked afterwards: the token answers 401.
  - The last run passed all 64 steps, 35 Python and 29 TypeScript. They cover create with every input, wait, get, list filters and paging, exec (exit code, env, `$HOME`, a 50 s silent command), file round trips (absolute, home-relative, a directory with a symlink), groups, an image import and a sandbox started from it, the deprecated builds, execution history, the async client, terminate, and the refusals.
  - With `SDK_COMPAT_SUSPEND=1`, an idle-suspended sandbox resumed on a download in 5 s.
  - Nothing was left in the workspace: no live sandbox, and only the `Default` group.
- **Not bex's (graded, owned):**
  - Snapshots are refused by name until w32/m285. The Python SDK reports `snapshots.create`'s 404 as `SandboxNotFoundError`.
  - Group credentials are refused by name (Render early access).
  - TypeScript v1.8.0's `SandboxEndedError` wraps only a 404, but Render's pinned contract answers an ended sandbox with `409 sandbox_not_running`. v1.9.0 is w22/m445's.
- **Flake, recorded:** one Python run failed two downloads on a 5 s TLS handshake timeout from this client. Every other run of the same steps passed, and none of 100 handshakes to `api.bex.co` afterwards took over 1 s.
- **Follow-up for w22/m445:** its t006 still plans to `/pm unblock w32/m284` and retarget the walk to v1.9.0. m284 is now done, so that step becomes a v1.9.0 run of `scripts/render-sdk-compat.sh` (the harness pins its versions at the top).
