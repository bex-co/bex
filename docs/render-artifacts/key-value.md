# Render Key Value dashboard capture (w5/m12/t001)

Live capture against `dashboard.render.com` (authenticated session, 2026-07-09): created a free `m12-capture-kv` Key Value instance end to end through `/new/redis`, walked its detail page while `creating` and `available`, exercised the Connect dropdown and the delete confirmation, then deleted it. Screenshots (transient, not committed) in `.playwright-mcp/`: `kv-new-redis-form.png`, `kv-detail-creating.png`, `kv-detail-available.png`.

## `/new/redis` create form

Single-page form (not a dialog — unlike bex's Databases page, which uses a dialog; note this divergence for t004), fields top to bottom:

- **Name** — text input, placeholder `example-key-value-name`.
- **Project** (optional) — project + environment pickers. Bex has no project/environment concept yet; **omit**, not a KV-specific gap.
- **Region** — radio group (bex: single region; **omit**, already a known platform-wide gap).
- **Maxmemory Policy** — combobox, default `allkeys-lru (recommended for caches)`; options: `volatile-ttl`, `noeviction`, `volatile-lru`, `volatile-lfu`, `allkeys-lfu`, `allkeys-random`, `volatile-random`. **Disabled and forced when Free plan is selected** (Free has no persistence, so the policy field still shows but is locked).
- **Persistence Mode** — combobox: `Journal + Snapshot` (default), `Snapshot only`, `Off`. Forced to `Off` (disabled) when Free is selected.
- **Instance Type** — plan cards, two groups:
  - "For hobby projects": **Free** — $0/mo, 25 MB RAM, 50 connections, no persistence. Copy: "Free instances are not backed by a persistent disk... Only one free Key Value instance can be active for any workspace."
  - "For professional use": **Starter** ($10/mo, 256 MB, 250 conn, persistence) — default selected radio, **Standard** ($32/mo, 1 GB, 1,000 conn), plus paid tiers above bex's ladder (Pro/Pro Plus/Pro Max/Pro Ultra) that bex's catalog doesn't carry — expected, bex's `tiers.Valkey` only defines `free`/`starter`/`standard` (docs/ADR021-keyvalue-management.md).
- Submit: **"Create Key Value Instance"**.

`maxmemoryPolicy` and `persistenceMode` are **not** in bex's `createKeyValue` GraphQL mutation today (`lego/backend/internal/keyvalue/graphql.go`) — per t004's instruction, omit from the form rather than fake; note as a filed gap in t006.

## Detail page ("Info" tab)

Render's KV detail is a single "Info" page (plus separate Logs/Metrics tabs, out of scope per t005) with these sections, top to bottom:

- **Header**: product eyebrow "KEY VALUE", name + plan badge (links to a `/plan` upgrade page) + "Upgrade your instance" link, "View docs", and a **Connect** dropdown button (see below). Service ID row with a copy button.
- **General**: Name (editable inline), Created (relative time), **Status** (`creating` as plain text while provisioning; a green checkmark **"Available"** badge once ready), Maxmemory Policy (disabled select showing current value + an Edit affordance), Persistence Mode (plain text), Region (plain text), Runtime (plain text, e.g. "Valkey 8.1.4").
- **Key Value Instance**: current Instance Type card (name, RAM, connection limit) + an "Update"/"Upgrade" link to the plan page. Bex supports plan changes post-create via `PATCH /v1/key-value/{id}` / GraphQL `updateKeyValuePlan` / MCP `update_key_value(plan:)` (shipped `w2/m16`; the MCP half folded into the patch tool at `w1/m74`).
- **Connections**: the on-page (not dropdown) connection info —
  - **Internal Key Value URL**: `redis://<slug>:6379` — a text field + Copy button, plus a separate **"Enable Internal Authentication"** link. **Notable: Render's internal URL has no username/password by default** — auth is opt-in. Bex's internal URL is **always** password-protected (`redis://default:<password>@<name>.<ns>.svc:6379`) — a conscious superset, already recorded in `docs/ADR018-render-parity.md`'s Key Value row. Don't change bex's posture to match; just confirm the wording of the UI doesn't claim parity it doesn't have.
  - **External Key Value URL** and **Valkey CLI Command**: when no IP is allow-listed at the instance level, both render as an info alert: "External traffic not allowed. Add IP addresses in the Networking section." — i.e. Render gates external connection info on a separate per-instance IP allowlist, not just a public/private toggle. Render's API still answers both strings for such a store (w9/134). Bex's model stays `public: bool` for whether the external endpoint is routed, with `spec.ipAllowList` (shipped w7/m5) as an additional source-CIDR gate on the public route; unlike Render, an empty allowlist on a public store means open, not blocked. Since w9/134 a private store's connection info answers both strings over the host it keeps, as Render's does, and the dashboard shows an equivalent note in their place ([ADR021 §2](../ADR021-keyvalue-management.md)).
  - The **Connect** dropdown (header button) duplicates the same internal/external strings in a tabbed popover (Internal/External tabs) rather than the inline section — a redundant presentation of the same two fields. bex's t005 only needs one Connect-panel, not both a dropdown and inline section; the m8 Databases detail's single reveal panel is the right scope.
- **Networking**: instance-level IP allowlist management — bex equivalent shipped w7/m5 (detail-page Networking section over `keyValueIpAllowList`/`setKeyValueIpAllowList`; `[]string` CIDRs, no per-entry description).
- **Log Stream**: third-party log-stream forwarding config — unrelated to this milestone, omit.
- **Danger zone** (bottom, unlabeled buttons resolved via DOM `data-test-id`): **"Delete Key Value Instance"** (destructive, red) and **"Suspend Key Value Instance"** (critical ghost). Delete opens a **type-to-confirm** dialog: "Type `sudo delete key value <name>` below to confirm," a single text input, and a Delete button that's disabled until the typed string matches exactly. Suspend uses the parallel `sudo suspend key value <name>` confirmation; Resume replaces it while suspended and runs immediately. The live bundle was reverified on 2026-07-17, and bex now mirrors these exact phrases and the shared bottom-row placement. Here `sudo` is confirmation copy, not a local privilege boundary.

## Pages: sidebar, routes and Info (w3/m649, 2026-11-25)

Read 2026-11-25 for w3/m649 from Render's public dashboard bundle, without sign-in. Not walked signed in: the bundle is the evidence, as it was for the Postgres pages ([postgres-dashboard.md](postgres-dashboard.md)).

**Evidence.** `https://dashboard.render.com/` loads `assets/index-D5V4ZgbE.js` (sha256 `8d15889ace2d…`), which holds the Key Value layout, its route table, its Info, Logs and Metrics pages and the `/plan` page. The note that filed this read `index-BPMVVQol.js` (`4eb3557f6970…`) the same day. The hashes change with every Render deploy, so find the layout by its frozen title map, `Object.freeze({"":"Info","/info":"Info","/logs":"Logs","/metrics":"Metrics","/plan":null})`, as [metrics-page.md § Status-code label](metrics-page.md#status-code-label-w3080-2026-11-20) describes the method.

### Sidebar

The layout hands the shared resource shell `project: "datastores"` and this sidebar:

| Group | Entries, in order | Gate |
| --- | --- | --- |
| top level | **Info** (`/r/<id>/info`, `hrefPattern` the bare `/r/<id>`, so also active there) | — |
| Monitor | **Logs**, **Metrics** | Logs with `viewRedisLogs`, Metrics with `viewRedisMetrics`; the group has no entries without both |

- Before the instance loads, the sidebar draws Info alone (`initialSideNav: [{ items: [info] }]`).
- The back link reads **Dashboard** (`/`), or **Environment** when the instance sits in a project environment.

### Routes

`/r/:id` renders the layout, with these children:

| Path | Page |
| --- | --- |
| `*`, `info` | Info (the bare URL shows Info with the URL unchanged, and so does any unknown sub-path) |
| `metrics`, `metrics/:period` | Metrics (without `viewRedisMetrics`, a no-permission page) |
| `logs` | Logs: the shared log explorer with `searchInUrl` and the full-screen layout |
| `plan` | a full-page plan chooser with no title; its Cancel and its Change plan both return to the bare URL |

The layout's document title is `<name> ・ Key Value` on every page. Its header carries View docs and Connect, and its Upgrade instance link is `/r/<id>/info#compute`. It passes `hideResourceHeader` from the same hook as the Postgres layout. Its banners are the suspension notice and scheduled maintenance.

### Info

In order: the title "Info", then

1. **General**: Name (editable), Created (Creating since, while creating), Status, **Maxmemory Policy** (editable), **Persistence Mode** (editable; a free instance reads Off), Region, Runtime.
2. **Compute** (`#compute`): the plan cards with Edit, then "Review your compute selection" and Upgrade compute.
3. **Connections**: Internal Key Value URL (with Enable Internal Authentication), External Key Value URL and the Valkey CLI Command; without an IP allow list both read "External traffic not allowed. Add IP addresses in the Networking section.", linking `#networking`.
4. **Networking**: the IP allow list.
5. A log stream override (log streams are a [`DO_NOT_DO.md`](../../.pm/DO_NOT_DO.md) non-goal).
6. The delete card: Delete Key Value Instance, with Suspend beside it.

**Logs** is the shared log viewer. **Metrics** is the metric groups, with an upgrade link to `/plan` where a free instance's disk chart has no data.

### Page map (bex, w3/m649)

Every card of bex's former single Key Value page goes to Info, in the order of its Render counterpart. Render's General holds the Maxmemory Policy and Persistence Mode fields; bex keeps them as their own cards, with their confirmations, right after Details.

| bex card (old anchor) | Page, place | Render counterpart |
| --- | --- | --- |
| Details (`#metadata`) | Info, 1st | General |
| Maxmemory policy (`#maxmemory-policy`) | Info, 2nd | General's Maxmemory Policy |
| Persistence mode (`#persistence-mode`) | Info, 3rd | General's Persistence Mode |
| Plan (`#plan`) | Info, 4th | Compute (`#compute`) |
| Connections (`#connection`) | Info, 5th | Connections |
| Networking (`#networking`) | Info, 6th | Networking |
| Danger zone (`#danger-zone`): delete, suspend | Info, 7th | the delete card |
| the Logs tab (`?tab=logs`) | Logs | Logs |
| the Metrics tab (`?tab=metrics`) | Metrics | Metrics |

### What bex does (w3/m649)

- A Key Value instance's pages swap the workspace nav for this sidebar: the back link (Dashboard), the name, Info / Monitor: Logs, Metrics. A Viewer or a Billing member (denied `can_view_logs`) gets no Logs, a Billing member (denied `can_view_activity`) no Metrics, and an emptied group goes. Unlike Render's, the loading sidebar draws all three entries, so nothing shifts once the instance resolves.
- `/keyvalue/<id>` and `/keyvalue/<id>/info` show Info; `/logs` and `/metrics` show their pages, each with its sidebar label as its heading and a document title led by it ("Logs ・ sessions-cache ・ Key Value ・ bex Dashboard"). Render's `/r/<id>/<page>` reaches them through the `/r/` alias, query and hash kept, and bex-api's `dashboardUrl` (`/r/<id>`) opens Info.
- `/plan` lands on Info's plan card (`/keyvalue/<id>#plan`), `#compute` scrolls to that card, `/metrics/<period>` lands on `/metrics`, and any other sub-path on Info.
- The old single page's links land on their page: `?tab=logs` on `/logs` and `?tab=metrics` on `/metrics`, with their range and filter keys; each section anchor stays on Info, its card scrolled into view.

### Still apart

- Render's `/plan` is a full-page plan chooser; bex's plan picker is the plan card on Info, where `/plan` lands ([w3/127](../../.pm/w3/127.md)).
- Render's Logs and Metrics pages hide the resource header; bex keeps it, as on a Postgres database's pages ([w3/128](../../.pm/w3/128.md), after [w3/117](../../.pm/w3/117.md) for Postgres).
- Render folds Maxmemory Policy and Persistence Mode into General; bex keeps them as cards of their own beside it.
- Render's document title has no page name; bex's names the page first, as its service tabs do.

## Plan review (w3/m674, 2026-11-26)

Read from Render's public dashboard bundle `index-D5V4ZgbE.js` (sha256 `8d15889ace2d…`), without sign-in. Info's Compute section opens Render's one review component ([postgres-dashboard.md § Plan review](postgres-dashboard.md#plan-review-w3m674)) on Save:

- Lead "Review your compute selection."; one line, **Compute plan**: the new plan's CPU, RAM and connection limit, Edit "Edit compute plan" back to the picker, and its monthly price. Total is the plan's price: Render lists no Key Value storage line.
- Warning "Your Key Value instance will be unavailable while updating.", followed by " Free instances have no persistence, so all existing data will be deleted." when the instance has no disk (`planSpec.diskGiB === 0`, a Free instance).
- Button **Upgrade compute**. Smaller plans are disabled in the picker ("Currently, we don't support downgrading Key Value instances.").

### What bex does (w3/m674)

- Info's plan card opens `ReviewChangesDialog` with Render's lead, the Compute plan line (CPU, RAM, connections and the catalogue's `monthlyUsd`), the Total, Render's warning and Upgrade compute. A refused change shows its error in the dialog and keeps it open.
- The data-loss sentence follows bex's condition, not Render's: bex's Free tier has a disk and persistence ([ADR021](../ADR021-keyvalue-management.md), w6/m127), so a plan change deletes data only when Persistence is Off (or unread), whatever the plan. Then the warning adds bex's sentence ("Persistence is Off, so the restart deletes every key in ‹name›. This cannot be undone.") and Upgrade compute waits for `sudo clear key value <name>`, as every bex confirmation that deletes data does (dashboard AGENTS.md § Destructive confirmations, w4/m263). A persisted Free instance keeps its data, so its review has Render's warning alone.

## Where Render shows the connection limit (w6/m459 t001, 2026-11-17)

Read from Render's docs and their screenshots on 2026-11-17; no live Render session was available (the repository holds no Render dashboard login), so the 2026-07-09 capture above was not re-walked.

| Render surface | Shows the limit? | Source |
| --- | --- | --- |
| `/new/redis` Instance Type cards | Yes: a line of its own under the RAM, "50 Connections", "250 Connections", "1,000 Connections", "5,000 Connections", "10,000 Connections" | render.com/docs/key-value step "select a compute plan. This determines its available RAM and connection limit", screenshot `valkey-instance-types.png` ("Selecting a Valkey compute plan"); the 2026-07-09 capture above |
| Info page, Key Value Instance card | Yes: name, RAM, connection limit | the 2026-07-09 capture above |
| The plan page (Update/Upgrade) | Yes: the same cards | render.com/docs/key-value § Changing compute plans: "a larger compute plan with more RAM and a higher connection limit" |
| Metrics page, Active Connections chart | Yes: "Limit 250" beside the title | render.com/docs/key-value § Metrics, screenshot `kv-active-connections.png` ("Key Value active connections metric") |
| REST, MCP, Blueprint | No plan catalog at all | render.com/docs/compute-plans lists the limits; the API has no catalog endpoint |

bex shows it in the same four dashboard places since w6/m459, read from GraphQL `keyValueInstanceTypes.connectionLimit` (lego/types/tiers, Render's figures since w6/m456): each plan card's own line, the Details card's Connection limit row beside Instance type (bex's Info page carries the instance facts in its Details card), the plan picker's cards, and "Limit N" beside the Metrics page's Active Connections chart (a tab until w3/m649). Postgres is in [postgres-connection-limit.md](postgres-connection-limit.md).

## No live "list" page — unified Services table

Render's dashboard **no longer has a dedicated Postgres/Key Value list page** in the current UI; all resource types (web services, static sites, Postgres, Key Value, …) live in one "Ungrouped Services" / project-scoped table with columns **Status, Runtime, Region, Updated** (Runtime shows e.g. "Valkey 8" the same way it shows "Static" for a static site) and active/suspended/all tabs. There is no separate top-level "Key Value" or "Databases" nav item — everything is reached through the unified services table.

This is a real change from what informed bex's own Databases page (w5/m8), which — lacking a live Render session at the time — built a **dedicated** Databases nav entry + list page as an intentional simplification, not a byte-for-byte copy of Render's current unified table. m12's task spec (t003) explicitly directs a dedicated "Key Value" sidebar entry "after Databases," i.e. continuing bex's own established IA rather than chasing Render's latest unified-table redesign. **Recommendation: keep the divergence intentional and documented** (already true of m8) — build the dedicated `/keyvalue` list page per the milestone spec, and note in t006 that bex's per-resource-type list pages are a deliberate simplification versus Render's current single unified table, not an oversight.

## Field → bex GraphQL mapping (for t002/t004/t005)

| Render UI field | bex GraphQL field | Notes |
| --- | --- | --- |
| Name | `KeyValue.name` / `id` | name-as-id |
| Instance Type / plan badge | `KeyValue.plan` | `free`/`starter`/`standard` only |
| Status badge | `KeyValue.status` | `available`/`creating`/`unavailable` |
| Runtime ("Valkey 8.1.4") | `KeyValue.version` | bex's is optional, may be coarser |
| Created | `KeyValue.createdAt` |  |
| Internal Key Value URL | `KeyValueConnectionInfo.internalConnectionString` | always has embedded password in bex (superset) |
| External Key Value URL | `KeyValueConnectionInfo.externalConnectionString` | answered for every store; shown only when `KeyValue.public` (w9/134) |
| Valkey CLI Command | `KeyValueConnectionInfo.cliCommand` | ` REDISCLI_AUTH=… valkey-cli … --tls` (w9/m521); shown only when `KeyValue.public` (w9/134) |
| Maxmemory Policy, Persistence Mode, Project/Environment, Region, IP allowlist | — | not in bex's contract; omit from create form + detail, don't fake |

## Delete confirmation copy

Render: `Type sudo delete key value <name> below to confirm.` Bex mirrors the exact phrase so the destructive flow and button labels match the reference page; the command-like text is only a confirmation token.
