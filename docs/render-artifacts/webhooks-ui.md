# Render webhooks dashboard UX — live capture

Captured 2026-07-17 by walking both dashboards end to end (real create → inspect → delete on each side; probe webhooks removed afterwards). Screenshots referenced below live in `.playwright-mcp/` (gitignored, not committed): `render-webhooks-new.png`, `render-webhook-settings.png`, `bex-webhook-modal.png`, `bex-webhook-modal-overlap.png`, `bex-webhook-secret.png`. Owning milestone: `w1/m49`.

## Render `/webhooks/new` — the create page

A dedicated full page (not a modal), heading **"Create a new Webhook"**, reached from the Webhooks list's New button. Three fields, helper copy under each label:

| Field | Helper copy | Notes |
| --- | --- | --- |
| Name | "A unique name for this webhook." | placeholder `Webhook Name` |
| Endpoint URL | "The webhook sends each notification to this URL as a POST request." | placeholder `https://example.com/webhook` |
| Subscribed Events | "Choose which events in your workspace will trigger a webhook notification." | the picker below |

The **Subscribed Events picker**:

- A live counter above the tree: `0 events selected` → `8 events selected` as boxes are checked.
- A **"Search for events"** text box filtering the tree.
- A tri-state **"All events"** master checkbox: unchecked / `mixed` / checked (observed `checked=mixed` after a partial selection).
- Collapsible **groups** with an expand/collapse chevron; the group's own checkbox cascades to every child (checking **Deploy** selected all 8 deploy events at once).
- Human-readable labels ("Deploy Started", "Postgres Backup Completed"), never raw keys.
- Group inventory (~60 events total): Autoscaling (Started/Ended/Config Changed) · Cron Job Run (Started/Ended) · Deploy (Build Started/Ended, Deploy Started/Ended, Image Pull Failed, Pipeline Minutes Exhausted, Pre-Deploy Started/Ended) · Disk (Created/Updated/Deleted) · Key Value (Available/Unhealthy/Config Restart) · Maintenance (Started/Ended) · Maintenance Mode (Enabled/URI Updated) · Postgres (25 events: Created/Available/Restarted/Unavailable, Backup ×3, Cluster Leader Changed, Connection Pool ×2, Credentials ×2, Disk ×2, HA Status, PITR ×4, Read Replica ×2, Restore ×2, Upgrade ×3) · Server Status (Available/Restarted/Failed/Hardware Failure) · Suspension (Service Resumed/Suspended) · Zero Downtime Redeploy (Started/Ended) · ungrouped singles: Branch Deleted, Commit Ignored, Instance Count Changed, Instance Type Changed, Job Run Ended.

Submit is **"Create Webhook"**; success redirects straight to the detail page (no secret interstitial — see the Settings section for why).

## Render `/webhook/<whk-id>` — the detail page

Create lands here (`/webhook/whk-…`, **singular** path segment). Layout:

- **Header**: "Webhook" kicker · name + **Enabled** badge (the badge links to `settings#general`) · `Webhook ID: whk-…` with a copy button · the endpoint URL as a link with a copy button · subscribed-event chips showing 5 with a **"Show 3 more"** expander · **"Created by \<email> on \<date>"** with avatar.
- **Sidebar** (replaces the global nav while on the page): "All webhooks" back-link, then **Activity** (`/webhook/<id>/activity`) and **Settings** (`/webhook/<id>/settings`).
- **Activity** (also the default view at the bare detail URL): **"Recent deliveries"** — "Refresh the table to fetch the latest events" + a refresh button, filter tabs **All / Successful / Failed**, empty state "This webhook has no recent deliveries".

### Settings

Two sections + delete:

- **General**: **Status** toggle ("Webhooks do not send any notifications while disabled." — switch labeled "Webhook enabled") · editable **Name** · editable **Endpoint URL** · **Signing secret** — "A token used to sign and verify request payloads.", a masked `whsec_…` value with **Show secret** and **Copy** buttons. **The secret is retrievable at any time.**
- **Subscribed Events**: the same picker as the create page, prefilled, with a **"Save changes"** button (disabled until dirty).
- **Delete Webhook** at the bottom: confirmation dialog demanding the literal text **`delete webhook <name>`** typed into a "Sudo Command" field before the destructive button enables ("This action cannot be undone.").

## bex before w1/m49

- `dashboard.bex.co/webhooks/new` → **404**. Creation was a modal on `/webhooks` (`create-webhook-dialog.tsx`): Name, Destination URL, and a **flat alphabetical checkbox list of 17 raw snake_case keys** in monospace — no search, no groups, no labels, no counter, no all-events master.
- Two-screen modal flow ending in the one-time `whsec_…` secret reveal (copy button + "you won't be able to see it again").
- List rows: name/URL, raw-key event chips, Enabled switch, a **delivery-history modal**, delete behind a simple confirm. **No edit surface at all** — even though `PATCH /v1/webhooks/{id}`, GraphQL `updateWebhookEndpoint`, and MCP `update_webhook_endpoint` had all shipped in w3/m11; the backend audit (2026-07-17) found the verb store-to-adapter complete and simply never wired into the dashboard.

