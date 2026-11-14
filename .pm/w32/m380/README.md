# w32 · m380 — A directory copied into or out of a sandbox keeps its symlinks and hard links, as on Render

**Worker:** worker32 **Goal:** `bex sandboxes copy` (and the Render SDK's directory upload and download) transfers a directory that contains symbolic links or hard links. Links arrive as links, sockets and FIFOs are skipped, and a download that cannot be served answers its real cause instead of `409 file ticket already used` or `503 sandbox file gateway unavailable`. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | An uploaded directory archive keeps its symlinks (and hard links to its own files); special entries are skipped | 1.5h | — |
| t002 | A downloaded directory archive carries the sandbox's symlinks and hard links; special entries are skipped | 1h | t001 |
| t003 | A download that fails before its first byte answers its cause; bex-api never replays a file transfer to the gateway | 1h | — |
| t004 | Blast radius: agent-session attachment and skill uploads, the destination-path symlink rule, limits, and the docs that grade copy | 30m | t001, t002, t003 |
| t005 | Production walk with the release CLI | 45m | t004 |
| t006 | Render parity | 20m | t005 |
| t007 | Simplify | 15m | t006 |
| t008 | Test coverage | 30m | t006 |
| t009 | Closeout | 10m | t007, t008 |

## Definition of done

On production, with the released `bex` CLI against a new sandbox in `bex-canary`:

- `bex sandboxes copy ./src <sbx>:up` succeeds for a local directory holding a relative symlink (`link -> with space.txt`), an absolute one (`abs-link -> /etc/hostname`), a dangling one and a symlink to a directory. In the sandbox, `ls -la ~/up` shows each as a link with the same target. A local Python virtualenv (`python3 -m venv --without-pip venv`, which links `bin/python -> python3` and `lib64 -> lib`) uploads and keeps its links.
- In the sandbox, a directory holding `top.txt`, `rel-link -> top.txt`, `abs-link -> /etc/hostname`, `dirlink -> sub` and `hard.txt` (a second hard link to `top.txt`) downloads with `bex sandboxes copy <sbx>:dl ./dl`. Locally the symlinks read back their targets, and `hard.txt` has the same content as `top.txt`; the pinned client restores it as a link.
- A download that is refused before any byte is sent answers one HTTP status naming the cause, on HTTP/1.1 and HTTP/2. A first attempt never comes back as `409 file ticket already used` or `503 sandbox file gateway unavailable`.
- A control still holds: a plain tree (spaces, CJK and emoji names, a hidden file, an empty directory, `0755` and `0444` files) round-trips as on 2026-11-13. An archive entry that would escape its destination, or a child placed under a symlink entry, is still refused.
- `docs/cli-compatibility-checklist.md`, `docs/render-artifacts/ea-sandbox.md` and ADR018's sandbox table describe the new archive rule.

## Source + Goal linkage

- **Source:** `/qa-find-bugs-cli` loop sweep 6c54, 2026-11-13, in `bex-canary` (`tea-daif693dqjvc73e7as3g`). It used release `bex` v0.5.0 (Render CLI v2.35.0 pin) and the HEAD build `head91f543f` (Render CLI v2.36.0, pin `4fd34231b637`) against `https://api.bex.co/v1/`, with a human device login, on production pin `5de3d5bcce8e`. The evidence is below.
- **Goal linkage:** pillar 5 sandboxes and [ADR008 § Render parity first](../../../docs/ADR008-vision.md#render-parity-first). Copying a project in and its results out is the basic sandbox loop. Project trees routinely contain links: `node_modules/.bin`, Python virtualenvs, git checkouts with symlinked files, and hard-linked package stores.
- **Expected outcome:** any directory a user or agent copies in or out arrives with its links, as Render documents. A refused download names its cause.
- **Why now:** today one link anywhere in a tree fails the whole transfer in either direction. The download failure masquerades as a spent ticket or an outage, so users cannot tell what to fix. w7/m150 recorded "links and special archive entries are refused" as a bound of its first implementation. No ADR018 row signs it off as a divergence.
- **Render parity:** included. The change alters REST's connect-token file routes, which the CLI and SDK use. GraphQL and MCP have no file transfer, and the dashboard has no sandbox file surface.

## Evidence (sweep 6c54, 2026-11-13 ~13:46–13:52 UTC)

**Render's contract:**

- render.com/docs/sandboxes-files (fetched 2026-11-13), for the Python SDK's `copy_to`: "Symlinks are preserved, and sockets, FIFOs, and other special files are skipped." Also: "Downloading a directory returns its contents as a tar archive."
- The pinned client agrees.
  - `pkg/sandbox/files.go` `writeTar` streams "the directory tree … mirroring the sandbox agent's semantics: entry names are relative to root … and symlinks are stored, not followed". It skips sockets and other special files.
  - `extractTar` restores `tar.TypeSymlink` entries and `tar.TypeLink` entries. Its comment on the latter: "The sandbox's tar emits these for multiply-linked files".
  - `TestTarRoundTrip` round-trips a symlink.

**bex on production** (sandbox `sbx-dbrh980k58is73auo4s0`, since terminated):

```text
# upload: a plain tree works; one symlink fails the whole tree (release and HEAD CLI alike)
$ bex sandboxes copy ./plain <sbx>:up-plain     # spaces, 中文名.txt, emoji, .hidden, empty/, 0755 run.sh
Uploaded ./plain to sbx-dbrh980k58is73auo4s0:up-plain
$ bex sandboxes copy ./symonly <sbx>:up-symonly # a.txt + l -> a.txt
Error: received response code 400: invalid or unsupported sandbox archive: links and special entries are unsupported
$ bex sandboxes copy ./venv19 <sbx>:venv19      # python3 -m venv --without-pip: bin/python -> python3, lib64 -> lib
Error: received response code 400: invalid or unsupported sandbox archive: links and special entries are unsupported
# (a local hard-link pair uploads, as two separate files: Go's tar.FileInfoHeader writes both as regular files)

# download: every link kind fails, reported as a spent ticket or an outage
$ bex sandboxes exec <sbx> -- bash -c 'cd ~ && mkdir v-sym v-hard v-abs v-dirlink …; ln -s a v-sym/l; ln v-hard/b v-hard/h; …'
$ bex sandboxes copy <sbx>:v-sym ./v-sym        # relative symlink
Error: received response code 409: file ticket already used
$ bex sandboxes copy <sbx>:v-hard ./v-hard      # hard link
Error: received response code 409: file ticket already used
$ bex sandboxes copy <sbx>:v-abs ./v-abs        # absolute symlink; v-dirlink (link to a directory) the same
Error: received response code 409: file ticket already used
$ bex sandboxes copy <sbx>:mid ./mid            # 6 × 3 MB files + one symlink
Error: received response code 503: sandbox file gateway unavailable
$ bex sandboxes copy <sbx>:up-plain ./x         # control: no links
Downloaded sbx-dbrh980k58is73auo4s0:up-plain to ./x
```

Rechecked on pin `1c4d721bc8dc` (2026-11-13 16:45 UTC, `sbx-dbrjt7tgh3vs73b14cmg`): a symlink upload still answers the same `400`, and a symlink download answers `503 sandbox file gateway unavailable`.

Rechecked on pin `3e30e291ed3c` (2026-11-13 23:15 UTC, `sbx-dbrpjtruoa8c73enef20`). That pin carries w8/313's change to the gateway's upload path. Both bugs are unchanged: a symlink upload answers the same `400`. A symlink download answers `409 file ticket already used` twice through the CLI, and once each over HTTP/2 and HTTP/1.1 through the single-request probe.

One request per ticket, using a download ticket minted as the pinned client does. The probe was private and printed neither the token nor the URI query. curl made no retry:

```text
POST /v1/sandboxes/<sbx>/files/download/token?ownerId=tea-…&path=v-sym  → 201 {executionId, expiresAt, method: GET, token: <redacted>, uri: https://api.bex.co/v1/sandboxes/<sbx>/files/download/exe-…?<redacted>}
GET  <uri>  (HTTP/2)   → 409 {"error":"file ticket already used","id":"conflict","message":"file ticket already used"}
GET  <uri>  (HTTP/1.1) → 503 {"error":"sandbox file gateway unavailable","id":"unavailable","message":"sandbox file gateway unavailable"}
GET  <uri for up-hardonly, no links> (HTTP/1.1) → 200 Content-Type: application/x-tar, Content-Encoding: gzip
```

**Root cause (traced at `ae142a062`; line numbers rechecked at `7d19803fd`, after w8/313 changed the gateway):**

- **Links are refused.** `copyArchive` (`lego/backend/internal/sshgateway/sandboxfiles/sandboxfiles.go:668`) vets every directory archive in both directions:
  - an upload before it reaches the sandbox's `tar -xf` (`upload`, `:458`; `stageScript`, `:336-354`);
  - a download as it relays the sandbox's `tar -cf - -C "$name" .` (`downloadScript`, `:492-505`; `download`, `:550`).
  - It returns `links and special entries are unsupported` for anything but `tar.TypeReg` and `tar.TypeDir` (`:691-693`).
- **The download's cause is lost.**
  - `download` sets its headers, then streams through a buffering `gzip.Writer`. On any copy error it panics with `http.ErrAbortHandler` (`:568-574`), even when no byte has reached the client yet.
  - bex-api reverse-proxies the transfer with the default transport (`lego/backend/internal/sandbox/file_connect.go:266-287`). When the gateway drops a reused connection before answering, Go's transport replays the GET, which carries no body.
  - The gateway has already consumed the ticket's nonce (`sandboxfiles.go:107-109`), so the replay answers `409 file ticket already used`. On a fresh connection, the proxy's `ErrorHandler` (`file_connect.go:281-283`) answers `503 sandbox file gateway unavailable`.
- **History:** the rule dates from the original copy path (`c201b1203`, w7/m150), whose README says "The server refuses links entirely". The checklist row `sandboxes copy` and `ea-sandbox.md` repeat it. ADR018 has no row for it.

**Unverified:**

- A link that is archived after enough data has streamed would fail mid-response. The client would see a truncated gzip stream with part of the tree extracted. That is traced from `download`, not observed: in each test the sandbox's tar archived the link first.
- Render's own handling of hard links on upload. The Render docs quote covers symlinks and special files only. Python's `tarfile` records a second link to an already archived inode as a hard link, so an SDK upload may carry them.
- Whether Render keeps a symlink that points outside the destination. The pinned client creates one on download and refuses later writes through it.
