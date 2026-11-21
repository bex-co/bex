# w6 · m546 — A native Go service runs from Render's Go source path, so a disk mounted where Render's docs say keeps its data

**Worker:** worker6 **Goal:** a native Go service's working directory is Render's `/opt/render/project/go/src/github.com/<owner>/<repo>`, so a disk mounted at Render's documented Go example path (`…/<repo>/data`) holds the files the app writes to `data/`, and an existing Go service's paths and mounts keep resolving. **Status:** done

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [Record Render's Go source path and decide the layout](done/t001.md) — **DONE** | 30m | — |
| t002 | [A Go image builds and runs in Render's Go source path](done/t002.md) — **DONE** | 60m | t001 |
| t003 | [Every process of a Go service starts there: start command, pre-deploy, cron run, Shell and SSH](done/t003.md) — **DONE** | 60m | t002 |
| t004 | [Tests: a disk at Render's Go example path keeps relative writes; an existing service keeps its mounts](done/t004.md) — **DONE** | 45m | t003 |
| t005 | [Ship and verify on production](done/t005.md) — **DONE** | 45m | t004 |
| t006 | [Record the path in ADR060, ADR082 and ADR018](done/t006.md) — **DONE** | 15m | t005 |
| t007 | [Simplify](done/t007.md) — **DONE** | 15m | t006 |
| t008 | [Test coverage](done/t008.md) — **DONE** | 30m | t006 |
| t009 | [Closeout](done/t009.md) — **DONE** | 10m | t007, t008 |

## Definition of done

- On production, a native Go service created in `bex-canary` runs with `pwd` = `/opt/render/project/go/src/github.com/<owner>/<repo>` in its start command, its Shell and its SSH session.
- That service, with a disk mounted at `/opt/render/project/go/src/github.com/<owner>/<repo>/data`, keeps a file it wrote to the relative path `data/` across a code deploy.
- An existing native Go service keeps working after the change: `/opt/render/project/src` still resolves to its source, and a disk it mounted under `/opt/render/project/src/…` still holds its data after the next deploy (tested in envtest/dev-6; on production if such a service exists, recorded which).
- ADR060, ADR082 and ADR018 name the path.

## t001 record: Render's Go source path and the layout (2026-11-21)

- **Render's evidence, read 2026-11-20.** [render.com/docs/disks](https://render.com/docs/disks): "To mount your disk in a subdirectory of your service's source code, the absolute path depends on your runtime". Its table: "Node.js, Python, Ruby, Elixir, and Rust" → `/opt/render/project/src`, example `/opt/render/project/src/storage`; "Go" → `/opt/render/project/go/src/github.com/<owner>/<repo>`, example `/opt/render/project/go/src/github.com/<owner>/<repo>/data`; Docker → the Dockerfile's `WORKDIR`. Its reserved list is exact paths only ("You cannot mount a disk at any of these exact mount paths": `/`, `/opt`, `/opt/render`, `/opt/render/project`, `/opt/render/project/src`, `/home`, `/home/render`, `/etc`, `/etc/secrets`). [render.com/docs/monorepo-support](https://render.com/docs/monorepo-support): "Render runs commands and interacts with files relative to your service's root directory". Build errors Render users published for a root directory `backend` name `/opt/render/project/src/backend/…`, so the root directory sits under the source path. The render-oss/skills `render-disks` SKILL.md at `main` no longer repeats the table; it keeps the Node example only.
- **Layout rules (shipped in `nativeSourceDir`, `lego/operator/internal/build/native.go`):**
  1. A Go service's directory is GOPATH's `src/<host>/<owner>/<repo>`, from the service's repository URL in any accepted form (https, ssh, `git@host:path`). The host is lowercased, the owner and repository keep the URL's case (as `RENDER_GIT_REPO_SLUG` does), and `.git` goes. GitHub's form is **verified** (Render's table). The case rule is **unverified**: Render documents placeholders only.
  2. A non-GitHub host gets `<host>/<full path>`, GitLab subgroups included. **Unverified**: Render documents GitHub only.
  3. The root directory is appended, since Render runs commands from it. **Inferred** from the monorepo docs and the published `/opt/render/project/src/<rootDir>` paths, not seen for Go.
  4. A URL with no owner and repository, or any part outside letters, digits, `.`, `_` and `-` (the `WORKDIR` line expands `$` and `\`), keeps `/opt/render/project/src`. This is a bex-only fallback with no Render counterpart.
  5. Static sites keep `/opt/render/project/src`, since Render's static sites have no runtime.
  6. **Existing services:** every Go service in every workspace moves on its next build (ADR008: no allowlist). `/opt/render/project/src` becomes a link to the new directory, so old absolute paths and mounts resolve. That `/opt/render/project/src` keeps resolving on Render's Go services is **unverified**.
  7. **Denylist unchanged, no follow-up:** Render refuses exact paths only, and the Go source root is not on its list, so bex's list (`disks.go:74`, `app_types.go:1309`, `disk-section.tsx`) already matches.
- **Production, read 2026-11-21T06:20Z:** 27 Apps. One is a native Go service, `block-eden-mono-1` (`srv-dbavjsm05jqs73cjk3og`, workspace `tea-d98210cbbpdc73dcrkvg`, root directory `projects/reverse-proxy-poc`, start command `./app`), and it has no disk. Its next deploy runs in `/opt/render/project/go/src/github.com/bex-co/block-eden-mono/projects/reverse-proxy-poc`, and its relative start command is unaffected. No Go service has a disk under `/opt/render/project/src`, so the existing-service check runs on the `w6-m546-go-old` fixture (t004).
- **Seen in passing (follow-up 158):** the same rule 3 holds for every other runtime on Render, but bex runs a Node, Python, Ruby, Elixir or Rust service with a root directory from `/opt/render/project/src` itself, not `/opt/render/project/src/<rootDir>`.

## Source + Goal linkage

- **Source:** `/pm-brainstorm one aspect of render parity for w6` 2026-11-20 (item 1, persistent disks), approved by the user's `/pmx them to w6`. Evidence:
  - `lego/operator/internal/build/native.go:555` sets `nativeWorkdir = "/opt/render/project/src"` and `nativeDockerfile` (`native.go:456-470`) writes it as the `WORKDIR` of every runtime, Go included; `nativeGoPath = "/opt/render/project/go"` (`native.go:168`) is Render's GOPATH, but the source does not live under it.
  - [Render — Persistent Disks](https://render.com/docs/disks) lists each native runtime's source path; Go's is `/opt/render/project/go/src/github.com/<owner>/<repo>`, and its Go example mounts a disk at `…/<repo>/data`. [render-oss/skills render-disks SKILL.md](https://github.com/render-oss/skills/blob/main/skills/render-disks/SKILL.md) repeats the table.
  - No file in the repository mentions `go/src/github` (checked on `origin/main` `0b97bf731`), and ADR018's Persistent disks row records no divergence for it. A Go app written for Render that writes to `data/` under a disk mounted at Render's Go path writes into the image layer on bex, and loses it on every deploy.
  - Other consumers of `nativeWorkdir`: `depscache.go:107-108`, `nativePythonVenv` (`native.go:550`), the publish check (`nativestaticdeps.go:162`, `nativepublishcheck_test.go`). The mount-path denylist (`disks.go:74`, `app_types.go:1309`, `dashboard/src/features/services/components/disk-section.tsx:425`) names `/opt/render/project/src`.
- **Goal linkage:** ADR008 § Render parity first: an app migrated from Render runs unchanged, its disk included.
- **Expected outcome:** a Go service configured from Render's disk docs stops losing its data on every deploy.
- **Why now:** the failure is silent data loss, the worst outcome of the disk items; disks reopened three months ago (ADR082), so the fewer Go services exist on the old path, the cheaper the move.
- **Open decision (recommendation, not a user decision):** t001 records the proposal's recommendation: every Go service in every workspace moves (ADR008: no allowlist without a widen task), with `/opt/render/project/src` kept as a symlink to the new directory so old absolute paths and mounts resolve; non-GitHub repositories use `<host>/<owner>/<repo>` from the repository URL, which Render does not document (unverified).
- **Render parity task omitted:** the working directory is runtime mechanism; no REST, GraphQL, MCP or dashboard field changes. If t001 finds the Go source root must join the mount-path denylist (a REST/UI change), it files that as a follow-up note instead of widening this milestone.

## Unblock record (2026-11-21)

- **Affected:** w6/m546 t005, then t009.
- **Source:** verified evidence. The gate was ``pin must carry `35ac61c80` ``. The newest pin commit on `origin/main` is `b32b3ff43` (`chore(deploy): pin platform images to cce6c749f6ff`), and `git merge-base --is-ancestor 35ac61c80 cce6c749f6ff` holds. Every `bex-system` Deployment on production (`bex-controller-manager`, `bex-api`, `bex-ssh-gateway`, …) runs `ghcr.io/bex-co/bex-operator@sha256:e14f8ee487ea…`, the digest that pin commit writes.
- **Scope:** unchanged. No gate remains.
- **Next:** t005 on production as its steps say, then t009.
