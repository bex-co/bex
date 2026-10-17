# w1 · m181 — Auto-deploy after CI checks pass (`autoDeployTrigger: checksPass`)

**Worker:** worker1 **Goal:** a Git service set to "After CI Checks Pass" deploys a pushed commit only when all of its GitHub checks pass, on every surface and from Blueprints, as on Render. **Status:** todo

## Tasks (in order)

| id   | title                                                                                     | est   | depends_on |
| ---- | ----------------------------------------------------------------------------------------- | ----- | ---------- |
| t001 | Record Render's After-CI-Checks-Pass contract                                              | 30m   | —          |
| t002 | GitHub App: request checks read access and check events; refuse checksPass clearly when they are missing | 45m | t001 |
| t003 | Gate auto-deploy on the commit's checks                                                    | 1h30m | t002       |
| t004 | Accept checksPass on REST, GraphQL, MCP, Blueprints and the dashboard                      | 1h    | t003       |
| t005 | Update the parity docs                                                                     | 20m   | t004       |
| t006 | Live check with a GitHub Actions repository                                                | 45m   | t004       |
| t007 | Render parity                                                                              | 20m   | t005, t006 |
| t008 | Simplify                                                                                   | 15m   | t007       |
| t009 | Test coverage                                                                              | 40m   | t007       |
| t010 | Closeout                                                                                   | 10m   | t009       |

## Definition of done

- With `autoDeployTrigger: checksPass`, a pushed commit deploys only when all its GitHub checks conclude `success`/`neutral`/`skipped`; a failing or check-less commit does not deploy; a newer push supersedes a pending one (t001 contract).
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
