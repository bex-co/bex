# w1 · m72 — Converge the ten `dev-N` harnesses onto one parameterized script

**Worker:** worker1 **Goal:** one dev-environment implementation instead of ten diverging copies, so a milestone verified on one worker's stack is verified on the same stack every other worker has. **Status:** todo

## Tasks (in order)

| id   | title                                                       | est | depends_on |
| ---- | ----------------------------------------------------------- | --- | ---------- |
| t001 | Reconcile the eight `up.sh` variants into one feature set     | 1h  | —          |
| t002 | Write `scripts/dev-env.sh` (up / down / status, N-parameterized) | 2h | t001    |
| t003 | Migrate all ten workstreams onto it                           | 1h  | t002       |
| t004 | Bound the log growth                                          | 30m | t002       |
| t005 | Simplify                                                      | 30m | t003, t004 |
| t006 | Test coverage                                                 | 45m | t003, t004 |
| t007 | Closeout                                                      | 15m | t005, t006 |

## Definition of done

- One implementation — `scripts/dev-env.sh <N> {up|down|status}` — replaces the ten copied harnesses. Each `.pm/wN/dev-N/` keeps only what is genuinely per-workstream: `ports.env`, `README.md`, `.gitignore`, and any deliberate local override.
- Every capability present in any of the eight `up.sh` variants is either in the shared script or explicitly recorded as dropped, with a reason. Nothing is lost silently.
- All ten workstreams come up, report healthy, and tear down through the shared script. At least two are verified end to end on the live local cluster, including one of the divergent variants (`w5`, whose `up.sh` is 274 lines against `w2`'s 175).
- Log growth is bounded, so no harness can reach the 3.9 GB currently sitting in `.pm/w5/dev-5/logs`.
- Each `wN/README.md`'s "Local dev environment" section points at the shared script.
- No worker's running environment is destroyed by the migration.

## Source + Goal linkage

- **Source:** principal-engineer architecture review, 2026-08-18 (session hand-off, no inbox note). Measured at HEAD: 133 tracked files across ten `.pm/wN/dev-N/` directories, ~708 LOC copied ten times (7,078 tracked LOC total).
- **Goal linkage:** developer infrastructure, not product. It advances every roadmap milestone indirectly by making per-workstream verification evidence comparable.
- **Expected outcome:** ~7,078 tracked LOC collapses to one script plus ten small config files, and a fix to the dev stack reaches all ten workers instead of one.
- **Why now:** the copies have already diverged, and the divergence is now affecting what can be tested where. `up.sh` has **eight distinct variants across ten copies** (only `w2`/`w7`/`w10` still agree); `status.sh` has five. Concretely: `w1/dev-1/up.sh` wires `BEX_SMTP_ADDR` + `BEX_REQUIRE_VERIFIED_INVITE_EMAIL` and forwards a Mailpit SMTP port, and `w2/dev-2` does not — so invite-email behaviour simply cannot be exercised on some workers' stacks, and a milestone that verifies it depends on which workstream happened to run it. That is a correctness-of-evidence problem, and it compounds with every new copy.
- **Correction to the hand-off brief:** the brief asked to gitignore `dev-*/bin` and `dev-*/logs`. That is already done — each `dev-N/.gitignore` lists `.kubeconfig`, `.pids/`, `logs/`, `bin/`, and `git ls-files` confirms zero tracked files under either. The 3.9 GB is untracked local disk, so the real work there is bounding log growth (t004), not gitignoring.
- **Render parity closing task omitted:** this touches no REST/GraphQL/MCP/UI surface and no product code — it is local developer tooling only.

## Constraints

- `/pm` is normally the only skill that writes to `.pm/`. This milestone deliberately restructures files under `.pm/wN/dev-N/`; that is dev-harness tooling, not board state, and no milestone/task/inbox file is touched. Recorded here so the exception is visible rather than looking like drift.
- Another worker may be mid-session on any `dev-N`. Migration must not delete a running environment out from under them — see t003.
