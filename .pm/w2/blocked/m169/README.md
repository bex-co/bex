# w2 · m169 — The forums run on bex as an ordinary tenant: uploads persist, nothing is hand-patched

**Worker:** worker2 **Goal:** the three forums (beancount.io/forum, tianpan.co/forum, forum.bex.co/forum) are deployed purely through bex's product surfaces — Blueprint, custom domains, dashboard/API/CLI, `render ssh` — exactly as an unrelated Render user could; every recoverable upload is served again, and new uploads survive restarts and redeploys. **Status:** blocked — t001–t003 done; t005 needs the user: (1) tenant-owned forum hostnames added as bex custom domains need DNS records in the user's Cloudflare, and (2) the main-site `/forum` proxies (outside bex, behind a Caddy hop) must be repointed from the hand-made `tianpan-forum.onbex.co` / `blockeden-forum.onbex.co` names; t004 deliberately rides t006's deploy window (the App rebuilds the whole Discourse image from `bex-co/discourse_docker` `main` on every push)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Rebuild the missing-upload inventory from the three forum databases and match it to the recovered files — **DONE** | 45m | — |
| t002 | Decide the mount path, size and on-disk layout from the running image — **DONE** | 30m | t001 |
| t003 | Inventory every hand-made object and settle who serves beancount.io — **DONE** | 30m | t002 |
| t004 | Move the forum's `/` → `/forum` redirect into the application | 40m | t003 |
| t005 | Serve every forum domain through bex custom domains, then delete the hand-built Ingresses and Middleware | 1h | t003, t004 |
| t006 | Make the Blueprint the single source of truth — runtime, `fromDatabase` hosts, `disk`, domains — and sync it as a tenant (**STOP: the user picks the downtime window**) | 45m | t005 |
| t007 | Restore the recovered originals the way a tenant can (`render ssh`), verifying every referenced URL | 45m | t006 |
| t008 | Prove durability: a fresh upload survives a restart and a redeploy | 30m | t007 |
| t009 | Record the unrecoverable uploads, the bex gaps found, and retire the deferred entry | 20m | t008 |
| t010 | Simplify | 15m | t009 |
| t011 | Test coverage | 30m | t009 |
| t012 | Closeout | 10m | t010, t011 |

## Definition of done

