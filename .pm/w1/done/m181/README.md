# w1 · m181 — Auto-deploy after CI checks pass (`autoDeployTrigger: checksPass`)

**Worker:** worker1 **Goal:** a Git service set to "After CI Checks Pass" deploys a pushed commit only when all of its GitHub checks pass, on every surface and from Blueprints, as on Render. **Status:** done (2026-10-26: t006 live check passed after the App owner subscribed bex-co to Check suite)

## Tasks (in order)

| id   | title                                                                                     | est   | depends_on |
| ---- | ----------------------------------------------------------------------------------------- | ----- | ---------- |
| t001 | Record Render's After-CI-Checks-Pass contract — **DONE** | 30m   | —          |
| t002 | GitHub App: request checks read access and check events; refuse checksPass clearly when they are missing — **DONE** | 45m | t001 |
| t003 | Gate auto-deploy on the commit's checks — **DONE** | 1h30m | t002       |
| t004 | Accept checksPass on REST, GraphQL, MCP, Blueprints and the dashboard — **DONE** | 1h    | t003       |
| t005 | Update the parity docs — **DONE** | 20m   | t004       |
| t011 | Make checked-commit acceptance and release handoff durable and idempotent — **DONE** | 60m | t003 |
| t012 | Recover interrupted checked-commit dispatch — **DONE** | 60m | t011 |
| t013 | Fence checked-commit retries against supersession and current settings — **DONE** | 45m | t012 |
| t006 | Live check with a GitHub Actions repository — **DONE**                                                | 45m   | t004, t013 |
| t007 | Render parity — **DONE** | 20m   | t005, t006, t013 |
| t008 | Simplify — **DONE** | 15m   | t007       |
| t009 | Test coverage — **DONE** | 40m   | t007       |
| t010 | Closeout — **DONE**                                                                                   | 10m   | t009       |

## Definition of done

- With `autoDeployTrigger: checksPass`, a pushed commit deploys only when all its GitHub checks conclude `success`/`neutral`/`skipped`; a failing or check-less commit does not deploy (a passing re-run of a failed check does, amended 2026-10-17 as a bex decision); a newer push supersedes a pending one (t001 contract).
- REST, GraphQL, MCP, the Blueprint compiler and the dashboard accept and read back `checksPass`; `commit` and `off` are unchanged.
- An installation without `checks: read` gets a coded refusal naming the permission.
- t006's live check passes on a real repository.

## Evidence (verified 2026-10-17 at `2c95fd35b`)

- Refused today: `lego/backend/internal/apps/rest.go:626-627`, `graphql.go:447`, `blueprint_compiler.go:726-727`; the webhook ignores every event but `push` (`webhook.go:307-315`); the App manifest asks only `contents` + `metadata` and `push` (ADR026 §2).
- Recorded as unbuilt, not a non-goal: `docs/ADR049-render-yaml-parity.md:172` ("unsupported until bex can observe and gate on branch checks"); `docs/ADR018-render-parity.md:82,191`; w5/m53 and w1/m63/t009 (done) deferred it.
- Render: render.com/docs/deploys, fetched 2026-10-17 (see t001).

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w1` 2026-10-17 item 5 (ADR018 unowned-gap sweep), verified the same day and approved by the user for filing.
- **Goal linkage:** `render.yaml` parity (ADR049) and GitHub integration (ADR026) under the Render-alternative core (ADR008).
- **Expected outcome:** teams with CI migrate their `render.yaml` unchanged and keep their deploy gate instead of stripping it.
- **Why now:** it is the one Render value of a core service field bex still refuses on every surface, and the GitHub connection rework it builds on has shipped (w2/m162 code is in; only its live walk is blocked, and it does not conflict).
- **Render parity included:** REST, GraphQL, MCP, Blueprints and the dashboard change.
- **Cost / risk:** existing GitHub Apps (bex's own and every self-hosted one) must be granted `checks: read` by their owners; until then `checksPass` is refused with the coded error, never silently treated as `commit`. GitLab/Bitbucket stay out (DO_NOT_DO).

## Approved correctness extension — 2026-10-20

The user approved brainstorm item 3 as a correctness repair to the existing Render-compatible feature. Render documents the CI pass/fail/zero-check gate at https://render.com/docs/deploys#integrating-with-ci. **Crash recovery, exactly-once release handoff, rerun behavior and newer-push supersession are bex correctness choices; Render's equivalent guarantees are undocumented and were not measured.** A durable intent/worker is a possible implementation, not a Render contract; choose the smallest mechanism that meets the observable cases below.

`checks_pass.go` removes the awaited marker before `redeployFetched`; interruption there loses accepted work. Its failure restoration uses an unconditional merge patch, so restoring A can overwrite newer B. Source-derived counterexamples, not a live reproduction.

Additional definition of done: interruption between claim and release creation recovers one release; duplicate deliveries cannot create another for the same accepted dispatch; late restoration of A cannot replace B; fresh trigger/source settings govern retry. t011–t013 are implementable now; the owner-only Check suite event subscription continues to gate t006 and final closeout. Repeat closing reviews for this change rather than treating their 2026-10-17 runs as current evidence.

## Implementation checkpoint — 2026-10-20

Durable receipt recovery and source-revision fencing are implemented and verified locally. No public field or UI choice changed. The owner-only live gate remains: `gh api /apps/bex-co` reports checks write, events deployment/push, and no check_suite. t006, final live parity in t007 and t010 remain open; this milestone is parked rather than marked done.

## Closeout — 2026-10-26

The gate was cleared once the production App subscribed to Check suite, which the agent did in the owner's signed-in browser on the user's instruction. On a real GitHub Actions repository (t006):

- a failing check did not deploy;
- a passing check deployed about 4s after the suite completed;
- of two back-to-back pushes, only the newer one deployed.

Fixtures were deleted.
