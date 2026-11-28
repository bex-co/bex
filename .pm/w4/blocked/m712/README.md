# w4 · m712 — Services and datastores in different regions don't reach each other over the private network, as on Render

**Worker:** worker4 **Goal:** a workspace's private network is split by the region each resource reads back, as Render's is. A service, a cron run or a pre-deploy command reaches another service's internal address, or a Postgres or Key Value internal URL, only when both read back the same region. Everything else stays as it is: same-region traffic, environment isolation, public URLs, external datastore URLs and SSH. Nothing restarts to apply it, and ADR041's unsigned "No region axis" goes. **Status:** blocked (t001–t004 and t006–t009 done; t005 waits for production to carry `c92ab66019`, then t010)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Count production's cross-region resources, design the boundary, and stop for the user if real cross-region traffic is found — **DONE** | 1h | — |
| [t002](done/t002.md) | App pods (services, cron runs, pre-deploy) carry their region label, relabelled in place — **DONE** | 1h30m | t001 |
| [t003](done/t003.md) | Postgres and Key Value pods carry their region label, relabelled in place — **DONE** | 1h30m | t001 |
| [t004](done/t004.md) | The workspace's private-network policies split by region × environment — **DONE** | 2h | t002, t003 |
| [t005](t005.md) | envtest, and a production check with disposable resources in two regions | 1h15m | t004 |
| [t006](done/t006.md) | Connect → Internal says the private network is per region, ADR041 D5 is struck, and ADR018 records the rule — **DONE** | 45m | t005 |
| [t007](done/t007.md) | Render parity — **DONE** | 20m | t006 |
| [t008](done/t008.md) | Simplify — **DONE** | 15m | t007 |
| [t009](done/t009.md) | Test coverage — **DONE** | 30m | t007, t008 |
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

## Production count (t001, 2026-11-27)