- **No hand-made objects.** Every Kubernetes object serving the forums is created and owned by bex from the App/Blueprint: none of the hand-built Ingresses (`…-beancount-forum-ms`, `…-forum-bex-co`, `…-tianpan-forum`, `…-blockeden-forum`) or the `…-forum-bex-co-root` Middleware remains, and no object carries a hand-set ownerReference.
- **Blueprint is the truth.** `discourse_docker/_infra/bex/bex-forums-multisite.yaml` matches the live service (runtime, `fromDatabase` hosts, `disk`, domains), is synced through bex's Blueprint surface, and a re-sync reports no drift.
- **Domains through the product.** Every forum hostname bex serves is a verified bex custom domain with a product-issued certificate; beancount.io is attached to whichever App t003 shows owns it; `forum.bex.co/` still redirects to `/forum`, now from the application.
- **Uploads.** Every referenced upload with recovered bytes returns 200 on its own host with a matching SHA-1; the unrecoverable ones are counted with their disposition; a fresh upload survives a pod restart and a redeploy, then is removed.
- **Gaps are filed, not bypassed.** Anything an ordinary tenant could not do is recorded as a bex gap (inbox note or QA finding) instead of being patched around in the cluster.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w2` (2026-10-19), item 2, chosen by the user the same day. Promotes the `.pm/FUTURE-MAYBE.md` entry "Forum upload restore + durable disk" (from `w7/done/042.md`, deferred 2026-09-06), whose trigger — "`nvme4tbfish` is attached" — is met: this repository runs from that volume, and `/Volumes/nvme4tbfish/discourse_docker/data/{beancount,block-eden,tianpan}-forum/uploads` hold 14, 16 and 11 files (counted 2026-10-19).
- **Goal linkage:** [ADR082](../../../../docs/ADR082-persistent-disks.md) persistent disks, applied to the exact workload class it was re-opened for ("uploads directories"), on bex's own first-party forums — including forum.bex.co, bex's community forum (w2/m168 wired its OIDC login).
- **Expected outcome:** missing forum images return, and uploads stop being lost on every restart and redeploy.
- **Why now:** the loss is ongoing, not historical. The live multisite App `tea-d98210cbbpdc73dcrkvg-beancount-forum` has no disk and no S3 store (`bex-forums-multisite.yaml` declares neither), and its pod has no volumes; the current pod started 2026-10-19T00:56Z, so everything uploaded before that restart is already gone (read-only check, 2026-10-19). Forum content operations moved to `bex-co/obm` on 2026-10-18, so the forums are actively used.
- **Direction (user decision 2026-10-20):** deploy the forums the regular way an ordinary bex.co tenant would, not as an internal service. The earlier plan to diff and re-apply hand `kubectl patch`es is withdrawn; any step a tenant cannot do through the product becomes a filed bex gap.
- **Render parity omitted:** this uses bex's existing tenant surfaces as a customer; it changes no REST/GraphQL/MCP/UI behavior itself. Gaps it uncovers are filed as their own items (with parity tasks there).
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
- **For the Blueprint task (t006):** the live App is Git-built (`spec.repo`, `spec.dockerfilePath`), not the `runtime: image` that `bex-forums-multisite.yaml` declares — reconcile that drift when declaring the disk, and diff the live Deployment for surviving manual patches before the `Recreate` deploy.

## t003 findings (2026-10-20, read-only)

| Public URL | Path today | bex object | Regular-tenant target |
| --- | --- | --- | --- |
| `beancount.io/forum` | Cloudflare → beancount.io main site (`via: 1.1 Caddy`, outside this App) → forwards `/forum` to the forum App | bex-managed Ingress `beancount-forum.onbex.co` (the App's own platform host) | already regular |
| `tianpan.co/forum` | Cloudflare → tianpan.co main site (Caddy) → `/forum` to the forum App | hand-made Ingress `tianpan-forum.onbex.co` (hand-set ownerReference) | a tenant-owned host, e.g. `forum.tianpan.co`, added as a bex custom domain; main-site proxy repointed to it |
| `blockeden.xyz/forum` | Cloudflare → blockeden.xyz main site (Caddy; `blockeden.xyz` is a verified bex custom domain on `block-eden-mono`) → `/forum` to the forum App | hand-made Ingress `blockeden-forum.onbex.co` | proxy to `forum.bex.co` (the site's canonical host) instead |
| `forum.bex.co` | Cloudflare → bex directly; `/` → 301 `/forum` | hand-made Ingress `forum-bex-co` + Middleware `forum-bex-co-root` | `forum.bex.co` as a bex custom domain on the forum App; redirect in the app (t004) |
| — | — | hand-made Ingress `beancount-forum-ms` (`beancount.io` + onbex host, no owner) | dead: `beancount.io` never reaches it (Cloudflare → main site); delete |

- A Render tenant gets exactly **one** platform hostname per service, so the extra `*-forum.onbex.co` names cannot exist the regular way; each non-default site needs a tenant-owned hostname as a custom domain. Discourse multisite picks the site by `Host` (`multisite.yml` `host_names`: tianpan → `tianpan.co`, `tianpan-forum.onbex.co`; blockeden → `forum.bex.co`, `blockeden-forum.onbex.co`, `blockeden.xyz`), so the new hostnames are added to `host_names` too.
- The bex domain store lists `beancount.io` / `www.beancount.io` as **pending** on `block-eden-mono-1` — the main-site migration, unrelated to the forums.
- The main-site proxies are not in any local repo found (the `block-eden-mono/projects/reverse-proxy-mac/Caddyfile` routes are a legacy local setup).
