# w4 · m710 — Every Networking panel shows the workspace and environment inbound IP rules that also apply, as Render's does

**Worker:** worker4 **Goal:** the Networking panel of a web service, a static site, a Postgres database, a Key Value instance and an environment shows, read-only above its own editable rules, every higher level of inbound IP rules that also applies to it, how many IP ranges get through all levels, and Render's Validate IP address check that names the level that blocks an address. Today each panel shows only its own level. **Status:** done

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | A shared "Affected by" block shows the workspace and environment rules read-only, and the service Networking panel mounts it — **DONE** | 1h30m | — |
| [t002](done/t002.md) | Postgres Access control, Key Value Networking and the environment's Networking card mount the same block — **DONE** | 1h | t001 |
| [t003](done/t003.md) | The panel counts the IP ranges every level allows and checks an address with Render's Validate IP address field — **DONE** | 1h15m | t001 |
| [t004](done/t004.md) | Matching skeletons, 8 locales, a dev-4 walk with all three levels set, and the ADR018 rows — **DONE** | 1h | t002, t003 |
| [t005](done/t005.md) | Render parity — **DONE** | 25m | t004 |
| [t006](done/t006.md) | Simplify — **DONE** | 15m | t005 |
| [t007](done/t007.md) | Test coverage — **DONE** | 30m | t005, t006 |
| [t008](done/t008.md) | Closeout — **DONE** | 10m | t007 |

Total: **8 tasks, about 6h05m**.

## Definition of done

- A web service or static site in an environment shows, above its own rules, an "Affected by" section for the workspace level and one for the environment level. Each has Render's breadcrumb, a View rules link to where that level is edited, and that level's rules as a read-only Source/Description table, or Render's warning that the level blocks all internet traffic when it has no rule. A service outside any environment shows only the workspace level.
- The Postgres Access control panel and the Key Value Networking panel show the same levels. The environment's Networking card shows the workspace level only, and Workspace Settings shows none.
- Each panel says how many IP ranges are allowed through every level together. Its Validate IP address field reads "Access is allowed for this IP address" for an address every level admits. Otherwise it names the first level that refuses the address: workspace, then environment, then the resource's own unsaved draft.
- Pending and ready states match at 1440 and 390 px, in all 8 locales.
- On dev-4, with all three levels set so that only one address passes them all, the service panel's count and its checks for allowed and refused addresses match what the edge actually admits.
- The ADR018 rows for service, environment, workspace, Postgres and Key Value inbound IP rules describe the view.

## Source + Goal linkage

