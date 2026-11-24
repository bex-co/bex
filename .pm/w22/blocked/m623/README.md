# w22 · m623 — One user's concurrent sandbox execs and file transfers are not held to the SSH session cap, and a concurrency refusal is a 429

**Worker:** worker22 **Goal:** a user can run Render's 1,000 sandbox execs a minute: the ssh-gateway holds sandbox exec and file streams to a limiter of their own, sized so Render's per-minute classes bind first, native SSH, the web shell and agent attach keep their 5 per identity, and a refusal that remains answers 429 with `Retry-After`, never 400. **Status:** blocked (t001–t004 and t006–t008 done and shipped in `f27106b204`; t005 waits for production to run it, then t009)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | The gateway holds sandbox exec and file streams to a limiter of their own; native SSH, web shell and agent attach keep 5 per identity — **DONE** | 75m | — |
| t002 | bex-api answers the gateway's concurrency refusal with 429 and `Retry-After` on every exec and file surface — **DONE** | 50m | — |
| t003 | The gateway's production manifest sets the sandbox limiter, and the env inventory names it — **DONE** | 20m | t001 |
| t004 | ADR104 D7, ADR018 and ADR035 state the sandbox stream bound, or that there is none — **DONE** | 30m | t001, t002 |
| t005 | On production: w22/m443's gate names the fix commit, then the rate leg prints PASS | 40m | t003, t004 |
| t006 | Render parity — **DONE** | 25m | t002, t004 |
| t007 | Simplify — **DONE** | 20m | t006 |
| t008 | Test coverage — **DONE** | 30m | t006 |
| t009 | Closeout | 10m | t005, t007, t008 |

## Definition of done

- A gateway test holds more than 5 concurrent sandbox exec streams for one subject on one server, and more than 5 file transfers, while a native SSH session (and a web shell) for the same subject is still refused at its sixth.
- bex-api answers a gateway concurrency refusal with 429 and `Retry-After` on the exec redeem, the single-step `POST …/exec`, MCP's `sandbox_exec`, the file transfer redeem, and the resume-wait SSE path, and a test covers each. No path answers 400 `too many concurrent exec sessions`.
- `lego/operator/config/ssh/deployment.yaml` sets the new limiter's variables, and `lego/backend/AGENTS.md` lists them.
- ADR104 D7 § API rate limits and the ADR018 row in `docs/ADR018/render-operation-omissions.md` state the bound or that there is none. A bound that remains is recorded in ADR018 as a divergence awaiting the user's sign-off. ADR035 says which sessions the 5 per identity covers.
- The w22/m443 line in this workstream's README names the fix commit in its gate (``pin must carry `<sha>` ``).
- Once production runs that commit, `gh workflow run sandbox-measure.yml -f leg=rate` prints PASS: 600 execs inside a minute, none refused. The run is recorded here and in w22/m443.

## Source + Goal linkage

