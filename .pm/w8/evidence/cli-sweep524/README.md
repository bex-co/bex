# CLI sweep 524 — create-time `--env-var` reaches the process

**Verdict: no bug filed.** Same isolated session, `bex-canary`, HEAD build `218dffcd1`, 2026-10-19T09:28–09:29Z. Two free echo-server fixtures created, verified, deleted (former URLs `404`).

| step | observed result |
| --- | --- |
| `create --env-var QA524_MARKER=marker524abc --env-var EMPTY524=` | Exit `0` → `live`; the echo server's own `environment` report shows `QA524_MARKER=marker524abc` **and** `EMPTY524` present with an empty value — create-time env reaches the process, and an explicit empty value is carried, not dropped. |
| Duplicate key (`--env-var DUP524=first --env-var DUP524=second`) | Exit `0`; process env reads `second`, and `GET /env-vars` stores exactly one `DUP524` row (`second`) — deterministic last-wins collapse, no duplicate rows, no divergent store-vs-process state. |
| `--env-var NOEQUALS`, `--env-var =nokey` | Exit `1` client-side (`expected KEY=VALUE`), no service created. |
| `--env-var 1BAD=x` | Exit `1`, server `400 (ENVIRONMENT_VARIABLE_INVALID)` naming the naming rule; no service created. |

## Safety and unrun checks

- Cleanup: exact-owned `srv-dbau53qeg9sc73e8oei0` and `srv-dbau5efb7irc73chfv2g` deleted by ID; both former URLs 404. The marker values are harmless non-secrets by design.
- The duplicate-key dialect is recorded as observed behavior (last-wins); no Render oracle found in the pinned client or schema that demands a refusal, and the stored state is unambiguous, so it is not filed.
- Unrun: env values with `=`/newlines/unicode inside the value, >253-char keys (the cap is named in the 1BAD message), secret-file create flags (covered by sweep 456).
- Deploy lag means deployed revisions can precede local `main`.
