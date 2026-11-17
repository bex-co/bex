# w32 · m446 — A signed-in request never fails with `503 tenant onboarding unavailable` because a workspace is busy

**Worker:** worker32 **Goal:** a returning person's authenticated request on any surface is answered by its own handler. A transient fault in the per-request onboarding check (`EnsureTenant`) no longer turns into `503 tenant onboarding unavailable`, and the fault that does reach a caller is logged with its cause. **Status:** todo (t002, t003 done)

## Tasks (in order)

| id   | title                                                                                                 | est | depends_on |
| ---- | ----------------------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Log the cause of every onboarding 503 and name the failing call on production                         | 30m | —          |
| t002 | An OpenFGA check that errors still serves an existing admin, now that grant writes are idempotent — **DONE** | 20m | t001 |
| t003 | A returning person whose workspace is already known is served through a transient onboarding fault — **DONE** | 45m | t001, t002 |
| t004 | Production: the CLI's create, deploy and `--wait` journey sees no onboarding 503                      | 30m | t003       |
| t005 | Render parity                                                                                         | 15m | t004       |
| t006 | Simplify                                                                                              | 15m | t005       |
| t007 | Test coverage                                                                                         | 30m | t005       |
| t008 | Closeout                                                                                              | 10m | t006, t007 |

## Definition of done

1. On production, three runs of the CLI journey below see no `503` on any call. Each run uses released `bex` with a human device-login token in a QA workspace: create a free image web service, then `deploys create --image … --wait`, polling `bex deploys list <id> -o json` once a second from the create until 60 s after the second deploy reads `live`. Before this milestone the same journey hit 4 such 503s in about 190 calls.
2. When `EnsureTenant` does fail a request, bex-api logs the underlying error with the request id, and the answer is `503` whose body names the cause class. It never reads only `tenant onboarding unavailable` with no log line.
3. `ensureGranted` never sends a `POST /stores/{id}/write` for an admin tuple that already exists. A failed or erroring `Check` is retried or returned, and an "already exists" write answer counts as granted.
4. A returning person whose personal workspace id was resolved within the last few minutes is still served when the store or OpenFGA has a transient fault on the cache-miss path. The account-deletion tombstone still wins, as ADR086 requires.

## Source + Goal linkage

- **Source:** `/qa-find-bugs-cli` loop sweep 134 (nonce 51c4), 2026-11-16 09:52–10:11 UTC, on production `api.bex.co` pin `f114fdd9f5a6`. The run used released `bex` v0.6.0 (pinned Render CLI v2.36.0) in workspace bex-canary `tea-daif693dqjvc73e7as3g`, with a human device-login token, non-TTY and `-o json`.
- **Evidence (sanitized):**
  - CLI, 09:58:25: `bex deploys list srv-dbtd74m004dc73ajmh70 -o json` exited 1 with stderr `Error: unexpected response: 503 Service Unavailable`. That was 47 s into that service's first deploy.
  - Two more `deploys list` calls exited 1, with stderr not captured. One was at 09:53:04, 31 s into `srv-dbtd4mm004dc73ajmh0g`'s first deploy. The other was at 09:56:12, 1 s after `srv-dbtd6f6004dc73ajmh50` was created.
  - REST, 10:02:36: `GET /v1/services/srv-dbtd74m004dc73ajmh70/deploys` with the same bearer answered `503` in 0.42 s, with `content-type: application/json`, `server: cloudflare` and `cf-cache-status: DYNAMIC`. The body was `{"error":"tenant onboarding unavailable","id":"unavailable","message":"tenant onboarding unavailable"}`. This was 1 of about 100 calls at 1/s made around an image deploy on that service (deploy `dep-dbtd8oh9hcnc73acin9g`, live at 10:01:50).
  - Controls with no create or deploy in flight: 70/70 `200` on the same endpoint at 09:59–10:00, 240/240 `200` on `GET /v1/owners?limit=1` at 10:06–10:11, and a 12-call CLI burst with no failure. That makes 4 failures in about 190 calls while services were being created or deployed, and 0 in about 320 quiet calls.
  - All three fixtures are deleted (REST `404`). Nothing pre-existing was touched.