### The create-modal bug (blocking, found live)

`create-webhook-dialog.tsx:149` set `max-h-56` on the shadcn `ScrollArea` **root**; the component forwards caller classes to `ScrollAreaPrimitive.Root` while the Viewport is `size-full`. Inside an auto-height root a percentage height resolves to content height, so nothing constrained the list: the 17 rows rendered ~680px tall, overflowed `DialogContent` (dialog rect bottom y≈839, rows measured to y≈1209 at a 1100px viewport), and the footer's **Create button (y≈778) was hit-test-intercepted by the `maintenance_mode_enabled` row** — a real click on Create toggled that checkbox instead of submitting. Reproduced at ~800px and 1100px viewport heights; the flow was only completable via JS `el.click()`. The modal (and the bug) is deleted by w1/m49; the ScrollArea primitive misuse class is swept in `w1/m49/t007`.

## Divergences (bex vs Render), with dispositions

| Surface | Render | bex | Disposition |
| --- | --- | --- | --- |
| Signing secret | Retrievable anytime in Settings (masked, Show/Copy) | **Mint-once**: returned only by the create verb, shown on the create page's secret step | **Keep mint-once** — user decision 2026-07-17 (w1/m49/t006): narrower exposure surface; Settings shows a "shown once at creation" note instead |
| Event vocabulary | ~60 events across 11 groups | 17 events (the mechanisms bex actually emits) | Vocabulary grows with mechanisms in their owning milestones; the picker's grouping map degrades unknown keys to an "Other" group |
| Post-create landing | Straight to the detail page | In-page secret step first (consequence of mint-once), then detail | Keep — forced by the secret contract |
| `plan_changed` | Named "Instance Type Changed" | bex's plans replace instance types (w1/m8) | Label bex's `plan_changed` as "Plan Changed"; same semantic slot |
| Per-webhook sidebar | Dedicated sidebar nav on the detail page | In-page tab nav (Activity/Settings), global sidebar retained | Accepted — bex's detail pages (services, env-groups) use in-page navigation |
| Created-by identity | Email + avatar | Kratos identity UUID (what `EndpointView.createdBy` records) | Accepted for m49 (no-API-growth non-goal); an email enrichment via the owners API's Kratos lookup is possible follow-up work |

## w1/m49 verification (2026-07-17, dev-1)

Walked live against dev-1 (`bash .pm/w1/dev-1/up.sh`, dashboard on :50010 → bex-api :54010) as a freshly registered user, at a 1280×800 viewport — the height at which the old modal's Create button was unclickable:

1. `/webhooks` → **Add webhook** navigates to `/webhooks/new` (no dialog). The page shows the served 17-key vocabulary as human-labeled groups (Cron Job Run, Deploy, Maintenance Mode, Postgres, Suspension + four singles), search, tri-state All events, live counter. Checking the **Deploy** group cascaded to both children ("3 events selected" with Server Restarted).
2. **Create webhook** submitted on a real click — the old hit-test interception is gone (the modal it lived in is deleted). The mint-once secret step rendered (`whsec_…` + Copy), and **View webhook** landed on `/webhook/whk-d9dg18i9086n6btmq8ug`.
3. Detail header: kicker, name + Enabled badge, Webhook ID + copy, URL + copy, event chips, "Created by \<identity-uuid> on July 17, 2026". Activity showed the empty state, then — after creating a probe service in the workspace — a real **`deploy_started` delivery row** (Pending, attempt 1, HTTP 405 from example.com), with All/Successful/Failed filters behaving (the Pending row excluded under Failed) and Refresh present.
4. Settings: form prefilled, "All events" showed `mixed`, Save disabled until dirty; renaming + adding the Suspension group saved through `updateWebhookEndpoint` and the header/chips updated in place from the normalized cache (no refetch).
5. Delete required typing `delete webhook m49-parity-probe-renamed` exactly (button disabled on a near-miss), then landed on `/webhooks` with the endpoint gone. Probe service deleted afterwards; screenshots: `.playwright-mcp/m49-webhooks-new-page.png`, `m49-webhook-secret-step.png`, `m49-webhook-detail-activity.png`.

Cross-surface check: the dashboard's update payload sends only dirty fields over GraphQL `updateWebhookEndpoint` using that surface's `eventTypes` name, matching REST `PATCH /v1/webhooks/{id}` merge semantics with REST's `eventFilter` field and the MCP `update_webhook_endpoint` tool's `eventTypes`; the status toggle uses `setWebhookEndpointEnabled`, the same core verb REST PATCH's `enabled` maps to. No new drift introduced.
