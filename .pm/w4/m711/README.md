# w4 · m711 — A service's outbound IPs are in its Connect menu's Outbound tab, as on Render, not a Settings card

**Worker:** worker4 **Goal:** a web service's, private service's or background worker's outbound IPs appear where Render shows them: in the header's Connect menu, in an Outbound section with Render's title and description and a copyable list. The Settings → Outbound IPs card goes away for every type, and a static site therefore shows no outbound IPs anywhere, as on Render. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | The Connect menu gains an Outbound section listing the service's outbound IPs | 1h15m | — |
| [t002](t002.md) | The Settings → Outbound IPs card is gone for every type, static sites included | 45m | t001 |
| [t003](t003.md) | 8 locales, a dev-4 walk of a web service, a worker and a static site, and the ADR018 row | 45m | t002 |
| [t004](t004.md) | Render parity | 20m | t003 |
| [t005](t005.md) | Simplify | 15m | t004 |
| [t006](t006.md) | Test coverage | 30m | t004, t005 |
| [t007](t007.md) | Closeout | 10m | t006 |

Total: **7 tasks, about 4h**.

## Definition of done

- A running web service, private service or background worker with outbound IPs has an Outbound section in its Connect menu. The section shows Render's title "Outbound IP Addresses", Render's description and the addresses as a copyable list. A service whose list is empty shows no Outbound section, as Render shows none.
- No service type has a Settings → Outbound IPs card, and the settings skeleton has no slot for one. A static site has no Connect menu and no outbound IPs, as on Render.
- The Connect menu's pending and ready states hold at 1440 and 390 px, the Outbound section is reachable by keyboard, and the copy exists in all 8 locales.
- The ADR018 "Service outbound IPs" row's dashboard note names the Connect menu.
- The description keeps its copy key, `services.outboundIpsDescription`, which w21/m616 t005 names as the copy it will change.

## Source + Goal linkage

- **Source:** `/pm-brainstorm one aspect other than postgresql to w4`, 2026-11-27 (the aspect: networking, as Render's Inbound IP rules, Outbound IP addresses, Private network and Regions docs describe), item 2. Before filing it was re-checked by hand at `origin/main` `d3ee03af17`, against Render's dashboard bundle (`index-D5V4ZgbE.js` and its `CronJob-DioyV2Vf.js` chunk), which was read the same day.
  - No open, blocked or done item owns it. w8/010 built the Settings card on 2026-09-08 as "bex's equivalent" of Render's Connect panel, with no recorded sign-off for the placement.
  - w21/m616, blocked on the user, changes which addresses the list holds and, in its t005, the copy. This milestone only moves where the list appears. It keeps today's truthful "can change" sentence, under the same copy key, until m616 t005 replaces it.
- **Render docs:** [render.com/docs/outbound-ip-addresses](https://render.com/docs/outbound-ip-addresses) says "Open the Connect dropdown in the upper right" and "Switch to the *Outbound* tab", and "static sites don't use outbound IP addresses, because they can't initiate outbound traffic".
- **Render's dashboard bundle:**
  - The server page mounts the Connect menu unless `env.isStatic`. Its tabs are Internal (when the service has internal addresses), SSH (when SSH is available) and Outbound (when `outboundIPs` is non-empty).
  - The Outbound tab is titled "Outbound IP Addresses". It reads "Network requests from your service to the public internet will come from one of the following IP addresses or ranges." and "These IP addresses are not unique to your service or workspace. They are shared by other Render services in the same region.", over a multi-line copyable block.
  - Render keeps no outbound IP card on any Settings page; "Outbound IP Addresses" appears only in that tab.
  - The tab's dedicated IP line ("Need a unique, static outbound IP? Create a …") is out of scope: dedicated outbound IPs are a `.pm/DO_NOT_DO.md` non-goal.
- **bex today:**
  - `dashboard/src/features/services/lib/settings-sections.ts:88` pushes `outbound-ips` for every type but cron, static sites included, and `routes/services.$serviceId.settings.tsx:343-345` renders `ServiceOutboundIpsPanel` there.
  - The Connect popover (`ServiceConnectButton`, `features/services/components/service-detail-header.tsx:664-735`) holds only Internal and SSH.
- **Cron jobs stay as they are:** Render's cron header mounts the same Connect menu. w3/m555's live walk of Render saw no Connect on a running cron job, and bex's cron header has none either.
- **Goal linkage:** [ADR008 § Render parity first](../../../docs/ADR008-vision.md#render-parity-first) and [ADR125](../../../docs/ADR125-stable-tenant-egress.md) (tenant egress).
- **Expected outcome:** a user following Render's docs (Connect → Outbound) finds the addresses where the docs say. A static site no longer lists addresses it never sends from.
- **Why now:** the addresses are read when allowlisting a service at a third party. That is what w21/m616 and w4/757 are making dependable, so the place to read them should be Render's before those land.
- **Render parity:** included, because this changes the dashboard. REST, GraphQL and MCP are unchanged.
