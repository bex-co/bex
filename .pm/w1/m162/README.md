# w1 · m162 — Native-runtime monorepo builds: rootDir sets the working directory, not the build context

**Worker:** worker1 **Goal:** a service inside a workspace monorepo (npm/pnpm workspaces, `go.work`) with `rootDir` set builds and runs on bex the way it does on Render — the whole repository is available, and build/start commands run from the root directory. **Status:** todo

## Tasks (in order)

| id   | title                                                                                                      | est | depends_on |
| ---- | ---------------------------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Capture Render's Root Directory and monorepo semantics                                                     | 30m | —          |
| t002 | Native BuildKit path: the context stays the repo root and the generated Dockerfile works from rootDir      | 60m | t001       |
| t003 | kpack path: keep the full clone for workspace monorepos and pass the buildpack project path                | 60m | t001       |
| t004 | Check in `examples/shared-package-monorepo` as the build fixture                                           | 30m | t002       |
| t005 | Correct ADR004 and ADR018 row 69                                                                           | 20m | t002, t003 |
| t006 | Live: the example monorepo deploys with `rootDir: apps/api` and serves the shared greeting                 | 45m | t004, t005 |
| t007 | Render parity                                                                                              | 20m | t006       |
| t008 | Simplify                                                                                                   | 15m | t007       |
| t009 | Test coverage                                                                                              | 40m | t007       |
| t010 | Closeout                                                                                                   | 10m | t009       |

## Definition of done

- **The example deploys.** `examples/shared-package-monorepo` (checked in by t004) deploys as a Node native web service with `rootDir: apps/api` and answers `hello-from-shared:api:1.0.0` on its URL; the same repo fails to build on the pre-fix operator and both observations are recorded in this README.
- **Nothing else moves.** A Dockerfile service with `dockerContext` set produces the same build Job args as before; a rootDir that escapes the checkout is still refused; a non-workspace repo on the `buildpack` extension renders the same kpack Image as today.
- **The docs stop overclaiming.** ADR004 describes rootDir as the working directory and ADR018 row 69 reflects the shipped behavior with a dated Render artifact.

## Root cause

- `lego/operator/internal/build/build.go:850` — `contextDir := boundedSourceDir(o.RootDir)` narrows the BuildKit context to the root directory for native builds, so the workspace root `package.json` and `packages/shared` are not in the context.
- `lego/operator/internal/build/kpack.go:108` — `source.subPath = rootDir` does the same for the `buildpack` extension.
- Render clones the whole repository and runs commands from the root directory (t001 captures the evidence).

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w1` 2026-09-20, item 2. Evidence: the untracked `examples/shared-package-monorepo/` scaffold in the working tree, `build.go:850`, `kpack.go:108`, ADR004:186/284, ADR018:69.
- **Goal linkage:** `docs/ADR018-render-parity.md` (Root Directory is a Render-facing field on the primary build path) and `docs/ADR004-app-deployment.md` monorepo support.
- **Expected outcome:** workspace monorepos deploy on bex without restructuring; the parity ledger row is true.
- **Why now:** the scaffold in the working tree shows the shape is being tried today, and the ledger currently claims ✅ for behavior bex does not have.
- **Render parity:** included — the change is user-facing build semantics exposed through `rootDir` on REST, GraphQL, MCP and the dashboard Settings page.
