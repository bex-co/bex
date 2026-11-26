# w4 · m650 — A native build reads the whole repository, as Render's native runtimes do

**Worker:** worker4 **Goal:** a native-runtime service with a Root Directory builds and runs with the rest of its repository in reach, so a Go module rooted above the Root Directory or a pnpm/npm workspace builds on bex as it does on Render, while Docker builds keep seeing only their Root Directory or Docker context. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | Record Render's current Root Directory wording and supersede the board's old-wording exclusions | 30m | — |
| [t002](t002.md) | A native build's context is the whole repository; its commands still run in the Root Directory | 1h30m | t001 |
| [t003](t003.md) | Disk mounts and every other root-only assumption follow the whole-repository image | 1h | t002 |
| [t004](t004.md) | Layer cache, image size and secret files stay sound | 45m | t002 |
| [t005](t005.md) | dev-4: a Go module above `cmd/api` and a pnpm workspace's `apps/web` build and serve | 1h | t002 |
| [t006](t006.md) | A push to a build-filter path outside the root redeploys, and the build reads the change | 30m | t005 |
| [t007](t007.md) | Simplify | 15m | t003, t004, t006 |
| [t008](t008.md) | Test coverage | 30m | t007 |
| [t009](t009.md) | Closeout | 10m | t008 |

Total: **9 tasks, about 6h10m**.

## Definition of done

On dev-4, a native Go service whose `go.mod` sits above its Root Directory `cmd/api` builds and serves, and a native Node service in a pnpm workspace with Root Directory `apps/web` resolves `packages/ui` at build and at run time. A Dockerfile service with a Root Directory still cannot copy a parent file, as on Render. An existing native service with a disk keeps its mount path across the change. A push that changes only a file outside the Root Directory deploys only when a build filter includes it, and that build reads the change. ADR018's "Runtime, build strategy + Root Directory" row states the new behavior with Render's 2026-11-25 wording.

## Source + Goal linkage

- **Source:** `/pm-brainstorm one aspect of render parity to w4`, 2026-11-25 (the aspect: creating a service from a repository, as Render's create docs describe), item 1. Found by an independent research agent; the Render quote and the bex code were re-checked by hand the same day. Filed by `/pmx them all to w4` at `origin/main` `c196617a8b`.
- **Evidence (2026-11-25):** [render.com/docs/monorepo-support](https://render.com/docs/monorepo-support) § Setting a root directory: "Docker-based services can't access files in your repo outside their root directory." and "Services on a native runtime _can_ access files outside their root directory, but changes to those files don't trigger an autodeploy." The older sentence ("not available to the service at build time or at runtime") is gone. bex: `dockerContextDir` (`lego/operator/internal/build/dockerinputs.go:180`) is the Docker context or the Root Directory for every builder, and the native Dockerfile's `COPY . .` (`native.go:529`) copies only that into `nativeRootDir` (`native.go:663`). The board excluded this three times on the old wording: [w7/m428 t003](../../w7/done/m428/done/t003.md), [w6/m558](../../w6/done/m558/README.md), [w8/399](../../w8/done/399.md).
- **Goal linkage:** [ADR008 § Render parity first](../../../docs/ADR008-vision.md#render-parity-first): a repository that builds on Render must build on bex, the migration path of a Render alternative.
- **Expected outcome:** Go modules with several packages and pnpm, npm or yarn workspaces build on bex with a Root Directory, instead of failing on missing parent files.
- **Why now:** the exclusions rest on wording Render removed, and w7/m428 already deploys a push to an included path outside the Root Directory that the build then cannot read, so the two halves disagree today.
- **Render parity:** omitted. The change is operator build mechanism with no REST, GraphQL, MCP or dashboard surface; t001 amends the ADR018 row itself. No user decision is open: the tasks follow Render's documented behavior under the parity rule.
