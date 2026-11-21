# w6 · m549 — Every native runtime has magic-wormhole, so disk files move in and out through Shell, as on Render

**Worker:** worker6 **Goal:** `wormhole` is installed on every native runtime image, as Render documents ("The `magic-wormhole` library is pre-installed on all Render native runtimes"), so a tenant can send a file from a disk-bearing service's Shell or SSH session to their own machine and back, the transfer path Render's disk docs give besides SCP. **Status:** done (2026-11-21)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [Choose magic-wormhole's source and pin it for every native image factory](done/t001.md) — **DONE** | 45m | — |
| t002 | [The native tools check sends and receives a file through wormhole on both architectures](done/t002.md) — **DONE** | 60m | t001 |
| t003 | [Qualify and adopt the new native image pins](done/t003.md) — **DONE** | 45m | t002 |
| t004 | [Ship and verify on production: a disk file out of and into a native service through Shell](done/t004.md) — **DONE** | 45m | t003 |
| t005 | [Record wormhole in ADR082, ADR060 and ADR018](done/t005.md) — **DONE** | 15m | t004 |
| t006 | [Simplify](done/t006.md) — **DONE** | 15m | t005 |
| t007 | [Test coverage](done/t007.md) — **DONE** | 30m | t005 |
| t008 | [Closeout](done/t008.md) — **DONE** | 10m | t006, t007 |

## Definition of done

- `wormhole --version` succeeds in the Shell of a native service on every runtime family bex offers (checked by the image qualification on amd64 and arm64, and on production for at least a Go and a Node service).
- On production, a 50 MB file sent with `wormhole send` from a disk-bearing native service's Shell in `bex-canary` is received on a workstation with a matching sha256, and a file sent the other way lands on the disk with a matching sha256.
- ADR082's file-transfer row names wormhole as the Render-parity path; SCP/SFTP stays a recorded divergence.

## Source + Goal linkage

- **Source:** `/pm-brainstorm one aspect of render parity for w6` 2026-11-20 (item 4, persistent disks), approved by the user's `/pmx them to w6`. Evidence:
  - [Render — Persistent Disks](https://render.com/docs/disks): "The `magic-wormhole` library is pre-installed on all Render native runtimes", given as the way to move files to and from a disk alongside `scp -s`.
  - No image, script or doc in the repository installs or mentions wormhole except ADR082's parity table (`docs/ADR082-persistent-disks.md:99`: "File transfer | SSH + `scp`/Magic-Wormhole | divergence … transfer via instance shell + app-level tooling").
  - Native images are built by two factories: the shared Bookworm layer `deploy/native-common/Dockerfile` (installs packages through `sh /tmp/bex-native-install.sh <pkgs>` from `deploy/native-base/install.sh`; pinned as `nativeCommonBase` in `lego/operator/internal/build/nativebases.go:33`) and the per-toolchain bases of `deploy/native-base/Dockerfile` (Bullseye and other suites, pinned per toolchain image at `nativebases.go:42+`). Checks live in `deploy/native-base/check-tools.py`, qualification in `.github/workflows/native-common-image.yml` and `.github/scripts/native-common-qualify.py`.
- **Goal linkage:** ADR008 § Render parity first. This is the one transfer path in Render's disk docs bex can offer without reopening ADR035's SCP/SFTP ban on `srv-` SSH, which `DO_NOT_DO.md` keeps.
- **Expected outcome:** a tenant moving from Render imports or exports disk data exactly as Render's docs describe.
- **Why now:** with SCP held by `DO_NOT_DO.md`, a disk service on bex has no documented way to move files at all; the native image pipelines (w8/m91–m107) are fresh, so adding one package costs the least now.
- **Render parity task omitted:** image contents only; Shell and SSH surfaces do not change.

## Unblock (2026-11-21)

- **Affected:** t004, then t008. **Source:** verified evidence, the gate's own checkable form.
- **Finding:** production pin `7e481dece7ac` (`a4db56cd9`, the newest `chore(deploy): pin platform images` on `origin/main`) carries `0f4ab445b`: `git merge-base --is-ancestor 0f4ab445b 7e481dece7ac` exits 0. A native build that starts after that roll runs on the common image `sha256:49dbbd72` with `magic-wormhole` 0.12.0.
- **Next:** t004 on production as the QA user in `bex-canary` (a paid native Go service with a disk and a paid native Node service, `wormhole --version` in each Shell, 50 MB both ways with matching sha256, fixtures deleted by id including the PV and the Hetzner volume), then t008. Render basis: [Persistent Disks](https://render.com/docs/disks), "The `magic-wormhole` library is pre-installed on all Render native runtimes".
