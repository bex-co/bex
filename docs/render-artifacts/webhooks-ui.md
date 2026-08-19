# Render webhooks dashboard UX — live capture

Captured 2026-07-17 by walking both dashboards end to end (real create → inspect → delete on each side; probe webhooks removed afterwards). Screenshots referenced below live in `.playwright-mcp/` (gitignored, not committed): `render-webhooks-new.png`, `render-webhook-settings.png`, `bex-webhook-modal.png`, `bex-webhook-modal-overlap.png`, `bex-webhook-secret.png`. Owning milestone: `w1/m49`.

> **2026-08-16 contract refresh and m70 result:** The API/wire comparison and current 67-value OpenAPI enum are pinned in [webhooks-api.md](webhooks-api.md). Render now publishes webhook CRUD and delivery-history endpoints. m70 aligned bex's supported non-secret REST fields/envelopes and refreshed the dashboard over that contract. This earlier walk remains the visual reference, but its ~60-event catalog and any API-absence inference must not be treated as the current wire contract.

> **2026-08-17 attempt-history refresh (`w8/m25`):** Render's current official [webhook guide](https://render.com/docs/webhooks) still pins stable event/body identity across automatic retries with fresh send timestamps/signatures. Render's official [Recent deliveries walkthrough](https://render.com/blog/light-up-your-builds-with-render-webhooks) explicitly describes the dashboard table as every attempt, with expandable request JSON and endpoint response plus a Resend action. Together with the 2026-07-17 live walk below, this is the current UI contract used by m25. It is a dated official-source comparison, not a claim that the m25 verifier was run live against Render or bex on this date.

> **2026-08-17 management refresh (`w8/m26`):** A new authenticated walk measured Render's served picker at exactly 64 normalized values, separate from its 67-value OpenAPI enum and 62-value prose catalog. The complete sets are pinned in the [machine-readable fixture](fixtures/render-webhook-vocabulary-2026-08-17.json). OpenAPI-only is exactly artifact fetch/source failures, three edge-cache events, and `plan_changed`; picker-only is `instance_type_changed` plus the two Postgres credential events. The same walk captured inline required/URL/name-conflict feedback, endpoint search, latest-delivery state, visible event-catalog loading, and manager-only mutation controls. These are the m26 acceptance contract; unsupported vocabulary remains dispositioned rather than added for count parity.

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
| Event vocabulary | 67 OpenAPI values (the dashboard/prose grouping differs slightly) | 32 truthful values: 29 API overlaps plus `branch_changed` and two prose-documented Postgres credential events | Remaining provider/anti-goal/source-bound families are enumerated in [webhooks-api.md](webhooks-api.md); the picker degrades an unknown future key to "Other" rather than hiding it |
| Post-create landing | Straight to the detail page | In-page secret step first (consequence of mint-once), then detail | Keep — forced by the secret contract |
| `plan_changed` | Named "Instance Type Changed" | bex's plans replace instance types (w1/m8) | Label bex's `plan_changed` as "Plan Changed"; same semantic slot |
| Per-webhook sidebar | Dedicated sidebar nav on the detail page | In-page tab nav (Activity/Settings), global sidebar retained | Accepted — bex's detail pages (services, env-groups) use in-page navigation |
| Created-by identity | Email + avatar | Stored subject resolved to the authorized workspace member's email; no avatar field exists in the current member contract | Email gap closed by m70; avatar remains a small presentation divergence rather than exposing the raw subject when resolution succeeds |

## m70 dashboard verification (2026-08-16)

The refreshed dashboard consumes the corrected GraphQL dialect over the same core semantics:

- Create requires a name and selection, can start disabled, shows coded server validation/conflict detail, and compacts a full picker selection to the empty future-inclusive all-events representation.
- List, detail chips, and Activity use translated human labels; an empty stored filter reloads as **All events**, not an empty subscription. The endpoint opens as an external link and the enabled badge links to Settings.
- Detail resolves the creator subject through the workspace member query, falling back safely if the identity provider has no match.
- Activity presents the stable first-attempt time, HTTP status or transport error, and expandable bounded response/error evidence. Successful/failed and timestamp predicates execute on the server, so each explicit page contains matching history without a browser-side history crawl.
- Settings validates a non-empty name and HTTPS URL client-side, preserves sparse update semantics, and saves All events as `eventTypes: []`.

Focused component/hook coverage contains 30 interaction assertions across the picker, create hook, header, Settings, and Activity; the repository locale-parity test enforces matching English/Chinese keys. The accepted layout and mint-once divergences below remain unchanged.

## w1/m49 verification (2026-07-17, dev-1)

Walked live against dev-1 (`bash .pm/w1/dev-1/up.sh` — now `bash scripts/dev-env.sh 1 up`, w1/m72; dashboard on :50010 → bex-api :54010) as a freshly registered user, at a 1280×800 viewport — the height at which the old modal's Create button was unclickable:

1. `/webhooks` → **Add webhook** navigates to `/webhooks/new` (no dialog). The page shows the served 17-key vocabulary as human-labeled groups (Cron Job Run, Deploy, Maintenance Mode, Postgres, Suspension + four singles), search, tri-state All events, live counter. Checking the **Deploy** group cascaded to both children ("3 events selected" with Server Restarted).
2. **Create webhook** submitted on a real click — the old hit-test interception is gone (the modal it lived in is deleted). The mint-once secret step rendered (`whsec_…` + Copy), and **View webhook** landed on `/webhook/whk-d9dg18i9086n6btmq8ug`.
3. Detail header: kicker, name + Enabled badge, Webhook ID + copy, URL + copy, event chips, "Created by \<identity-uuid> on July 17, 2026". Activity showed the empty state, then — after creating a probe service in the workspace — a real **`deploy_started` delivery row** (Pending, attempt 1, HTTP 405 from example.com), with All/Successful/Failed filters behaving (the Pending row excluded under Failed) and Refresh present.
4. Settings: form prefilled, "All events" showed `mixed`, Save disabled until dirty; renaming + adding the Suspension group saved through `updateWebhookEndpoint` and the header/chips updated in place from the normalized cache (no refetch).
5. Delete required typing `delete webhook m49-parity-probe-renamed` exactly (button disabled on a near-miss), then landed on `/webhooks` with the endpoint gone. Probe service deleted afterwards; screenshots: `.playwright-mcp/m49-webhooks-new-page.png`, `m49-webhook-secret-step.png`, `m49-webhook-detail-activity.png`.

Cross-surface check: the dashboard's update payload sends only dirty fields over GraphQL `updateWebhookEndpoint` using that surface's `eventTypes` name, matching REST `PATCH /v1/webhooks/{id}` merge semantics with REST's `eventFilter` field and the MCP `update_webhook_endpoint` tool's `eventTypes`; the status toggle uses `setWebhookEndpointEnabled`, the same core verb REST PATCH's `enabled` maps to. No new drift introduced.

## m25 attempt detail and Resend result (2026-08-17)

The Activity view now treats a delivery row as one immutable network attempt rather than the latest mutable state of one notification:

- Initial sends, automatic retries, and manual Resends render independently by attempt ID and exact send time. Relative time remains visible for scanning; the exact UTC timestamp is available in the row detail.
- A failed exchange enters **Failed** immediately even if the logical notification still has a scheduled retry. Parent state and next-attempt time are diagnostics, not a replacement status for that failed row.
- Expanding a row shows the bounded JSON request and the bounded endpoint response or transport error. The signing secret is neither returned by the API nor placed in browser state or logs.
- Workspace admins can confirm **Resend** on a failed attempt while the endpoint is enabled. One user action generates one idempotency key, shows queued/progress/result states, and reconciles the returned reservation with the polled attempt list without a full-page reload. Non-admins and disabled endpoints do not expose the control; server authorization remains authoritative.
- Activity polls only the newest page at a bounded cadence while the document is visible. Attempt-ID reconciliation prepends new rows without duplicating them or discarding already loaded keyset pages; changing filters/date bounds resets the relevant page set, and hidden/unmounted views stop polling. Manual Refresh remains available.

Render exposes Resend in its dashboard but does not publish a corresponding public replay route. The Bex REST, GraphQL, and MCP replay operations are therefore labeled extensions. The visual workflow and evidence semantics match; the API availability is intentionally broader for automation and agents.

## m26 management walkthrough result (2026-08-17, dev-8)

The closing authenticated walkthrough used two disposable Kratos identities, an explicit OpenFGA viewer tuple, the real GraphQL/store path, and a headless browser against the CAPD-backed dev-8 stack. It proved the management behaviors introduced after the Render audit:

1. The admin create page reported required name, destination, and event failures beside their fields, summarized them for assistive technology, and focused the first invalid control. The API independently returned `WEBHOOK_EVENT_FILTER_INVALID` for an unknown event and `WEBHOOK_NAME_CONFLICT` for the duplicate-name race; the create response exposed the mint-once secret only at creation.
2. The list searched endpoint names, destinations, and translated event labels, displayed a named no-match state, bounded five subscriptions to three visible chips plus “Show 2 more,” and projected one failed immutable attempt as **Retrying** with exact/relative timing. That projection came from the endpoint list's single batched store query, not a per-row history request.
3. The admin detail and Settings surfaces exposed Activity evidence, Resend, toggle/edit/save/delete, and the actionable picker. The read-only member saw the same endpoint with its destination reduced to the origin, could inspect the immutable failed attempt, and saw static/read-only list, create, Activity, and Settings surfaces with no create, toggle, Resend, save, or delete affordance. A direct viewer update still failed authoritatively as forbidden.
4. The picker has distinct translated loading, failure-with-Retry, empty-catalog, and ready states. Submission stays unavailable with a visible reason until the catalog is usable; successful retry restores the normal searchable/grouped/all-events picker rather than a second local vocabulary.
5. The probe endpoint was explicitly deleted, its attempt subtree cascaded, both personal workspaces and both identities were removed, and exact post-run checks reported zero probe identities, endpoints, or tenants. No screenshot, cookie jar, destination credential, or signing secret was persisted.

This walkthrough closes the observed management-feedback gap. It does not turn the dated 64-value Render picker into Bex's supported list: Bex still serves the 32 types backed by durable producers, while the fixture and weekly checker keep the 64-picker/67-OpenAPI split visible.
