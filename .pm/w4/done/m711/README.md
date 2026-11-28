# w4 · m711 — A service's outbound IPs are in its Connect menu's Outbound tab, as on Render, not a Settings card

**Worker:** worker4 **Goal:** a web service's, private service's or background worker's outbound IPs appear where Render shows them: in the header's Connect menu, in an Outbound section with Render's title and description and a copyable list. The Settings → Outbound IPs card goes away for every type, and a static site therefore shows no outbound IPs anywhere, as on Render. **Status:** done

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | The Connect menu gains an Outbound section listing the service's outbound IPs — **DONE** | 1h15m | — |
| [t002](done/t002.md) | The Settings → Outbound IPs card is gone for every type, static sites included — **DONE** | 45m | t001 |
| [t003](done/t003.md) | 8 locales, a dev-4 walk of a web service, a worker and a static site, and the ADR018 row — **DONE** | 45m | t002 |
| [t004](done/t004.md) | Render parity — **DONE** | 20m | t003 |
| [t005](done/t005.md) | Simplify — **DONE** | 15m | t004 |
| [t006](done/t006.md) | Test coverage — **DONE** | 30m | t004, t005 |
| [t007](done/t007.md) | Closeout — **DONE** | 10m | t006 |

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
- **Goal linkage:** [ADR008 § Render parity first](../../../../docs/ADR008-vision.md#render-parity-first) and [ADR125](../../../../docs/ADR125-stable-tenant-egress.md) (tenant egress).
- **Expected outcome:** a user following Render's docs (Connect → Outbound) finds the addresses where the docs say. A static site no longer lists addresses it never sends from.
- **Why now:** the addresses are read when allowlisting a service at a third party. That is what w21/m616 and w4/757 are making dependable, so the place to read them should be Render's before those land.
- **Render parity:** included, because this changes the dashboard. REST, GraphQL and MCP are unchanged.

## Outcome (2026-11-27)

Shipped as specified, dashboard only; REST `GET /v1/services/{id}/outbound-ips`, GraphQL `Service.outboundIps` and MCP `get_service_outbound_ips` are unchanged.

- **New home (for w21/m616 t005).** The list is `ConnectOutboundIps` in `dashboard/src/features/services/components/service-detail-header.tsx`, mounted in the Connect popover after SSH while `service.outboundIps.ips` is non-empty. It reads the header's own `server` read, so the header's first paint and requests are unchanged. `service-outbound-ips-panel.tsx`, its test, the `outbound-ips` Settings section, its route case and its skeleton slot are gone, and with them the navigation's one-section filter (`navigableSettingsSections`). The copy keys: `services.outboundIpsTitle` ("Outbound IP Addresses"), `services.outboundIpsLead` (Render's first line), `services.outboundIpsDescription` (unchanged words, the line m616 t005 rewrites) and `services.outboundIpsCopy` ("Copy outbound IP addresses"); `services.outboundIpsEmpty` is gone, since an empty list now shows no section.
- **Shape.** Render's `Ta` (`CronJob-DioyV2Vf.js`, re-read 2026-11-27) is a title, two lines and one multi-line code block of the addresses joined by newlines. bex draws the addresses as a list named by the title, with one Copy button that copies them one per line. Render's dedicated IP line is left out (`.pm/DO_NOT_DO.md`).
- **dev-4 walk (t003).** bex-api restarted by `dev-env.sh 4 up` with `BEX_OUTBOUND_IPS=203.0.113.7,198.51.100.24` (documentation addresses), so the real server read answered a non-empty set (the CAPD nodes have no ExternalIP), and the dashboard from this tree on `:50040`, signed in as a disposable verified identity whose workspace is (disposable). A web service, a background worker and a static site (disposable): at 1440 and 390 the web service's and the worker's Connect showed the Outbound section after SSH with both addresses, Copy put `198.51.100.24/32\n203.0.113.7/32` on the clipboard, Tab reached the Copy and Escape returned focus to Connect; the static site's header had no Connect; no Settings page (web, worker, static) had an outbound section or address. With production's count injected into the client's read (8 addresses), the popover ended at y=704 of 844 at 390 and y=624 of 900 at 1440, with nothing to scroll. Everything was deleted afterwards and bex-api restarted without the variable.
- **Render parity (t004).** Production answered 8 addresses for a first-party web service on both GraphQL and REST, which is what the section will list once deployed. The ADR018 "Service outbound IPs" row ([services-lifecycle.md](../../../../docs/ADR018/services-lifecycle.md)) now names the Connect menu. A running cron job keeps no Connect, as w3/m555's walk of Render saw. One drift filed: Render links "IP addresses or ranges" to its docs and bex's docs have no outbound IP page, [w4/760](../../760.md).
- **Simplify (t005).** The three Connect sections share `ConnectSectionTitle`; the identity filter `navigableSettingsSections` and its type were removed.
- **Tests (t006).** `service-detail-header.test.tsx` (Connect → Outbound: the three types, the order after SSH, the copy, an empty and an unselected list, a static site; the keyboard test reaches the new Copy) and `services.$serviceId.settings.test.tsx` (no type's section list or page shows an outbound address). Each of these mutants failed them: the section never rendered, rendered for an empty list, copied comma-joined, lost the lead line, sat before SSH, lost its list name, a static site given Connect, and an extra Settings section.