Read-only, on the production app cluster (`scripts/fetch-app-kubeconfig.sh`), at about 04:05Z on 2026-11-28 (pin `7d1dce861173`, which carries w9/105's `StampRegion`, `018004cd3d`). bex-api runs with `BEX_REGION=fsn1`, so a resource without a region reads back `frankfurt`.

- **Resources by region.** 32 App, 5 Database and 3 KeyValue CRs across 10 workspaces. **None** carries `app.bex.co/region`: every create so far named no region, so all 40 read back `frankfurt`.
- **Workspaces whose resources read back more than one region: 0.** So no env var, secret file or Blueprint link can name a resource of another region, and no workspace has cross-region private traffic to lose. The milestone continues.
- **Policies.** 84 hosting namespaces carry `allow-same-namespace`; none has an `isolate-<env>` boundary (no isolated environment). No CNPG Pooler runs.

## Production check baseline (2026-11-28, 04:35Z, before the upgrade)

Pod UIDs of first-party workloads, to compare once production runs this milestone's operator (t005 step 2). bex's own workspace (`tea-d98210cbbpdc73dcrkvg`):

- `tea-d98210cbbpdc73dcrkvg-beancount-forum-578bb9cfb8-mnznm` `d013c2b0-93cf-4e09-9aec-5bd6a8a14de2`
- `tea-d98210cbbpdc73dcrkvg-twenty-crm-74bf7f5fd8-k7rc5` `bd4d15fa-fef1-4f29-9293-be5efb59f05f`
- Postgres `dpg-d9nqg95cavls73fp8m20-1` `80172205-1260-4d77-b4e1-ac823da639de`
- Key Value `red-d9p49kdrtmes73c34ovg-0` `5e84c5b5-629b-438b-bd70-9d351973dde2` and its proxy `red-d9p49kdrtmes73c34ovg-internal-5bf6648486-2pm5p` `b6211ff1-ca76-441b-8f67-90b501820bc2`

`bex-canary` (`tea-daif693dqjvc73e7as3g`): `tea-daif693dqjvc73e7as3g-hello-go-7875965c6b-29whc` `b8dc0067-2fe2-44a7-8ce0-5813cd00f25a`, Key Value `red-dbpf5q5me2kc73f2l0og-0` `9cb16254-3a90-46c7-a2a6-b859bcf7a35e`.

## Design (t001)

- **Label.** `app.bex.co/region` (`appv1alpha1.LabelRegion`), the same key as the CR annotation, whose value it copies (`appv1alpha1.AnnotationRegion`, which `resourcemeta.AnnotRegion` now aliases). The operator sets it only when the CR carries the annotation: a resource created without a region gets no label. Render's region is fixed at creation, so the value never changes.
- **A pod without the label** belongs to the region an unstamped resource reads back, `BEX_REGION`'s alias (`NamespaceReconciler.Region`, wired from `resourcemeta.Config.EffectiveRegion("")`). Its selector is `region NotIn [the other four Render regions]`, which matches a pod without the key and one labelled with the alias. With `BEX_REGION` unset the alias is empty: `NotIn [all five]`, so unstamped pods share no network with any region.
- **Policies, region × environment.** Kubernetes cannot select "the same value as mine", so each region has its own policy, and all five Render regions get theirs whether or not the workspace uses them (a resource's first pod has its network from its first moment, and the converge lists no resource). In one selector the expressions are ANDed, so each policy's selector is `environment-id NotIn [isolated]` AND the region's expression, and its one ingress peer and one egress peer are that same selector: a pod reaches another privately only within its region and environment scope.
  - The alias keeps the old names: `allow-same-namespace` and `isolate-<env>`. Each other region gets `allow-same-region-<region>` and `isolate-<env>-<region>`.
  - Egress decides it: the operator's per-store ingress policies (the Key Value internal proxy's clients, an isolated datastore's peers) stay region-blind, and a pod of another region still cannot send to them.
- **No gap.** No production pod carries the label and the alias's policies keep their names, so converging from today's set only narrows `allow-same-namespace` away from pods that do not exist yet. Other regions' policies are written before the alias's, and boundaries before allows, as today.
- **No rollout.** The label joins the in-place labels (`inPlacePodLabels`, beside `environment-id` and `network-isolation`): new pods carry it from their template; an existing workload keeps its template (`holdIsolationLabel`) and its running pods are relabelled in place (`relabelPodsInPlace`), which `bex-operator-pod-labels` must admit. CNPG relabels instances from `inheritedMetadata`. A Pooler carries it in its template only when new (or already), and its running PgBouncer pods are relabelled in place. The store's own helpers that dial it carry it too: Key Value backup and purge Jobs, Postgres logical exports, CNPG's Jobs.

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
  - Every create surface accepts any Render region and records it (`app.bex.co/region`, `resourcemeta.StampRegion`, `lego/backend/internal/resourcemeta/metadata.go:143-165`). Every surface reads it back (w9/105). The resource is placed in `BEX_REGION` (user decision D1, 2026-10-29, [w8/152](../../../w8/done/152.md)).
  - Nothing reads the region for networking. `sameNamespaceNetworkPolicy` (`lego/backend/internal/store/namespaces.go:934-951`) splits a workspace only by isolated environment, so services in different regions reach each other.
  - ADR041 D5 (`docs/ADR041-service-addresses.md:85`) reads "No region axis | bex is one cluster; the private-network scope is workspace (+ environment isolation), never region".
- **Not against D1:** D1 decided that any Render region is accepted and placed in `BEX_REGION`. It says nothing about the private network. bex already applies Render's private-network limits: a free web service receives no private traffic (w8/m130), and workers, cron runs and pre-deploy commands only send (w6/m640).
- **Goal linkage:** [ADR008 § Render parity first](../../../../docs/ADR008-vision.md#render-parity-first), [ADR041](../../../../docs/ADR041-service-addresses.md) and [ADR043](../../../../docs/ADR043-tenant-namespace-isolation.md) (tenant namespaces and environment isolation).
- **Expected outcome:** a workspace's resources reach each other privately on bex exactly when they would on Render. A Blueprint or setup moved between the two behaves the same on both.
- **Why now:** w9/105 made the region readable on every surface on 2026-11-18, so a resource now visibly carries a region that the network ignores. The longer that lasts, the more workspaces may come to depend on cross-region private traffic that Render refuses. t001 counts them first.
- **Risk and its gate:** all bex resources run in one cluster, so the region is only a label. Enforcing it could break a workspace that mixed regions without knowing, for example API-created services with one region and dashboard-created datastores with another. t001 counts this on production, and if it finds cross-region private traffic in use, it stops and asks the user before anything is enforced.
- **Render parity:** included. t006 changes the Connect menu's copy. The rule itself changes no REST, GraphQL or MCP field.
