# w4 · m712 — Services and datastores in different regions don't reach each other over the private network, as on Render

**Worker:** worker4 **Goal:** a workspace's private network is split by the region each resource reads back, as Render's is. A service, a cron run or a pre-deploy command reaches another service's internal address, or a Postgres or Key Value internal URL, only when both read back the same region. Everything else stays as it is: same-region traffic, environment isolation, public URLs, external datastore URLs and SSH. Nothing restarts to apply it, and ADR041's unsigned "No region axis" goes. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | Count production's cross-region resources, design the boundary, and stop for the user if real cross-region traffic is found | 1h | — |
| [t002](t002.md) | App pods (services, cron runs, pre-deploy) carry their region label, relabelled in place | 1h30m | t001 |
| [t003](t003.md) | Postgres and Key Value pods carry their region label, relabelled in place | 1h30m | t001 |
| [t004](t004.md) | The workspace's private-network policies split by region × environment | 2h | t002, t003 |
| [t005](t005.md) | envtest, and a production check with disposable resources in two regions | 1h15m | t004 |
| [t006](t006.md) | Connect → Internal says the private network is per region, ADR041 D5 is struck, and ADR018 records the rule | 45m | t005 |
| [t007](t007.md) | Render parity | 20m | t006 |
| [t008](t008.md) | Simplify | 15m | t007 |
| [t009](t009.md) | Test coverage | 30m | t007, t008 |
| [t010](t010.md) | Closeout | 10m | t009 |

Total: **10 tasks, about 9h15m**.

## Definition of done

- In one workspace, a service that reads back `oregon` cannot open a TCP connection to these resources when they read back `frankfurt`:
  - a service's internal address, discovery name or pod IP;
  - a Postgres's internal URL;
  - a Key Value's internal URL.
- Two `frankfurt` resources reach each other as before. A resource created without a region counts as the region it reads back (the platform's alias).
- Environment isolation still holds within a region. Public URLs, external datastore URLs (through the SNI proxies), SSH and the platform's own probes and control paths are unchanged.
- Applying the change and every later reconcile leave each running pod's UID unchanged, which proves no rollout.
- The envtest suite covers each of these pairs:
  - same region and different regions;
  - with and without an environment;
  - isolated and not;
  - app to app, and app to datastore.
- On production, disposable resources in `bex-canary` show the refused and the admitted connection.
- The Connect menu's Internal section says the private network is per region, as Render's does. ADR041 D5 is struck and ADR018's private network rows state the rule.

## Source + Goal linkage

- **Source:** `/pm-brainstorm one aspect other than postgresql to w4`, 2026-11-27 (the aspect: networking, as Render's Inbound IP rules, Outbound IP addresses, Private network and Regions docs describe), item 3. Before filing it was re-checked by hand at `origin/main` `d3ee03af17`. No open, blocked or done item owns it.
  - `.pm/w6/README.md:130` (2026-11-25) counted "a region-scoped private network (ADR041 'No region axis')" among items "already owned, signed off or working". ADR041 D5 has no date and no user sign-off, and ADR018 records no such divergence.
  - Under ADR008, that dismissal does not stand. Nor does w9/m57's "conscious divergence" list.
- **Render:**
  - [render.com/docs/private-network](https://render.com/docs/private-network): "Your Render services in the same region can communicate over their shared private network" and "Other Render services are on the same private network if they're deployed in the same region _and_ they belong to the same workspace".
  - [render.com/docs/regions](https://render.com/docs/regions): "Each region provides a separate private network … services in _different_ regions can't communicate directly over a private network".
  - [render.com/docs/key-value](https://render.com/docs/key-value): the internal URL is "for connections from your other Render services running in the _same region_".
  - Render's Connect → Internal tab reads "All of your Render services in the same region can communicate internally over their private network" (dashboard bundle, `CronJob-DioyV2Vf.js` chunk).
- **bex today:**
  - Every create surface accepts any Render region and records it (`app.bex.co/region`, `resourcemeta.StampRegion`, `lego/backend/internal/resourcemeta/metadata.go:143-165`). Every surface reads it back (w9/105). The resource is placed in `BEX_REGION` (user decision D1, 2026-10-29, [w8/152](../../w8/done/152.md)).
  - Nothing reads the region for networking. `sameNamespaceNetworkPolicy` (`lego/backend/internal/store/namespaces.go:934-951`) splits a workspace only by isolated environment, so services in different regions reach each other.
  - ADR041 D5 (`docs/ADR041-service-addresses.md:85`) reads "No region axis | bex is one cluster; the private-network scope is workspace (+ environment isolation), never region".
- **Not against D1:** D1 decided that any Render region is accepted and placed in `BEX_REGION`. It says nothing about the private network. bex already applies Render's private-network limits: a free web service receives no private traffic (w8/m130), and workers, cron runs and pre-deploy commands only send (w6/m640).
- **Goal linkage:** [ADR008 § Render parity first](../../../docs/ADR008-vision.md#render-parity-first), [ADR041](../../../docs/ADR041-service-addresses.md) and [ADR043](../../../docs/ADR043-tenant-namespace-isolation.md) (tenant namespaces and environment isolation).
- **Expected outcome:** a workspace's resources reach each other privately on bex exactly when they would on Render. A Blueprint or setup moved between the two behaves the same on both.
- **Why now:** w9/105 made the region readable on every surface on 2026-11-18, so a resource now visibly carries a region that the network ignores. The longer that lasts, the more workspaces may come to depend on cross-region private traffic that Render refuses. t001 counts them first.
- **Risk and its gate:** all bex resources run in one cluster, so the region is only a label. Enforcing it could break a workspace that mixed regions without knowing, for example API-created services with one region and dashboard-created datastores with another. t001 counts this on production, and if it finds cross-region private traffic in use, it stops and asks the user before anything is enforced.
- **Render parity:** included. t006 changes the Connect menu's copy. The rule itself changes no REST, GraphQL or MCP field.
