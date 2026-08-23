# Render persistent disks capture

Captured 2026-08-23 from a live authenticated walk of the Render dashboard's **Disk** tab on a real paid web service (`srv-d2rnr3jipnbc73deuvgg`, Node runtime, Starter instance, no disk attached), cross-checked against Render's public docs (render.com/docs/disks, blueprint-spec) and API reference (api-docs.render.com), both captured 2026-08-22. This record pins the product copy and form contract that [ADR082](../ADR082-persistent-disks.md) replicates. The with-disk page state (usage graph, resize, snapshot list/restore) was **not** walked: reaching it requires creating a billable disk and taking a downtime deploy on a production service — those behaviors are pinned from the docs/API rows below instead.

## Live dashboard walk (2026-08-23)

| Concern | Render behavior (verbatim where quoted) | Evidence |
| --- | --- | --- |
| Sidebar placement | Resource-scoped sidebar, **Manage** group, entry labeled **"Disk"** (singular), URL `/web/{srv-id}/disks`, ordered between Previews and One-Off Jobs. | Live snapshot of `/web/srv-…/disks` |
| Empty state | Card headed **"Add Disk"**: "Attach an SSD to persist your service's filesystem data across deploys. Disks are charged at $0.25/GB per month. Learn more." (links to docs/disks) + an **Add Disk** button. | Live snapshot; screenshot `render-disks-empty-state.png` (session-local, `.playwright-mcp/`) |
| Price shown in product | **$0.25/GB per month** in the empty-state copy — confirms the docs/articles rate in-product. | Same empty-state card |
| Add Disk form | Inline form (replaces the card, not a modal): "Configure specifications for your disk." | Live snapshot; screenshot `render-disks-add-form.png` (session-local) |
| Warning list on the form | "Note the following:" — 1. "Attaching a disk disables zero-downtime deploys for the service." 2. "Services with an attached disk can't scale to multiple instances." 3. **"You can attach a maximum of one disk per service."** 4. "Only files under your disk's mount path are persisted." 5. "Other services can't access this service's disk." | Same form. Bullet 3 is the first place Render states the one-disk limit explicitly — the docs/Blueprint only imply it via the singular `disk` field. |
| Mount path field | Label "Mount path"; helper "The absolute mount path for the disk. Only files under this path are persisted across deploys. Cannot be the root directory (`/`)."; placeholder `/var/data`. | Same form |
| Size field | Helper "You can increase the size later, but you can't decrease it. We recommend starting with the lowest value that serves your use case." Quick-select chips **1 GB / 5 GB / 10 GB / 50 GB / 100 GB** plus a free-text box **defaulting to `10`** (GB). | Same form |
| No name field | The dashboard form has **no disk name input** — mount path + size only. `name` exists only in the Blueprint/API contract ("not currently displayed in the Render Dashboard"). | Same form; blueprint-spec |
| Form actions | Cancel / Add Disk. (Not submitted — see header note.) | Same form |
| Scaling tab interplay | On this disk-less service, Scaling shows the Autoscaling toggle + Manual Scaling slider (1–100) with **no disk mention**; the disk↔scaling constraint is surfaced on the Add Disk form (and per docs blocks scaling once a disk exists). | Live snapshot of `/web/srv-…/scaling` |

## Docs/API contract (captured 2026-08-22, pinned for ADR082)

| Concern | Render behavior | Evidence |
| --- | --- | --- |
| Eligible services | Paid web services, private services, background workers. No cron jobs, static sites, or free instance types. | render.com/docs/disks, docs/free |
| Blueprint schema | `disk: {name (required), mountPath (required), sizeGB (optional, default 10)}` on web/pserv/worker; `name`/`mountPath` mutable; `sizeGB` grow-only on sync; "You can't scale a service with an attached persistent disk." | render.com/docs/blueprint-spec |
| Mount-path denylist | Exact paths refused (subdirectories allowed): `/`, `/opt`, `/opt/render`, `/opt/render/project`, `/opt/render/project/src`, `/home`, `/home/render`, `/etc`, `/etc/secrets`. | render.com/docs/disks |
| Deploy semantics | Adding a disk triggers a deploy; thereafter deploys stop the old instance before starting the new one ("a few seconds, during which your service is unavailable"). | render.com/docs/disks |
| Build/pre-deploy | Disk not accessible during build or pre-deploy commands (separate compute), nor from one-off jobs. | render.com/docs/disks |
| Resize | Grow-only, online ("available within a few seconds"), no restart; API PATCH requires new size > current. | render.com/docs/disks, api-docs (update-disk) |
| Snapshots | Automatic daily, retained ≥7 days, full-disk restore only, restore discards post-snapshot writes and restarts the service; snapshot keys expire after 24 h; explicit warning against disk restore for database recovery; disks + snapshots encrypted at rest; no documented snapshot charge. | render.com/docs/disks, api-docs (snapshots) |
| API surface | `GET/POST /v1/disks`, `GET/PATCH/DELETE /v1/disks/{diskId}`, `GET /v1/disks/{diskId}/snapshots`, `POST /v1/disks/{diskId}/snapshots/restore`; ids `^dsk-[0-9a-z]{20}$`; delete: "All data on the disk will be lost." | api-docs.render.com |
| CLI | The official Render CLI has no disk commands (disks are dashboard/API/Blueprint only). | render.com/docs/cli-reference |
| Billing | $0.25/GB-month on **provisioned** size, prorated by the second, creation → removal. | render.com/articles (hosting-cost), render-oss/skills `render-disks` |

## bex implementation and explicit divergences

[ADR082](../ADR082-persistent-disks.md) is the design record; its § D2 behavior table maps every row above. Live-walk consequences for bex's dashboard Disks tab (ADR082 § D6):

- Mirror the form contract exactly: mount path + size only (no name input — bex auto-derives the API-level `name`), the `/var/data` placeholder, the five-bullet warning list, and the 1/5/10/50/100 GB quick-select with a 10 GB default.
- Mirror the empty-state card with bex's rate: **$0.175/GB per month** (30% off, ADR082 § D8) in place of Render's $0.25.
- Sidebar: add **"Disk"** (singular label) to the Manage group between the existing entries, replacing its current deliberate omission ([ADR018](../ADR018-render-parity.md) w1/m45 note).
- Divergences stay as ADR082 records them: grow may complete on the next restart (Hetzner CSI offline-expansion caveat), snapshots are file-level (no Hetzner volume snapshots), no SCP/SFTP transfer path (ADR035 ban stands).
