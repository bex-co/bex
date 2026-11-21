# w6 · m558 — A native service with a root directory runs from Render's `/opt/render/project/src/<rootDir>`, so a disk mounted at Render's path keeps its data

**Worker:** worker6 **Goal:** a Node, Python, Ruby, Elixir or Rust service with a root directory runs every process (start command, pre-deploy, cron run, Shell, SSH) in `/opt/render/project/src/<rootDir>`, as on Render, so a disk mounted at `/opt/render/project/src/<rootDir>/<dir>` holds what the app writes to `<dir>/`; the virtualenv and dependency caches sit where Render keeps them; and an existing service's disk under `/opt/render/project/src/…` still lands in its tree after its next deploy. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | Record where Render runs a service with a root directory, and decide the layout and how existing disks follow | 45m | — |
| [t002](t002.md) | Every native runtime builds and runs in `/opt/render/project/src/<rootDir>` | 60m | t001 |
| [t003](t003.md) | An existing service's disk under `/opt/render/project/src/…` keeps landing in its tree; the old-layout fixture on production | 60m | t002 |
| [t004](t004.md) | Ship and verify on production | 45m | t003 |
| [t005](t005.md) | Record the path in ADR060, ADR082, ADR018 and the `bex-disks` skill | 15m | t004 |
| [t006](t006.md) | Simplify | 15m | t005 |
| [t007](t007.md) | Test coverage | 30m | t005 |
| [t008](t008.md) | Closeout | 10m | t006, t007 |

Total: **8 tasks, about 4h40m**.

## Definition of done

- On production, a Starter native Node web service in `bex-canary` with root directory `backend` logs `pwd=/opt/render/project/src/backend` from its start command and its pre-deploy command, and `pwd` prints the same path in its Shell and an SSH session. A native cron job with a root directory logs the same shape of path from a run.
- That service, with a disk at `/opt/render/project/src/backend/uploads`, keeps a file it wrote to the relative path `uploads/` across a code deploy.
- A service built before the change with a disk under `/opt/render/project/src/…` (t003's old-layout fixture) still reads, after its next deploy, the file it wrote before the change, at the relative path it wrote it to.
- A native service with no root directory, and a Go service, get a byte-identical generated Dockerfile to before (unit tests).
- Python's virtualenv, Rust's `target` and Elixir's `deps` and `_build` sit where t001 found Render keeps them, and the first build after the change either seeds from the existing dependency cache or misses it once, recorded which.
- ADR060, ADR082 and ADR018 name the path.

## Source + Goal linkage

- **Source:** inbox note w6/158 ([done/158.md](../done/158.md)), filed by w6/m546 t001 on 2026-11-21 and promoted by `/loopx w6` on 2026-11-21. Evidence on `origin/main` `4d737506b`:
  - `nativeSourceDir` (`lego/operator/internal/build/native.go:579-586`) returns `nativeWorkdir` (`/opt/render/project/src`, `native.go:568`) for every runtime but Go, whatever `o.RootDir` is. The build context is the root directory alone (`boundedSourceDir(o.RootDir)`, `nativeresolve.go:411`), so `COPY . .` puts the root directory's contents straight into `/opt/render/project/src`.
  - Since w6/m546, a Go service's directory appends the root directory (`nativeGoSourceDir`, `native.go:616`), from Render's monorepo docs; the other runtimes do not.
  - Keyed to `nativeWorkdir` today: the Python virtualenv (`nativePythonVenv`, `native.go:563`), Rust's `target` and Elixir's `deps`/`_build` (`nativeDependencyDirs`, `depscache.go:107-108`), the static publish check (`nativePublishCheck`, `nativestaticdeps.go:162`). The publish Job's extract (`extractContainer`, `lego/operator/internal/publish/publish.go:653`) reads `PUBLISH_PATH` relative to the image's `WORKDIR`. native-init links secret files into its working directory (`lego/operator/cmd/native-init/secretfiles.go`).
  - Render: "Render runs commands and interacts with files relative to your service's root directory" ([render.com/docs/monorepo-support](https://render.com/docs/monorepo-support), read 2026-11-20). Build errors Render users published for a root directory `backend` name `/opt/render/project/src/backend/…`.
- **Goal linkage:** [ADR008 § Render parity first](../../../docs/ADR008-vision.md#render-parity-first): an app migrated from Render runs unchanged, its disk included.
- **Expected outcome:** a monorepo service configured from Render's docs stops writing its uploads into the image layer and losing them on every deploy, and a bex service that mounted its disk for the old layout keeps its data.
- **Why now:** it is the same silent data loss w6/m546 fixed for Go, on the runtimes most services use. Every new service with a root directory and a disk configured for bex's current layout is one more to carry across the move.
- **Why this is not a link like m546's:** m546 made `/opt/render/project/src` a link to the Go directory, outside it. Here the new directory sits inside the old one, so `/opt/render/project/src` cannot be a link to its own child. How an existing disk follows is t001's design decision.
- **Render parity task omitted:** the working directory is runtime mechanism, and no REST, GraphQL, MCP or dashboard field changes, as in m546. The `bex-disks` agent skill's example path is documentation, covered by t005. If t001 finds the mount-path denylist or a dashboard hint must change, it files that as a follow-up note.
