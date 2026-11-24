# w22 · m623 — One user's concurrent sandbox execs and file transfers are not held to the SSH session cap, and a concurrency refusal is a 429

**Worker:** worker22 **Goal:** a user can run Render's 1,000 sandbox execs a minute: the ssh-gateway holds sandbox exec and file streams to a limiter of their own, sized so Render's per-minute classes bind first, native SSH, the web shell and agent attach keep their 5 per identity, and a refusal that remains answers 429 with `Retry-After`, never 400. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | The gateway holds sandbox exec and file streams to a limiter of their own; native SSH, web shell and agent attach keep 5 per identity | 75m | — |
| t002 | bex-api answers the gateway's concurrency refusal with 429 and `Retry-After` on every exec and file surface | 50m | — |
| t003 | The gateway's production manifest sets the sandbox limiter, and the env inventory names it | 20m | t001 |
| t004 | ADR104 D7, ADR018 and ADR035 state the sandbox stream bound, or that there is none | 30m | t001, t002 |
| t005 | On production: w22/m443's gate names the fix commit, then the rate leg prints PASS | 40m | t003, t004 |
| t006 | Render parity | 25m | t002, t004 |
| t007 | Simplify | 20m | t006 |
| t008 | Test coverage | 30m | t006 |
| t009 | Closeout | 10m | t005, t007, t008 |

## Definition of done

- A gateway test holds more than 5 concurrent sandbox exec streams for one subject on one server, and more than 5 file transfers, while a native SSH session (and a web shell) for the same subject is still refused at its sixth.
- bex-api answers a gateway concurrency refusal with 429 and `Retry-After` on the exec redeem, the single-step `POST …/exec`, MCP's `sandbox_exec`, the file transfer redeem, and the resume-wait SSE path, and a test covers each. No path answers 400 `too many concurrent exec sessions`.
- `lego/operator/config/ssh/deployment.yaml` sets the new limiter's variables, and `lego/backend/AGENTS.md` lists them.
- ADR104 D7 § API rate limits and the ADR018 row in `docs/ADR018/render-operation-omissions.md` state the bound or that there is none. A bound that remains is recorded in ADR018 as a divergence awaiting the user's sign-off. ADR035 says which sessions the 5 per identity covers.
- The w22/m443 line in this workstream's README names the fix commit in its gate (``pin must carry `<sha>` ``).
- Once production runs that commit, `gh workflow run sandbox-measure.yml -f leg=rate` prints PASS: 600 execs inside a minute, none refused. The run is recorded here and in w22/m443.

## Source + Goal linkage

- **Source:** promoted from [w22/029](../done/029.md) (found by the w22/m443 worker, 2026-11-23, running m443's production check), 2026-11-24; the premise was verified at `origin/main` `3829308659`.
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
