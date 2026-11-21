# w6 · m558 — A native service with a root directory runs from Render's `/opt/render/project/src/<rootDir>`, so a disk mounted at Render's path keeps its data

**Worker:** worker6 **Goal:** a Node, Python, Ruby, Elixir or Rust service with a root directory runs every process (start command, pre-deploy, cron run, Shell, SSH) in `/opt/render/project/src/<rootDir>`, as on Render, so a disk mounted at `/opt/render/project/src/<rootDir>/<dir>` holds what the app writes to `<dir>/`; the virtualenv and dependency caches sit where Render keeps them; and an existing service's disk under `/opt/render/project/src/…` still lands in its tree after its next deploy. **Status:** todo (t001–t003, t005–t007 done)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [Record where Render runs a service with a root directory, and decide the layout and how existing disks follow](done/t001.md) — **DONE** | 45m | — |
| t002 | [Every native runtime builds and runs in `/opt/render/project/src/<rootDir>`](done/t002.md) — **DONE** | 60m | t001 |
| t003 | [An existing service's disk under `/opt/render/project/src/…` keeps landing in its tree; the old-layout fixture on production](done/t003.md) — **DONE** | 60m | t002 |
| [t004](t004.md) | Ship and verify on production | 45m | t003 |
| t005 | [Record the path in ADR060, ADR082, ADR018 and the `bex-disks` skill](done/t005.md) — **DONE** | 15m | t004 |
| t006 | [Simplify](done/t006.md) — **DONE** | 15m | t005 |
| t007 | [Test coverage](done/t007.md) — **DONE** | 30m | t005 |
| [t008](t008.md) | Closeout | 10m | t006, t007 |

Total: **8 tasks, about 4h40m**.

## Definition of done

- On production, a Starter native Node web service in `bex-canary` with root directory `backend` logs `pwd=/opt/render/project/src/backend` from its start command and its pre-deploy command, and `pwd` prints the same path in its Shell and an SSH session. A native cron job with a root directory logs the same shape of path from a run.
- That service, with a disk at `/opt/render/project/src/backend/uploads`, keeps a file it wrote to the relative path `uploads/` across a code deploy.
- A service built before the change with a disk under `/opt/render/project/src/…` (t003's old-layout fixture) still reads, after its next deploy, the file it wrote before the change, at the relative path it wrote it to.
- A native service with no root directory, and a Go service, get a byte-identical generated Dockerfile to before (unit tests).
- Python's virtualenv, Rust's `target` and Elixir's `deps` and `_build` sit where t001 found Render keeps them, and the first build after the change either seeds from the existing dependency cache or misses it once, recorded which.
- ADR060, ADR082 and ADR018 name the path.

## t001 record: where Render runs a service with a root directory, and the layout (2026-11-21)

- **Render's evidence, read 2026-11-21.**
  - [render.com/docs/monorepo-support](https://render.com/docs/monorepo-support): "Render runs commands and interacts with files relative to your service's root directory", and "Files outside your service's root directory are not available to the service at build time or at runtime." The build, start and pre-deploy commands and the publish directory are all listed as relative to it. **Verified.**
  - Node, root directory `backend`, in logs Render users published: `Cannot find package 'express' imported from /opt/render/project/src/backend/server.js`, and a TypeScript build with "Root Directory: backend" naming `/opt/render/project/src/backend/node_modules/express/index.js`. So the build and the service both run in `/opt/render/project/src/<rootDir>`, with `node_modules` there. **Verified.**
  - Python: [netgainsapp/jimbo-connect#81](https://github.com/netgainsapp/jimbo-connect/issues/81), filed 2026-08-02. The `render.yaml` at the time (`ee227567e7`) says `runtime: python`, `rootDir: backend` and `pip install -r requirements.txt`. Its traceback runs `/opt/render/project/src/backend/server.py` from `/opt/render/project/src/.venv/lib/python3.12/site-packages/…`. So the virtualenv stays at VENV_ROOT's documented `/opt/render/project/src/.venv`, outside the root directory. **Verified for pip.** uv with a root directory is **unverified**: `uv sync` installs into the project's own `.venv` (`<rootDir>/.venv`), not `VIRTUAL_ENV`. Render documents no `UV_PROJECT_ENVIRONMENT`, so bex sets none.
  - [render.com/docs/disks](https://render.com/docs/disks): "Node.js, Python, Ruby, Elixir, and Rust" → `/opt/render/project/src`. The reserved list is exact paths only. "You can't access persistent disks during a service's build command or pre-deploy command", and a cron job can't have a disk. **Verified.**
  - [render.com/docs/environment-variables](https://render.com/docs/environment-variables): VENV_ROOT `/opt/render/project/src/.venv`, CARGO_HOME `/opt/render/project/.cargo`, BUNDLE_PATH and GEM_PATH `/opt/render/project/.gems`, GOPATH `/opt/render/project/go`. None names a root directory, and the page never mentions one. **Verified.**
  - Rust's `target`: Render says "We're caching any target directory at your repo root". Cargo writes `target` beside the root directory's `Cargo.toml`, which is `<rootDir>/target`. Whether Render caches that one is **unverified**.
  - Elixir: Mix writes `deps` and `_build` in the project directory, the root directory. **Inferred.**
  - Static sites: the monorepo docs list the build command and the publish directory as relative to the root directory. A Vite build error published on Render's forum names `/opt/render/project/src/frontend/vite.config.js` (render.discourse.group thread 21598, which this machine could not resolve, 2026-11-21). **Inferred.**
- **Layout (t002):**
  - The `WORKDIR` is `/opt/render/project/src/<rootDir>` for every native runtime but Go (Node, Python, Ruby, Elixir, Rust) and for every static site, whatever its toolchain.
  - No root directory, `.`, `/`, or a part outside `[A-Za-z0-9._-]` (the `WORKDIR` line expands `$` and `\`) keeps `/opt/render/project/src`, byte for byte as before.
  - `/opt/render/project/src` is the parent of the new directory, so it is never a link.
  - A Go service keeps m546's GOPATH directory, which already appends the root directory, and its link.
  - Python's virtualenv stays at `/opt/render/project/src/.venv`, so its dependency cache key is unchanged and an existing cache still seeds.
  - Rust's `target` and Elixir's `deps` and `_build` move to `<WORKDIR>/…`, where Cargo and Mix write them. The first build after the change finds them in the cache under their old paths, which the seed discards, so it misses the cache once for these two runtimes when the service has a root directory (production has none).
  - The static publish check looks from the `WORKDIR`. The publish Job's extract already reads `PUBLISH_PATH` relative to the image's `WORKDIR`, and native-init links secret files into its working directory, so neither changes.
- **Production, read 2026-11-21T15:33Z:** 28 Apps. 6 are native: 4 Node and 2 Go. Two Node services have a root directory, `block-eden-mono` (`projects/eden-dash-v3`) and `tianpan-v4-web` (`projects/tianpan-v4-web`). Neither has a disk, and both run relative commands (`yarn build`, `yarn start`). No native service other than Go has a disk, and there are no Python, Ruby, Elixir or Rust services and no static site Apps. Those two move on their next deploy, and no existing disk needs to follow. The old-layout path is checked on t003's fixture.
- **Existing disks: candidate (b), the operator mounts the disk where its path pointed before.** A claim the operator makes from now on carries `app.bex.co/disk-layout: root-directory`. A claim without it predates the change. For a native non-Go service with a root directory whose disk path is under `/opt/render/project/src/` but outside `<rootDir>/`, a disk with such a claim is mounted twice: at its own path, and at the same path under the root directory. For example, `/opt/render/project/src/uploads` is also mounted at `/opt/render/project/src/backend/uploads`. So both the relative path `uploads/` and the absolute path the app had keep holding the data.
  - A path equal to the root directory or one of its parents would hide the source, so it is mounted only at its new place.
  - A path already inside `<rootDir>/` is where Render puts it, and stays.
  - A service linked to a build source follows the source's build.
  - The second mount is adopted lazily, with the next deploy (which builds the image that runs in the root directory), so the operator upgrade itself rolls no disk service. That is the pattern of m547's 1 s startup period.
  - A new service with root directory `backend` and a disk at `/opt/render/project/src/uploads` gets it there only, outside its tree, as on Render.
  - **Why not (a):** a link per top-level entry misses a directory the app makes at runtime. The image also cannot follow a remount, because `disk` is a release field and a remount does not rebuild.
  - **Why not (c):** native-init would need a writable root filesystem. It cannot help when the mount hides the tree, and it still needs the claim marker.
  - **Known limit:** a deploy that rolls the pre-change image (a rollback, or a release-only change before the next build) gets the second mount too. That is harmless when the path is outside the root directory. In the hiding case it moves the disk out of that old image's tree until the next build.
- **Measured on production's runtime (15:35Z):** pod `w6-m558-double-mount` in `bex-canary` mounted claim `w6-m558-double` (`hcloud-volumes`) at `/opt/render/project/src/uploads` and `/opt/render/project/src/backend/uploads`. `/proc/self/mountinfo` showed volume `scsi-0HC_Volume_107566087` at both paths, and a file written through one read back through the other. The pod and the claim were deleted afterwards.
- **No follow-up:** the mount-path denylist matches Render's exact list, and the dashboard's placeholder is `/var/data`. Neither names the source path.

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
