# w6 · m21 — Build/start command override

**Worker:** worker6 **Goal:** close a documented Render service-create field gap — explicit Dockerfile path + start-command override **Status:** todo

## Tasks (in order)

| id   | title                                                                                          | est  | depends_on |
| ---- | ----------------------------------------------------------------------------------------------- | ---- | ---------- |
| t001 | `App.Spec.DockerfilePath`, `StartCommand` fields (types + deepcopy + CRD yaml regen)             | 45m  | —          |
| t002 | operator/build.go: thread `DockerfilePath` into the BuildKit `--opt filename=` arg               | 1h   | t001       |
| t003 | operator app_controller.go: thread `StartCommand` into the Deployment container `Command`/`Args` | 1h   | t001       |
| t004 | backend/internal/apps: thread both fields through bex.yml ingestion + REST/GraphQL/MCP App create/update | 1.5h | t002, t003 |
| t005 | Render parity: verify field shape/semantics consistent across REST/GraphQL/MCP + dashboard UI    | 45m  | t004       |
| t006 | Simplify                                                                                          | 30m  | t005       |
| t007 | Test coverage                                                                                     | 1h   | t005       |
| t008 | Closeout                                                                                           | 15m  | t007       |

## Definition of done

An App can specify a non-root-relative Dockerfile path and an explicit container start command via bex.yml/REST/GraphQL/MCP, and both take effect on the resulting build and Deployment — verified with a build using a non-default Dockerfile location and a Deployment whose running command differs from the image's default `ENTRYPOINT`/`CMD`.

## Source + Goal linkage

- **Source:** `/pm-brainstorm more milestones` 2026-07-13 — `docs/ADR018-render-parity.md` service-create-fields row, corroborated by `lego/backend/internal/apps/mcp.go:106`'s own comment noting Render's `buildCommand`/`startCommand` aren't supported today. Materialized under `w6` **per user direction**.
- **Goal linkage:** Render parity — closes a documented divergence in the create-service surface.
- **Expected outcome:** repos needing a non-default Dockerfile location or a custom container start command (common for monorepos/multi-process images) no longer need workarounds.
- **Why now:** low-risk, self-contained, unblocks users hitting this today. Deliberately scoped to Dockerfile-only mechanics — Render's `buildCommand` has no equivalent for Docker-runtime services either, so it's correctly deferred to a future CNB/buildpack milestone where it gets real semantics (buildpack build-time env vars). Render parity included — this changes the create-service field surface across REST/GraphQL/MCP and should be checked against dashboard UI.
