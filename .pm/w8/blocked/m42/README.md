# w8 · m42 — Unstick production deploys: fix the flaky gates that have held `deploy.yml` red since 2026-09-22

**Worker:** worker8 **Goal:** `main` reaches production again on every push, and a stalled deploy is noticed in hours, not days. **Status:** blocked

## Tasks (in order)

| id   | title                                                                       | est | depends_on             |
| ---- | --------------------------------------------------------------------------- | --- | ---------------------- |
| t001 | Root-cause the dashboard vitest worker-start timeout and the 53-minute hang — **DONE** | 60m | —                      |
| t002 | Root-cause the `env-groups.test.tsx` 10 s test timeouts under CI load — **DONE**       | 40m | —                      |
| t003 | Root-cause the OpenSandbox `Pool scale` BeforeEach `Eventually` timeout — **DONE**     | 45m | —                      |
| t004 | Make ci-red-streak see deploy.yml — fetch runs per workflow, not a global window — **DONE** | 40m | —                      |
| t005 | Simplify — **DONE**                                                                    | 20m | t001, t002, t003, t004 |
| t006 | Test coverage — **DONE**                                                               | 30m | t005                   |
| t007 | Closeout                                                                    | 15m | t006                   |

## Definition of done

- Three consecutive `deploy.yml` runs on `main` complete with `conclusion: success` (build + deploy jobs ran, not skipped), and `gh run list --workflow deploy.yml` shows no gate failure caused by t001–t003's tests in the same window.
- Against production, the sandbox copy route that `9a5e77529` added answers instead of Go's default `404 page not found`: `bex ea sandboxes copy ./f.txt <sbx-id>:f.txt -o json` on a disposable sandbox exits 0 and a follow-up `bex ea sandboxes exec <sbx-id> -- cat f.txt` prints the file.
- Each flake has a named root cause written in its task and a fix that removes the cause. **No retries, sleeps, `test.retry`, raised `testTimeout`, or skips**. A raised timeout is acceptable only when the task proves the operation is slow by design rather than starved or waiting on a race.
- `scripts/ci-red-streak.sh` reports a deploy.yml failure streak even when the streak is interleaved with supersession cancellations and other workflows' runs (t004's fixture). Today's global 100-run window hid a 5-failure streak: the open issue #73 listed only `test (mobile)`.

## Evidence (2026-09-23, `/qa-find-bugs-cli` sweep 5)

Found from the CLI side: `bex ea sandboxes copy` returned `Error: received response code 404: 404 page not found` for both upload and download (`sbx-dapp31p4dm7c7390q9hg`, workspace `bex-canary`, since deleted). The route exists on `main` (`lego/backend/internal/sandbox/rest.go:165`, `POST /v1/sandboxes/{id}/files/{operation}/token`, commit `9a5e77529`, 2026-09-21 22:49 PDT). `fb91fe555` (15:51 the same day) is live: `--env-var` returns its named `SANDBOX_ENV_UNSUPPORTED`. So production is running an image from between those two commits.

`gh run list --workflow deploy.yml -L 60` on 2026-09-23 08:40Z: **1 success, 20 failure, 39 cancelled**. The last success was `8f07882f6` (2026-09-22T00:36Z). `ee9652171` (00:48Z) rolled out and then failed its final dashboard `rollout restart` step with `Unable to connect to the server: net/http: TLS handshake timeout` (run `35673466394`). Every run since has failed a test gate, so `build` and `deploy` were skipped. The many cancellations are supersession, expected with frequent pushes:

| run           | head        | failed gate                   | cause in log                                                                                                                                                                                           |
| ------------- | ----------- | ----------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `35692226114` | `9a5e77529` | dashboard                     | `env-groups.test.tsx`: `renames and typed-confirms deletion using the immutable group id` (10186 ms) and `moves scope and clones through server-side management verbs` (15522 ms), over the 10 s limit |
| `35696069432` | `60702a94a` | dashboard + opensandbox       | dashboard vitest went silent at 07:20:47 and was killed with `The operation was canceled` at 08:13:18                                                                                                  |
| `35709320846` | `96893f0ef` | opensandbox-controller        | `[FAIL] Pool scale When reconciling a resource [BeforeEach] should successfully scale out pool buffer size`, `Timed out after 10.393s`, `pool_controller_test.go:91` (35 passed / 1 failed)           |
| `35836680656` | `22069c1a2` | dashboard (439/439 files ok)  | `Vitest caught 1 unhandled error`: `[vitest-pool]: Failed to start forks worker for … use-loader-error-retry.test.tsx` caused by `[vitest-pool-runner]: Timeout waiting for worker to respond`      |

All of these run on the shared self-hosted pool `[self-hosted, Linux, ARM64, bex-ci]` (`.github/workflows/dashboard-test.yml:24`, `opensandbox-controller-test.yml:24`). The common thread is timing under load. `dashboard/vitest.config.ts` sets `testTimeout: 10000` and leaves pool size and worker count at the defaults. Runner concurrency and host load were **not measured** (the runners API needs admin).

## Status update (2026-09-26, sweep 10)

