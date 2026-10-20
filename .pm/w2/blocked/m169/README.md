# w2 · m169 — Forum uploads survive restarts: restore the recovered files and attach a durable disk

**Worker:** worker2 **Goal:** every forum upload whose bytes still exist is served again on beancount.io/forum, tianpan.co/forum and forum.bex.co/forum, and a new upload survives a pod restart and a redeploy. **Status:** blocked — t001–t002 done 2026-10-19; t003 (attach the disk) needs the user to authorize a brief downtime window for all three forums

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Rebuild the missing-upload inventory from the three forum databases and match it to the recovered files — **DONE** | 45m | — |
| t002 | Decide the mount path, size and on-disk layout from the running image — **DONE** | 30m | t001 |
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
- **Goal linkage:** [ADR082](../../../../docs/ADR082-persistent-disks.md) persistent disks, applied to the exact workload class it was re-opened for ("uploads directories"), on bex's own first-party forums — including forum.bex.co, bex's community forum (w2/m168 wired its OIDC login).
- **Expected outcome:** missing forum images return, and uploads stop being lost on every restart and redeploy.
- **Why now:** the loss is ongoing, not historical. The live multisite App `tea-d98210cbbpdc73dcrkvg-beancount-forum` has no disk and no S3 store (`bex-forums-multisite.yaml` declares neither), and its pod has no volumes; the current pod started 2026-10-19T00:56Z, so everything uploaded before that restart is already gone (read-only check, 2026-10-19). Forum content operations moved to `bex-co/obm` on 2026-10-18, so the forums are actively used.
- **Render parity omitted:** this operates a first-party tenant App through bex's existing disk surface; no REST/GraphQL/MCP/UI behavior changes. Any bex defect found while attaching the disk is filed separately (or fixed here with its own regression test, see t008).
- **Data handling:** forum database rows, file names and file contents are tenant data — keep inventories in a restricted local directory (mode 0700), commit only counts and dispositions.

## Evidence

**t001 inventory (2026-10-19, read-only).** Rebuilt from each forum database's `uploads` table plus `upload_references`; the private per-upload list is in `~/.local/share/bex/backups/forum-uploads-restore-2026-10/` (mode 0700, not in Git). System uploads (negative ids, 6 per site) excluded.

| site | referenced local uploads | served now | recoverable (SHA-1 match on `nvme4tbfish`) | unrecoverable |
| --- | --- | --- | --- | --- |
| beancount | 19 | 0 | 11 | 8 (created 2026-10-05 ×6, 2026-10-12 ×2) |
| tianpan | 9 | 0 | 8 | 1 (2026-10-05) |
| blockeden | 11 | 0 | 10 | 1 (2026-10-08) |
| **total** | **39** | **0** | **29** (21 unique objects) | **10** |

- Every referenced upload is currently missing. The ten unrecoverable ones were all uploaded **after** the move to bex and lost on later pod restarts — the ongoing loss this milestone stops.
- Probing method: the public hosts are unreliable for this (beancount.io's own frontend answers `/forum/uploads/...` itself through Cloudflare), so URLs were probed on the direct `*-forum.onbex.co` hosts. Discourse answers **500** for an upload whose row exists but whose file is absent, and 404 for an unknown file.

## Decisions

**t002 (2026-10-19, read-only `kubectl exec`).**

- **`mountPath: /var/www/discourse/public/uploads`** — Discourse's local store root (`public/uploads/<db>/…`). It is a real directory (not a symlink), `root:root 755`, empty; `/var/discourse/shared/uploads` is a separate, unused empty directory.
- **`sizeGB: 10`** (Blueprint default; current need ≈3.3 MB).
- **Ownership:** puma and sidekiq run as UID 0 with `CapEff: 0` (`capabilities.drop: [ALL]`, no `DAC_OVERRIDE`), so they can write only where root is the owner. A fresh volume root is `root:root`, so no `fsGroup` is needed. Revisit if `w6/m149`'s privilege policy changes the runtime user.
- **Target paths:** all 29 recoverable uploads are legacy single-site rows under `/forum/uploads/default/original/1X/<sha1>.<ext>` → `public/uploads/default/original/1X/`. Post-consolidation rows use per-site prefixes (`uploads/tianpan/…`, `uploads/blockeden/…`); the two such rows are among the unrecoverable ten.
- **For t003:** the live App is Git-built (`spec.repo`, `spec.dockerfilePath`), not the `runtime: image` that `bex-forums-multisite.yaml` declares — reconcile that drift when declaring the disk, and diff the live Deployment for surviving manual patches before the `Recreate` deploy.