- **Producer:**
  - `lego/backend/internal/api/auth.go:383-404`: every human-authenticated request, on REST, GraphQL and MCP alike (`internal/api/server.go:101`), calls `onboard.EnsureTenant`. Any uncoded error is answered with `WriteErrStatus(503, "tenant onboarding unavailable")`, and the error itself is dropped unlogged.
  - `internal/api/tenancy.go:159-193`: `EnsureTenant` reads the `account_deletions` tombstone on every request (`internal/store/accounts.go:69`) before its 30 s cache (`internal/core/http.go:595`). On a cache miss it reads `TenantForOwner` (`internal/store/workspaces.go:89`) and calls `ensureGranted`.
  - `internal/api/tenancy.go:383-397`: `ensureGranted` treats a `Check` that errors the same as one that answers false. It then writes the admin tuple, which OpenFGA refuses when the tuple already exists (the function's own doc comment, `tenancy.go:144-146`). So any transient check error on a cache miss becomes a 503 for an existing admin.
- **Consumer:**
  - The pinned Render CLI has no HTTP retry; each repo returns `fmt.Errorf("unexpected response: %v", resp.Status())`.
  - `pkg/tui/views/deploycreate.go:67` `WaitForDeploy` returns the first `GetDeploy` error. One 503 therefore makes `deploys create --wait` exit non-zero for a deploy that goes on to succeed.
- **Unverified:**
  - Which call failed: the tombstone `SELECT`, `TenantForOwner`, or an OpenFGA `Check` followed by the refused duplicate write. The 503 answers in 0.42 s, which argues against a timeout, and the cause is not logged anywhere.
  - **Likely candidate (w4/695, 2026-11-16):** w4 found OpenFGA `Check` posts failing at once with `read: connection reset by peer` on a reused idle keep-alive connection. One `bex-api` replica logged it 3 times in 80 minutes (22:20–22:37 UTC), and a `GET …/deploys` answered `503` within the same window. In `ensureGranted` that error falls through to the refused duplicate admin write, which fits the fast 503 here. The fix (`a77d36c92`) replays a `Check` that fails on a reused connection and lowers `IdleConnTimeout` to 30 s. It is not in production pin `938c5cb36b49` (23:07 UTC). t001 should search the replicas' logs for `connection reset by peer` before naming another cause. **Update, same evening:** w21/099 (`ef7e65cd3`) makes OpenFGA writes idempotent (`on_duplicate: "ignore"`), so a `Check` error now leads to a no-op grant write and the request is served. Together the two fixes remove this mechanism once they are deployed; neither is in pin `938c5cb36b49`. t002 shrinks to the test that pins the path. t001, t003 and t004 still stand, because the store-side reads were never ruled out. Both fixes reached production in pin `ef7e65cd3f46` (2026-11-16 23:59 UTC). Sweep 191's early recheck there saw 150 of 150 CLI `deploys list` calls succeed across a create, a first deploy and a `--wait` deploy (details in t004). t001's log search should start with that pin.
  - Whether creates and deploys in the same workspace cause the faults, for example through OpenFGA write load from new service tuples or through pool pressure, or only coincide with them.
  - The dashboard (Kratos session) and MCP paths were not probed, although they share the middleware.
- **Dedupe:**
  - No open or done item covers this production symptom.
  - w8/m38, m40 and m41 walkthroughs retried around the same message on local dev stacks with broken Postgres.
  - w3/m305 t005 saw it on dev-3 after a Docker restart.
  - `.pm/FUTURE-MAYBE.md` (046 F17) is about invite-accept projection, not this path.
- **Goal linkage:** [ADR008 § Render parity first](../../../docs/ADR008-vision.md#render-parity-first). Render's API does not fail a signed-in read because the workspace is busy, and the unmodified Render CLI and SDKs, which bex serves, do not retry.
- **Expected outcome:** CLI commands, `--wait` loops, SDK scripts and dashboard reads stop failing at random while the person is creating or deploying. A real onboarding fault leaves a log line that names its cause.
- **Why now:** creating or deploying is exactly when people poll, so the fault hits the busiest moments. A `--wait` that polls every 5 s across a 45 s deploy makes about 9 calls, so at roughly 2% per call it fails about one deploy in six.
- **Render parity:** included. The fix changes the error behavior of every authenticated REST, GraphQL and MCP request.