- **Source:** promoted from [w22/029](../../done/029.md) (found by the w22/m443 worker, 2026-11-23, running m443's production check), 2026-11-24; the premise was verified at `origin/main` `3829308659`.
  - [Run 45279460837](https://github.com/bex-co/bex/actions/runs/45279460837) of `sandbox-measure.yml`, leg `rate`, production pin `9fdf7e0a1171`: of 600 execs sent 20 at a time in one sandbox, none was rate limited, 394 streamed exit code 0, and 206 redeems answered **400** `too many concurrent exec sessions`. The 600 took 76 s.
  - The 400 comes from the gateway's session cap. `serveSandboxExec` refuses with 429 `session limit reached` when `Limits.Acquire(claims.Subject)` fails (`lego/backend/internal/sshgateway/sandboxsse/sandboxsse.go:126`). The file server does the same with `c.LimitKey(sandboxexec.SystemSubject)` (`sandboxfiles/sandboxfiles.go:118`). `mintAndDial` turns the 429 into `errExecSessionLimit` (`internal/sandbox/exec.go:361`), which wraps `core.ErrBadRequest` (`internal/sandbox/reachability.go:56`). The file redeem's reverse proxy passes the gateway's 429 JSON through unchanged (`internal/sandbox/file_connect.go`), without `Retry-After`.
  - One `SessionLimiter` per process (`cmd/ssh-gateway/main.go:218`) is shared by native SSH, the web shell, agent attach, sandbox exec and sandbox files. Production runs it at `BEX_SSH_MAX_SESSIONS` 100 and `BEX_SSH_MAX_SESSIONS_PER_IDENTITY` 5 on 2 replicas (`lego/operator/config/ssh/deployment.yaml`), so one user holds at most 10 concurrent sandbox streams, and fewer when the load balancer puts a sixth on one replica. Sandbox streams also spend the 100 global slots that native SSH needs.
  - At about 1.9 s a slot, that puts one user's ceiling near 300 execs a minute, below Render's 1,000.
- **Render:** [render.com/docs/sandboxes-limits](https://render.com/docs/sandboxes-limits) (read 2026-11-23) documents no limit on concurrent commands, streams or transfers. Its only exec limit is "Execute a command: 1,000" requests a minute per user, shared across the user's API keys, and an over-limit request fails with HTTP 429. Its only concurrency limit is sandboxes per group and per workspace. The SDK v1.10.0 contract bex froze (`lego/backend/internal/sandbox/testdata/sdk-v1.10.0-contract.json`) reads a 429 on a connect-token mint, exec stream or file transfer as Python's `RateLimitError`. A 400 is `ClientError`.
- **Goal linkage:** ADR008 § Render parity first; ADR104 D7 § API rate limits (w22/m443); the ADR018 sandbox rate-limit row.
- **Expected outcome:** an agent or script that runs sandbox commands in parallel meets only Render's per-minute limits and gets a 429 it can retry, never a 400. Native SSH keeps its abuse bound and no longer competes with sandbox streams for the 100 global slots.
- **Why now:** w22/m443 is parked on it. Its production run cannot pass while the cap holds a user below 1,000 execs a minute.
- **Render parity included:** the change reaches the REST and MCP exec surfaces, the REST file transfers and the pinned CLI and SDK, so t006 walks them. GraphQL serves execution records (`sandboxExecutions`) but runs no command, and the dashboard has no sandbox exec surface of its own; t006 confirms both.

## Progress

**Triage (2026-11-24, worker22, as the w22/029 worker):** promoted. The premise holds at `3829308659` (the paths above). The work spans the gateway, bex-api, the operator's gateway manifest, three ADRs and a production leg that waits for a deploy, which is more than an hour across several tasks.

**t001–t004 (2026-11-24, worker22):** sandbox exec and file streams hold `SandboxStreamLimits`, a caller pool of 100 per gateway replica with no per-identity bound below it and a platform pool of 32, 5 per key, apart from native SSH's 100 and 5. A held stream costs the gateway 1.32 to 1.39 MiB over SPDY (measured on a throwaway kind node, [t001](done/t001.md)), so the pod's memory limit went to 768Mi. bex-api answers the gateway's refusal as 429 `rate_limited` with `Retry-After: 1` on every exec and file surface. ADR104 D7 § Concurrent commands and transfers, ADR018's new row (a divergence awaiting sign-off) and ADR035 say so. Filed: w22/035 (the user's sign-off on the 100) and w22/036 (WebSocket exec streams, 0.28 to 0.30 MiB each).

**t006 Render parity (2026-11-24):** each surface's answer to the gateway's concurrency refusal, from `internal/sandbox/stream_limit_test.go`:

| Surface | Status | Code | Message | Wait |
| --- | --- | --- | --- | --- |
| Run-token redeem (`render sandboxes exec`, both SDKs) | 429 | `rate_limited` | too many sandbox commands and file transfers are running at once; retry in 1 second | `Retry-After: 1` |
| Single-step `POST /v1/sandboxes/{id}/exec` | 429 | `rate_limited` | the same | `Retry-After: 1` |
| File redeem, upload and download (`sandboxes files`, the SDKs' copy) | 429 | `rate_limited` | the same | `Retry-After: 1` |
| SSE `error` event after a resume wait | 429 (`status`) | `rate_limited` | the same | in the message |
| MCP `sandbox_exec` | tool error | `rate_limited:` prefix | the same | in the message |

The code is kept on these routes: none of the three redeem paths nor `POST …/exec` is an operation of Render's pinned OpenAPI, whose `errorCode` vocabulary would drop it. The pinned CLI (render-oss/cli at the v2.38.0 pin, `pkg/sandbox/repo.go` `errFromStreamResponse` and `pkg/sandbox/sse.go`) prints `received response code 429: <message>` for a redeem or transfer and `sandbox exec stream error status 429: <message>` for the event, so the wait reaches the user; before it printed `received response code 400: too many concurrent exec sessions`. The SDK v1.10.0 contract (`internal/sandbox/testdata/sdk-v1.10.0-contract.json`, `generic`) raises a 429 on a connect-token mint, exec stream or file transfer as Python's `RateLimitError`, where the 400 was `ClientError`; TypeScript's is a `ClientError` with status 429, as for Render's own 429. GraphQL runs no sandbox command (its sandbox exec fields are `sandboxExecutions` and `sandboxExecution`, reads), and `dashboard/src` calls no exec, run-token or file-token route, so neither changes. Render documents no concurrency limit, so the remaining bound is the ADR018 divergence (w22/035); no other drift.

**t008 Test coverage (2026-11-24):** each mutation, and the test that caught it:

| Mutation | Caught by |
| --- | --- |
| A sandbox server handed the SSH session limiter | does not compile: `Limits` is a `*SandboxStreamLimits` |
| `Admit` ignores `platform` (bex-api's streams in the caller pool) | `TestSandboxStreamPools`, `TestOneCallerHoldsMoreThanFiveExecStreams` |
| The per-identity default back to 5 | `TestSandboxStreamPools`, `TestOneCallerHoldsMoreThanFiveExecStreams` |
| A stream's end not idempotent (an unbalanced release) | `TestSandboxStreamPools` ("an end called twice freed two platform slots") |
| The platform pool's refusals counted as `sandbox_*` | `TestSandboxStreamPools`, `TestOneCallerHoldsMoreThanFiveExecStreams` |
| `errStreamLimit` back to a 400 `ErrBadRequest` | all five tests of `stream_limit_test.go` |
| The file proxy's 429 rewrite dropped | `TestFileStreamLimitIsA429OnREST` |
| `Transient` no longer reads the refusal | `TestStreamLimitIsTransientForASessionRead` |

`TestSandboxStreamPools` also holds that a release on one pool never frees a slot on the other, in both directions, and `TestSandboxStreamsDoNotSpendSSHSessionSlots` that sandbox streams leave the session limiter's 100 slots whole.

**Parked (2026-11-24, worker22):** t005 step 1. The fix is `f27106b204` on `origin/main`; w22/m443's gate and this milestone's both read ``pin must carry `f27106b204` ``. When production's pin carries it, t005 runs `gh workflow run sandbox-measure.yml -f leg=rate` and records the run, the pin, the exec count and seconds, the refusals by status and `bex_ssh_gateway_limit_rejections_total` by scope here and in w22/m443, then t009 closes the milestone. Nothing is needed from the user.
