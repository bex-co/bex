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
  - **External Key Value URL** and **Valkey CLI Command**: when no IP is allow-listed at the instance level, both render as an info alert: "External traffic not allowed. Add IP addresses in the Networking section." — i.e. Render gates external connection info on a separate per-instance IP allowlist, not just a public/private toggle. Bex's model stays `public: bool` for whether the external endpoint exists — `externalConnectionString`/`cliCommand` simply absent when not public — with `spec.ipAllowList` (shipped w7/m5) as an additional source-CIDR gate on the public route; unlike Render, an empty allowlist on a public store means open, not blocked.
  - The **Connect** dropdown (header button) duplicates the same internal/external strings in a tabbed popover (Internal/External tabs) rather than the inline section — a redundant presentation of the same two fields. bex's t005 only needs one Connect-panel, not both a dropdown and inline section; the m8 Databases detail's single reveal panel is the right scope.
- **Networking**: instance-level IP allowlist management — bex equivalent shipped w7/m5 (detail-page Networking section over `keyValueIpAllowList`/`setKeyValueIpAllowList`; `[]string` CIDRs, no per-entry description).
- **Log Stream**: third-party log-stream forwarding config — unrelated to this milestone, omit.
- **Danger zone** (bottom, unlabeled buttons resolved via DOM `data-test-id`): **"Delete Key Value Instance"** (destructive, red) and **"Suspend Key Value Instance"** (critical ghost). Delete opens a **type-to-confirm** dialog: "Type `sudo delete key value <name>` below to confirm," a single text input, and a Delete button that's disabled until the typed string matches exactly. Suspend uses the parallel `sudo suspend key value <name>` confirmation; Resume replaces it while suspended and runs immediately. The live bundle was reverified on 2026-07-17, and bex now mirrors these exact phrases and the shared bottom-row placement. Here `sudo` is confirmation copy, not a local privilege boundary.

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
| External Key Value URL | `KeyValueConnectionInfo.externalConnectionString` | present only when `KeyValue.public` |
| Valkey CLI Command | `KeyValueConnectionInfo.cliCommand` | `redis-cli -u <uri>` |
| Maxmemory Policy, Persistence Mode, Project/Environment, Region, IP allowlist | — | not in bex's contract; omit from create form + detail, don't fake |

## Delete confirmation copy

Render: `Type sudo delete key value <name> below to confirm.` Bex mirrors the exact phrase so the destructive flow and button labels match the reference page; the command-like text is only a confirmation token.
