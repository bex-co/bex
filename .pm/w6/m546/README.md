# w6 · m546 — A native Go service runs from Render's Go source path, so a disk mounted where Render's docs say keeps its data

**Worker:** worker6 **Goal:** a native Go service's working directory is Render's `/opt/render/project/go/src/github.com/<owner>/<repo>`, so a disk mounted at Render's documented Go example path (`…/<repo>/data`) holds the files the app writes to `data/`, and an existing Go service's paths and mounts keep resolving. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [Record Render's Go source path and decide the layout](t001.md) | 30m | — |
| t002 | [A Go image builds and runs in Render's Go source path](t002.md) | 60m | t001 |
| t003 | [Every process of a Go service starts there: start command, pre-deploy, cron run, Shell and SSH](t003.md) | 60m | t002 |
| t004 | [Tests: a disk at Render's Go example path keeps relative writes; an existing service keeps its mounts](t004.md) | 45m | t003 |
| t005 | [Ship and verify on production](t005.md) | 45m | t004 |
| t006 | [Record the path in ADR060, ADR082 and ADR018](t006.md) | 15m | t005 |
| t007 | [Simplify](t007.md) | 15m | t006 |
| t008 | [Test coverage](t008.md) | 30m | t006 |
| t009 | [Closeout](t009.md) | 10m | t007, t008 |

## Definition of done

- On production, a native Go service created in `bex-canary` runs with `pwd` = `/opt/render/project/go/src/github.com/<owner>/<repo>` in its start command, its Shell and its SSH session.
- That service, with a disk mounted at `/opt/render/project/go/src/github.com/<owner>/<repo>/data`, keeps a file it wrote to the relative path `data/` across a code deploy.
- An existing native Go service keeps working after the change: `/opt/render/project/src` still resolves to its source, and a disk it mounted under `/opt/render/project/src/…` still holds its data after the next deploy (tested in envtest/dev-6; on production if such a service exists, recorded which).
- ADR060, ADR082 and ADR018 name the path.

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