- **Source:** `/pm-brainstorm one aspect other than postgresql to w4`, 2026-11-27 (the aspect: networking, as Render's Inbound IP rules, Outbound IP addresses, Private network and Regions docs describe), item 1. Before filing it was re-checked by hand at `origin/main` `d3ee03af17`, against Render's dashboard bundle `index-D5V4ZgbE.js`, which was read the same day. No open, blocked or done item owns it.
  - w4/m28 made the levels compose in enforcement. w9/m388 added the workspace level. Neither shows the levels together.
  - w3/155 moves the workspace's own rules to a Network Access page. That is a separate change. This milestone's workspace View rules link goes to `/network-access`, which redirects to Workspace Settings until w3/155 makes it the page.
- **Render docs:** [render.com/docs/inbound-ip-rules](https://render.com/docs/inbound-ip-rules) says "an inbound IP must be allowed by _each level's_ rules to successfully connect" and "When you view a service's inbound IP rules, you can see each level of rules that apply to it".
- **Render's dashboard bundle:** one Networking component serves web and static services, primary Postgres, Key Value and environments. Find it by the strings "Affected by:", "View rules" and "Validate IP address".
  - It reads the owner's list, except on the workspace itself, and the environment's list, except on an environment.
  - Each level gets an "Affected by:" header with a Workspace › Environment breadcrumb and a View rules link that opens in a new tab. Below that is a Source/Description table, or "All internet traffic is blocked by `<level>` inbound IP rules."
  - A status line reads "`N` IP range(s) is/are allowed from outside of your private network". `N` is the intersection of every level's CIDRs.
  - The Validate IP address field (placeholder `0.0.0.0`) has four states: "Enter an IP address", "This IP address is not valid", "Access is allowed for this IP address" and "Access is not allowed for this IP address". When an address is refused, the message names the level: "Access is not allowed for this IP address based on your Workspace Inbound IP Rules." (likewise Environment, and the resource's own type).
- **bex today:**
  - `dashboard/src/features/services/components/service-networking-panel.tsx:26-121` renders only `currentAllowList`, which is the service's own `ipAllowListEntries` (`routes/services.$serviceId.settings.tsx`). The Postgres `access-control-panel.tsx`, `key-value-networking-panel.tsx` and `environment-networking-card.tsx` likewise show one level each.
  - The data is already served. `Workspace.ipAllowList` can be read by any member who can read the workspace (`lego/backend/internal/workspaces/graphql.go:68-85`), and `features/workspaces/hooks/use-workspace-ip-allow-list.ts` reads it. `Environment.ipAllowListEntries` comes from `lego/backend/internal/environments/graphql.go:46-47`.
- **Goal linkage:** [ADR008 § Render parity first](../../../../docs/ADR008-vision.md#render-parity-first) and [ADR032](../../../../docs/ADR032-environments.md) (environments and their rules).
- **Expected outcome:** someone who allows an address on a service, a database or an environment can see on the same page whether a higher level still blocks it, and which one.
- **Why now:** w9/m388 shipped the workspace level on 2026-11-14. Since then a request can be refused by any of three levels. Each level's page shows only its own rules, so the levels contradict each other silently, and the only way to find out is to visit each page in turn.
- **Render parity:** included, because this changes the dashboard. The backend reads it uses are unchanged on REST, GraphQL and MCP.
- **Not in scope:** Render's editor also asks `api.ipify.org` for the viewer's own address. t005 checks what Render uses that address for and files a follow-up for it, rather than building it here.

## Outcome (2026-11-27)

Shipped as specified, dashboard only; bex-api's REST, GraphQL and MCP reads are unchanged. The shared view is `dashboard/src/common/components/inbound-ip-levels.tsx`, its reads `common/hooks/use-inbound-ip-levels.ts` (`workspace(id).ipAllowList`, `environment(id).ipAllowListEntries`), and the count and check `common/lib/ip-allow-list.ts`. The parity record is the Inbound IP allowlist row of [docs/ADR018/custom-domains.md](../../../../docs/ADR018/custom-domains.md), cross-referenced from the workspace, Postgres, Key Value and environment rows.

- **Deviations from the tasks.** The count and the check read IPv4 only, not IPv4 and IPv6 (t003): Render's view parses IPv4 alone, its docs say "Only IPv4 CIDR ranges are supported", and bex's service hosts publish no AAAA record, so an IPv6 rule admits no client. The count reads the saved lists and the check the editor's draft, as Render's code does. The field's answer is also a line under it, not only the icon's tooltip, so a touch screen reads it. A level that cannot be read keeps the panel's own editor, with a line saying so, where Render replaces the whole panel.
- **Skeletons (t004).** Every pending state reserves its static copy with its own translated text. On dev-4 the levels' pending and ready heights were equal at 1440 and 390 px in all 8 locales, and the service Settings Networking card and the environment settings Networking card were equal on the route's pending frame (held on its code chunk) in English and Spanish. A Postgres or Key Value instance in an environment learns that only from its own allowlist read, so its preview assumes no environment, as it assumes the rule count, and grows by the environment's section when the read lands.
- **dev-4 walk (t004).** bex-api as `dev-env.sh 4 up` left it, the dashboard from this tree on `:50040`, and a disposable verified identity whose workspace (disposable) was set to Scale. Workspace `192.168.0.0/16` + `127.0.0.0/8`, environment `192.168.88.0/24` + `127.0.0.1/32`, web service `192.168.88.243/32` + `10.99.0.0/16`, so only the address of one curl pod passed all three. The service panel read "1 IP range is allowed", answered allowed for that pod, refused by the Web Service rules for `127.0.0.1` (a port-forward to the edge) and by the Environment rules for a second pod on another node (`192.168.221.x`), and refused `10.99.0.1` by the Workspace rules. The edge answered the three sources `200`, `403` and `403`. The environment, Postgres and Key Value panels drew the same levels in the browser. Everything was deleted afterwards.
- **Render parity (t005).** Render's editor asks `api.ipify.org` for the viewer's address only to offer "Use my IP address" in an empty rule row; filed as [w4/759](../../759.md).
- **Simplify (t006).** The service panel reuses `toIPAllowListDrafts` for its saved list; the rest was already direct.
- **Tests (t007).** `common/lib/__tests__/ip-allow-list.test.ts`, `common/components/__tests__/inbound-ip-levels.test.tsx` and `common/hooks/__tests__/use-inbound-ip-levels.test.tsx`. Reverting the narrower-range rule, the refusal order, a panel's draft feed, a panel's environment, the count's saved lists or the skeleton's environment section each failed them.
