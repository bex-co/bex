# w2 · m169 — Forum uploads survive restarts: restore the recovered files and attach a durable disk

**Worker:** worker2 **Goal:** every forum upload whose bytes still exist is served again on beancount.io/forum, tianpan.co/forum and forum.bex.co/forum, and a new upload survives a pod restart and a redeploy. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Rebuild the missing-upload inventory from the three forum databases and match it to the recovered files | 45m | — |
| t002 | Decide the mount path, size and on-disk layout from the running image | 30m | t001 |
| t003 | Attach the disk and declare it in the Blueprint (**STOP: needs the user's downtime authorization**) | 40m | t002 |
| t004 | Restore the recovered originals and verify every referenced URL by Host header | 45m | t003 |
| t005 | Prove durability: a fresh upload survives a restart and a redeploy | 30m | t004 |
| t006 | Record what could not be recovered and retire the deferred entry | 20m | t005 |
| t007 | Simplify | 15m | t006 |
| t008 | Test coverage | 30m | t006 |
| t009 | Closeout | 10m | t007, t008 |

## Definition of done

- Every upload referenced by the three forum databases whose bytes were recovered returns HTTP 200 on its own host, and the served bytes hash to the upload's recorded SHA-1.
- The uploads that could not be recovered are counted and listed in the private inventory (counts only in Git), with the chosen disposition recorded.
- The `beancount-forum` service (`srv-d9nqg9dcavls73fp8m2g`) has a persistent disk at the upload path, and `discourse_docker/_infra/bex/bex-forums-multisite.yaml` declares the same disk.
- A test upload made after the disk is attached still returns 200 after a pod deletion **and** after a bex redeploy; the test upload is then removed.
- All three forum hosts serve normally after the change (no manual Deployment patch silently lost).

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w2` (2026-10-19), item 2, chosen by the user the same day. Promotes the `.pm/FUTURE-MAYBE.md` entry "Forum upload restore + durable disk" (from `w7/done/042.md`, deferred 2026-09-06), whose trigger — "`nvme4tbfish` is attached" — is met: this repository runs from that volume, and `/Volumes/nvme4tbfish/discourse_docker/data/{beancount,block-eden,tianpan}-forum/uploads` hold 14, 16 and 11 files (counted 2026-10-19).
- **Goal linkage:** [ADR082](../../../docs/ADR082-persistent-disks.md) persistent disks, applied to the exact workload class it was re-opened for ("uploads directories"), on bex's own first-party forums — including forum.bex.co, bex's community forum (w2/m168 wired its OIDC login).
- **Expected outcome:** missing forum images return, and uploads stop being lost on every restart and redeploy.
- **Why now:** the loss is ongoing, not historical. The live multisite App `tea-d98210cbbpdc73dcrkvg-beancount-forum` has no disk and no S3 store (`bex-forums-multisite.yaml` declares neither), and its pod has no volumes; the current pod started 2026-10-19T00:56Z, so everything uploaded before that restart is already gone (read-only check, 2026-10-19). Forum content operations moved to `bex-co/obm` on 2026-10-18, so the forums are actively used.
- **Render parity omitted:** this operates a first-party tenant App through bex's existing disk surface; no REST/GraphQL/MCP/UI behavior changes. Any bex defect found while attaching the disk is filed separately (or fixed here with its own regression test, see t008).
- **Data handling:** forum database rows, file names and file contents are tenant data — keep inventories in a restricted local directory (mode 0700), commit only counts and dispositions.
