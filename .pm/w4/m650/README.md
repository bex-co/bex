# w4 · m650 — A native build reads the whole repository, as Render's native runtimes do

**Worker:** worker4 **Goal:** a native-runtime service with a Root Directory builds and runs with the rest of its repository in reach, so a Go module rooted above the Root Directory or a pnpm/npm workspace builds on bex as it does on Render, while Docker builds keep seeing only their Root Directory or Docker context. **Status:** todo (t001–t004, t007 and t008 done; t005, t006 and t009 wait for the production walk)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Record Render's current Root Directory wording and supersede the board's old-wording exclusions — **DONE** | 30m | — |
| [t002](done/t002.md) | A native build's context is the whole repository; its commands still run in the Root Directory — **DONE** | 1h30m | t001 |
| [t003](done/t003.md) | Disk mounts and every other root-only assumption follow the whole-repository image — **DONE** | 1h | t002 |
| [t004](done/t004.md) | Layer cache, image size and secret files stay sound — **DONE** | 45m | t002 |
| [t005](t005.md) | production: a Go module above `cmd/api` and a pnpm workspace's `apps/web` build and serve | 1h | t002 |
| [t006](t006.md) | A push to a build-filter path outside the root redeploys, and the build reads the change | 30m | t005 |
| [t007](done/t007.md) | Simplify — **DONE** | 15m | t003, t004, t006 |
| [t008](done/t008.md) | Test coverage — **DONE** | 30m | t007 |
| [t009](t009.md) | Closeout | 10m | t008 |

Total: **9 tasks, about 6h10m**.

## Definition of done

On production (moved from dev-4 on 2026-11-25, § Record), a native Go service whose `go.mod` sits above its Root Directory `cmd/api` builds and serves, and a native Node service in a pnpm workspace with Root Directory `apps/web` resolves `packages/ui` at build and at run time. A Dockerfile service with a Root Directory still cannot copy a parent file, as on Render. An existing native service with a disk keeps its mount path across the change. A push that changes only a file outside the Root Directory deploys only when a build filter includes it, and that build reads the change. ADR018's "Runtime, build strategy + Root Directory" row states the new behavior with Render's 2026-11-25 wording.

## Source + Goal linkage

- **Source:** `/pm-brainstorm one aspect of render parity to w4`, 2026-11-25 (the aspect: creating a service from a repository, as Render's create docs describe), item 1. Found by an independent research agent; the Render quote and the bex code were re-checked by hand the same day. Filed by `/pmx them all to w4` at `origin/main` `c196617a8b`.
- **Evidence (2026-11-25):** [render.com/docs/monorepo-support](https://render.com/docs/monorepo-support) § Setting a root directory: "Docker-based services can't access files in your repo outside their root directory." and "Services on a native runtime _can_ access files outside their root directory, but changes to those files don't trigger an autodeploy." The older sentence ("not available to the service at build time or at runtime") is gone. bex: `dockerContextDir` (`lego/operator/internal/build/dockerinputs.go:180`) is the Docker context or the Root Directory for every builder, and the native Dockerfile's `COPY . .` (`native.go:529`) copies only that into `nativeRootDir` (`native.go:663`). The board excluded this three times on the old wording: [w7/m428 t003](../../w7/done/m428/done/t003.md), [w6/m558](../../w6/done/m558/README.md), [w8/399](../../w8/done/399.md).
- **Goal linkage:** [ADR008 § Render parity first](../../../docs/ADR008-vision.md#render-parity-first): a repository that builds on Render must build on bex, the migration path of a Render alternative.
- **Expected outcome:** Go modules with several packages and pnpm, npm or yarn workspaces build on bex with a Root Directory, instead of failing on missing parent files.
- **Why now:** the exclusions rest on wording Render removed, and w7/m428 already deploys a push to an included path outside the Root Directory that the build then cannot read, so the two halves disagree today.
- **Render parity:** omitted. The change is operator build mechanism with no REST, GraphQL, MCP or dashboard surface; t001 amends the ADR018 row itself. No user decision is open: the tasks follow Render's documented behavior under the parity rule.

## Record

**2026-11-25, t001–t004, t007, t008 (worker4).** Code and docs shipped together; see each task's Result.

- **Why the live walk moves to production.** dev-4 cannot build a native service on this host. The shared local cluster's nodes are arm64, and the pinned common base (`ghcr.io/bex-co/bex-cnb-run@sha256:49dbbd72…`, `nativebases.go`) has no arm64 manifest: BuildKit answers `no match for platform in manifest`, and every native App on the local cluster read `Failed` with "bex could not download its native runtime image (from ghcr.io)" on 2026-11-25. So t005 and t006 run on production (amd64), as w6/m546, w6/m558 and w7/m428 did, once the deploy carries the code. The same fixtures already pass locally on real BuildKit through the Job's own arguments (`TestNativeImageReadsTheWholeRepository`, official images):
  - Go, `go.mod` at the top, Root Directory `cmd/api`, `go build -o app .`: served `read from internal/greet in /opt/render/project/go/src/github.com/example/app/cmd/api`.
  - pnpm workspace, Root Directory `apps/web`, `pnpm install --no-frozen-lockfile`, `node server.js`: served `text from packages/ui in /opt/render/project/src/apps/web`, with a repository `.dockerignore` of `**` / `!apps/api` that would have left `apps/web` out.
  - Docker control, Root Directory `svc`, `COPY ../shared/file.txt /file.txt`: failed with `"/shared/file.txt": not found`; the same Dockerfile with Docker context `.` built.
- **Measurements (t004), local BuildKit harness on Docker Desktop (arm64), Go fixture with Root Directory `cmd/api`.** Before the change a file outside the Root Directory never reached the build; the "with" row is what it costs now, for a tree shaped like the largest first-party monorepo below (2,700 files, 20 MB, incompressible bytes as the worst case).

  | Outside the Root Directory | Context transfer | `COPY . .` step | OCI archive | Build wall time |
  | --- | --- | --- | --- | --- |
  | nothing (as before) | 193 B, 0.0 s | 0.0 s | 441.7 MiB | 36–38 s |
  | 2,700 files, 20.1 MB | 0.8–9.4 s | 0.4–1.1 s | 460.8 MiB (+19.1 MiB) | 87–145 s |

  The wall times include each run's fresh image pulls and Docker Desktop's macOS file sharing, which the transfer column shows is slow (9.4 s for 20 MB); a Linux build node reads its checkout from a local `emptyDir`. Source text compresses, so a real repository adds less than the incompressible case.
- **What production's monorepo services gain.** `bex-co/block-eden-mono`, workspace `bex`, whose two native services set a Root Directory (w6/m558's production read, 2026-11-21), is 2,702 files and 19.8 MiB at `main` (GitHub tree API, 2026-11-25). `projects/eden-dash-v3` is 7.9 MiB and `projects/reverse-proxy-poc` 1.5 MiB, so their images grow by at most 11.9 MiB and 18.3 MiB of source before compression.
- **Next (t005, t006, t009).** Once production's pin carries the code commit: in `bex-canary` as the QA user, the Go and pnpm fixtures from a public repository build and serve, and the Docker control fails; then t006's two pushes in workspace `bex` with a temporary bex-co repository, as w7/m428 t004 did (`bex-canary` has no Git connection). Then t009.
