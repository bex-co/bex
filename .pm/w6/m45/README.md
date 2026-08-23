# w6 · m45 — Live QA hunt: native build PATH loss, invisible creation-time env vars, stale post-cancel status

**Worker:** worker6 **Goal:** a native Go/Rust web service actually builds and deploys; an env var set at service-creation time is visible, editable, and exportable through the same Environment tab/API that vars added later already use; and the service header's status pill matches the deploy's terminal state immediately after Cancel/Rollback, not after a 30s poll or a reload. **Status:** todo

## Tasks (in order)

| id   | title                                                                                                                      | est | depends_on               |
| ---- | --------------------------------------------------------------------------------------------------------------------------- | --- | ------------------------ |
| t001 | Fix native build/start command losing PATH under the login-shell wrapper (breaks Go, likely Rust)                          | 45m | —                         |
| t002 | Seed creation-time literal env vars into the secrets store so the Environment tab/API see them                              | 45m | —                         |
| t003 | Refetch the Server query after deploy Cancel/Rollback so the header status pill isn't stale                                 | 30m | —                         |
| t004 | Dashboard polish: dedupe live-tail reconnect log lines, runtime auto-detect on Root Directory change, deploy commit/date spacing | 30m | —                     |
| t005 | Render parity across REST/GraphQL/MCP/UI                                                                                     | 30m | [t001, t002, t003, t004] |
| t006 | Simplify the touched code                                                                                                    | 30m | t005                     |
| t007 | Test coverage for the fixed behaviors                                                                                        | 45m | t005                     |
| t008 | Closeout                                                                                                                      | 10m | t007                     |

## Definition of done

- Deploying a native Go OR Rust web service from a public repo reaches Live and its `.onbex.co` URL serves — verified live, not just via the generated Dockerfile text.
- An env var set in the New Web Service creation form is immediately visible in `envVars(serviceId)` and the Environment tab, without a follow-up Edit+Save.
- The service header's status pill matches the deploy's actual terminal state immediately after Cancel or Rollback, on all 3 surfaces that use `DeployActions`, with no reload needed.
- Regression tests exist for all of the above and fail on the pre-fix code.

## Source + Goal linkage

- **Source:** live QA hunt of `dashboard.bex.co` hosting features, 2026-08-22 (via `/qa-find-bugs`, run from a `/loop 10m` session). Signed in as workspace "bex". Evidence under `.playwright-mcp/` (gitignored): `qa-services-1-go-build-fail.png`, `qa-deploys-1-text-concat.png`, `qa-env-1-missing-var.png`.
- **Goal linkage:** core hosting-journey correctness (deploy a service, manage its config, trust its status) — `docs/ADR004-app-deployment.md`, `docs/ADR006-bex-api.md`, `docs/ADR018-render-parity.md`.
- **Expected outcome:** a customer deploying a Go or Rust service from a public repo gets a working service instead of a build that fails with a confusing toolchain error; a customer who sets an env var at creation time can see/edit/export it like any other; canceling or rolling back a deploy never leaves the header lying about the service's state.
- **Why now:** found in a real live hunt of production; all three primary findings have clean, isolated, file:line-pinned root causes and t001/t002 are correctness bugs on core hosting journeys (create a service, manage its env), not polish. Render parity closing task **included** — t001 touches the build pipeline surface exposed identically across native runtimes, and t002/t003 touch REST/GraphQL/MCP/UI env-var and deploy-action surfaces directly.

## Investigated and rejected (not filed)

- **Free-tier service manually scaling past 1 instance.** This hunt observed scaling a Free-instance-type test service to 4 (of a 100 platform ceiling) real running replicas with no plan-based rejection. Checked before filing: `docs/ADR018-render-parity.md` line 66 documents this as a **deliberate, user-confirmed divergence** from Render (`w1/m81`, 2026-08-19 — "bex has no per-plan replica cap... this is a plan-policy difference, not a UI-validation gap"). Not a bug; no task filed. The test service was scaled back to 1 instance during this hunt's cleanup.