`deploy.yml` run `36217068753` on `dbf24a217` went green end to end, and the deploy step finished at 05:12Z. That was the first success since `8f07882f6` (2026-09-22T00:36Z), and production caught up. Live checks that were previously blocked now pass: on `sbx-darl9oi9slkc73beqt9g` (stopped), directory upload, single-file upload to an absolute path, file download, and directory download all round-tripped. That meets the sandbox-copy DoD bullet. **Not** resolved by this: none of t001–t003's flakes were fixed (the run passed on a fresh roll of the dice) (t004's detector fix landed separately). The "three consecutive successful runs" bullet is 1/3.

## Source + Goal linkage

- **Source:** live `/qa-find-bugs-cli` sweep 5, 2026-09-23 (w8). The CLI symptom (sandbox copy 404) was traced to deploy lag, and the deploy lag to red gates.
- **Goal linkage:** continuous delivery from `main` (ADR058: "our own cluster keeps deploying continuously from `main` by digest"). Every fix on the board since 2026-09-22 00:48Z is shipped-to-main but not shipped-to-users. That includes `9a5e77529` (sandbox copy, w7/m150), `882758fc8`, `5da881aff`, `cbb01b3d0`, `8db6098ab`, `d1fd06847`, `3757d0ff1`, and `31ccd430b`.
- **Expected outcome:** pushes to `main` deploy without a human re-running flaky gates, and the existing red-streak detector flags a stall like this one within a day.
- **Why now:** production has been frozen for more than 32 h. Every live verification that depends on a recent fix is blocked, including w9/m94 t004 and w7/m150's live copy check, and QA hunts keep reporting deploy lag as if it were a product bug. **The immediate unblock is a human decision, not part of this milestone:** re-run `deploy.yml` on current `main` (a production deploy, so an agent should not trigger it without explicit approval).
- **Render parity omitted:** CI/deploy infrastructure only. No REST, GraphQL, MCP, or UI surface changes.

## Status update (2026-09-27 ~07:45Z, `/qa-find-bugs-cli` loop)

The self-hosted `bex-ci` pool has run no job since about 05:21Z (the last real completion was `docs (prettier)`). Every self-hosted workflow is `queued`, about 38 runs: `go lint`, `govulncheck`, `test (backend|dashboard|operator)`, `scripts (test)`, `docs (prettier)`. `deploy.yml` run `36300177540` (`d55061dbb`, created 06:28Z) has sat `queued` with all six gate jobs waiting on `self-hosted,Linux,ARM64,bex-ci`. It holds the deploy concurrency group, so each later push's run goes `pending` and is then `cancelled` by the next push (33 of the last 40 runs).

Production is therefore still `726042a28` (last deploy 2026-09-26 05:53Z). On `main` but **not live**: `w8/023`–`028`, `w8/m43`–`m46`, and `w4/m136`–`m140`/`142`–`144`. This looks like runners offline or wedged on the host, not flaky gates. It needs someone with access to the runner host (DO_NOT_DO #RUNNER-HOSTS: the operator's Mac).

## Status update (2026-09-28 ~05:30Z, `/qa-find-bugs-cli` loop)

`deploy.yml` run `36373293497` (`016391810`: 8 fixes, including `87c93899d` "bound permanent rollout image pull failures") sat `pending`/`queued` for about 1.5 h behind an 18-run backlog. Its `test-opensandbox-controller` gate then ended `failure` after 16 min (04:57–05:13Z) with **no failed step and no retrievable log** (`BlobNotFound`), which looks like a runner-lost job rather than a test failure. The deploy will not build or ship until the gate is re-run. Production stays on `4a0422577`.

## Unblocking work (2026-10-02)

The login and stale deploy-history blockers are resolved. Runs `36542511672` (`f22a0b068`), `36544512390` (`d303bd582`) and `36549825739` (`de9ac4d1c`) each completed build and deploy successfully on 2026-09-29. Ten inspected noncancelled deploy runs had a successful OpenSandbox gate, including `36919000487` whose unrelated backend gate failed. The sandbox-copy DoD was already satisfied on 2026-09-26 (§ Status update); t007's old login note was stale.

The user accepted removing duplicate main gates. The four reusable suites now run directly on PRs and through `deploy.yml` on main. Their workflow/script-only paths are included in `deploy.yml`, preserving coverage when only CI inputs change. Those input-only main changes also use the production pipeline. A superseded main push can be cancelled before testing; current main is gated before building. This removes four duplicate jobs per qualifying main push without changing tests, timeouts, runner hosts or fleet configuration.

Local workflow validation passes. **Remaining:** ship the workflow changes and observe the updated CI route. The shared host still has 7 CI + 3 production runners with 4-CPU quotas on a 15-CPU Docker VM, so reducing fleet concurrency remains a follow-up if contention persists after deduplication. No claim is made that the timeout-only change removed that host-wide cause.


## Verification and remaining release gate (2026-10-02)

Full backend suite against isolated Postgres/OpenFGA/OpenBao, operator `make test`, full CLI suite, backend/CLI lint, targeted Go race tests, workflow guards and ci-red-streak fixtures passed. The final affected backend packages passed again after review changes. Overlay mutation checks confirmed the image, workspace and rename regressions fail with their fixes removed. Markdown was formatted; QA fixtures and the isolated credentials were cleaned up.

Implementation and review are complete locally. Repository `AGENTS.md` requires an explicit `$ship` before commit/push. After ship, observe CI/production and complete this milestone's remaining live closeout; m45 also needs the updated CLI released. The earlier policy/sign-off/login blockers are resolved.
