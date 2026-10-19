# CLI sweep 556 — `jobs` one-off-job gate and selector dialects (live re-probe)

**Verdict: no bug filed.** Same isolated QA session (`puncsky@gmail.com`), `bex-canary` (`tea-daif693dqjvc73e7as3g`), HEAD-built CLI at `d98d07f62`, production pin still `f4e38371055f`, 2026-10-19T12:07–12:09Z. All probes were read-only or refused before any write; baseline stayed 8 services with zero `qa556` leftovers. No fixture was created, so nothing needed cleanup.

This closes the "Not yet re-probed live" note on the `jobs create` row of `docs/cli-compatibility-checklist.md`: creation is a deliberate non-goal (`.pm/DO_NOT_DO.md`, w4/m116/t003) refused with a named `410` before any record is written.

Against baseline `hello-go` (`srv-daif6dsmg29s73d1umvg`):

- `jobs list` → `[]`, exit 0 (history verb serves the empty set honestly).
- `jobs create --start-command 'echo qa556'` → exit 1, `410 (ONE_OFF_JOBS_UNSUPPORTED): one-off jobs are not supported on bex; run the command in the service itself (bex ssh), as a pre-deploy command, or as a cron job — existing jobs remain listable`. A following `jobs list` was still `[]`: the gate fires before any record, exactly as documented.
- `jobs create --plan-id plan-bogus` → the same 410 (gate precedes plan validation).
- `jobs create` with no `--start-command` → `400`; the raw REST body for `{}`, `{"startCommand":""}`, and an extra-field body is the uniform OpenAPI-boundary message `request body does not match the Render API schema` (`id: bad_request`). Verified this is the same body `POST /key-value` returns for a schema violation (w7/m49 boundary contract) — uniform by design, not a jobs-specific naming gap.
- Selectors: `jobs list`/`create` on a synthetic `srv-000…` and on the baseline Postgres id (`dpg-db9r654flioc739mcpbg`) → identical `404 (NOT_FOUND): not found` (lookup precedes the gate; cross-kind ids do not enumerate). `jobs cancel` on the synthetic service fails at the client's service pre-fetch with the same named 404.
- `jobs cancel` without `--confirm` prompts (`Are you sure you want to cancel job … for Service hello-go?`) and exits non-zero on EOF — the client confirmation guard, before any server call.
- `jobs cancel --confirm` jobId shapes follow the pinned Render pattern `^job-[0-9a-z]{20}$` at the boundary: wrong length or uppercase → `400 invalid path parameter "jobId"`; an exactly-20 lowercase nonexistent id (`job-daif6dsmg29s73d1umvg`) reaches the store → `404 (NOT_FOUND): job not found`.

## Unrun

- Cancel of a real pending/running pre-gate job (none exist in this workspace; creation is gated, so none can be minted for a fixture).
- GraphQL `createJob` / MCP `create_job` refusal parity (documented as identical; CLI sweep scope only).
- W8/129–131 live verification still waits for the production image pin to pass `5ae176831`.
