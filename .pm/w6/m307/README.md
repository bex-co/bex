# w6 · m307 — `build_seconds` counts what Render counts as pipeline minutes

**Worker:** worker6 **Goal:** the build meter follows Render's pipeline-minute basis: every build that ran (failed ones included), pre-deploy runs and buildpack builds, timed from the build pod's start. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [Failed builds are metered](t001.md) | 45m | — |
| t002 | [The build clock starts when the build pod starts](t002.md) | 1h | — |
| t003 | [Pre-deploy runs count toward `build_seconds`](t003.md) | 45m | — |
| t004 | [Buildpack (kpack) builds are metered](t004.md) | 1h | — |
| t005 | [Record the new basis in ADR023](t005.md) | 20m | t001, t002, t003, t004 |
| t006 | [Render parity](t006.md) | 30m | t005 |
| t007 | [Simplify](t007.md) | 15m | t006 |
| t008 | [Test coverage](t008.md) | 30m | t007 |
| t009 | [Closeout](t009.md) | 10m | t008 |

## Definition of done

- Usage unit tests cover failed builds, pre-deploy runs, kpack builds and the pod-start clock.
- On production after the deploy, one deliberately failing build and one service with a pre-deploy command each appear in their hour's `build_seconds` on `GET /v1/usage`, and a build's metered seconds exclude its node wait (compared with `bex_build_queue_seconds` for that build).
- ADR023 records the shipped basis.

## Source + Goal linkage

- **Source:** this session's 2026-11-10 discussion of the tenant build experience against Render, then `/pm-brainstorm for w6` (same session) and `/pmx them all to w6`; docs/ADR023 § `build_seconds` against Render's pipeline minutes (2026-11-10) found the four differences in `queryBuildSeconds`.
- **Goal linkage:** ADR008 Render parity first; Render counts the build command, Dockerfile builds and the pre-deploy command as pipeline minutes ([build pipeline](https://render.com/docs/build-pipeline)).
- **Expected outcome:** customers pay for the build work they caused, and not for the platform's node wait.
- **Why now:** `m308`'s included allowance is only right if the minutes are counted on Render's basis, so this milestone goes first.
- **Render parity task:** included. Usage numbers are tenant-visible on REST, GraphQL, MCP and the dashboard.
- **Unverified, recorded as the parity-sufficient reading (ADR030 §13), not a user decision:** whether Render charges failed or canceled builds; bex counts every build that ran.
