# w9 · m495 — Build-time private network reach to workspace web & private services (`fromService` `host`/`hostport`)

**Worker:** worker9 **Goal:** a build in a workspace's canonical namespace can reach its own workspace's eligible `web_service` and `private_service` pods (honoring environment network isolation), closing the open gap recorded in ADR018 `Pre-Deploy Command` and w8/m129. **Status:** in progress (t001–t003, t005, t006 done; t004's production check waits for a pin that carries this milestone's commit; then t007)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Threat review & ADR022 addendum: extending per-build NetworkPolicies to own-workspace web/private service pods — **DONE** | 30m | — |
| [t002](done/t002.md) | Extend `build_datastore_reach.go` to admit TCP traffic from this App's build pods to eligible `web_service`/`private_service` pods in its canonical workspace namespace — **DONE** | 1h | t001 |
| [t003](done/t003.md) | Operator unit & envtest coverage: own-workspace reach allowed; isolated-environment, cross-workspace, and platform-namespace probes denied — **DONE** | 45m | t002 |
| [t004](t004.md) | Production verification (build querying a sibling private service via `fromService` `hostport` + cross-workspace negative control) and update ADR018 & ADR022 | 45m | t003 |
| [t005](done/t005.md) | Simplify — **DONE** | 20m | t004 |
| [t006](done/t006.md) | Test coverage — **DONE** | 30m | t004, t005 |
| [t007](t007.md) | Closeout | 10m | t006 |

## Definition of done

- On production: a service whose `buildCommand` queries a sibling `private_service` or `web_service` in the same workspace (and same isolation scope) via `fromService` (`host`/`hostport`) succeeds during build and goes live.
- A build in another workspace or another isolated environment is still denied by NetworkPolicy; both per-build NetworkPolicies are removed when the build settles.
- ADR018 (`Pre-Deploy Command` row) no longer lists private/web service build reach as an open gap, and ADR022 records the threat review and isolation invariants.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for one aspect of render parity` (Build Caching & Build Environment Parity, 2026-11-18, item 4); [ADR018](../../../docs/ADR018-render-parity.md) (`Pre-Deploy Command` open gap), [w8/done/m129/README.md](../../w8/done/m129/README.md), `lego/operator/internal/controller/build_datastore_reach.go:132-139`.
- **Goal linkage:** Render parity first ([ADR008](../../../docs/ADR008-vision.md#render-parity-first)) — closes an explicitly recorded, unsigned-off open gap from `w8/m129` so `fromService` (`host`/`hostport`) works during builds as well as pre-deploy and runtime.
- **Expected outcome:** Blueprints and SSG/SSR services (e.g., Next.js, Astro, Gatsby) whose builds query a sibling internal CMS or private API via `fromService` succeed without public-internet workarounds.
- **Why now:** Independent of the S3 cache milestones (w9/m492–m494) and directly closes a documented open follow-up from `w8/m129`.
- **Render parity:** omitted — pure operator NetworkPolicy/isolation mechanism with no REST, GraphQL, MCP, CLI, or dashboard schema/UI change; ADR018 and ADR022 updates are included in t004.

## Progress (2026-11-19, worker9): t001–t003, t005, t006 shipped in code; t004 needs production

**Triage.** Work. `build_datastore_reach.go:130-139` still opened only 5432/6379/6380, and ADR018's Pre-Deploy row and ADR022's Residuals listed private-service build reach as an open gap. The 2026-10-29 unblock record of w8/m129 approved reach to the owning workspace's "databases and services". Render: the free-plan migrate-in-build recipe proves builds are on the private network, where services listen on "almost any port", and "Static sites are not on a private network" ([render.com/docs/private-network](https://render.com/docs/private-network)).

**Second blocker found.** `fromService` `host`/`hostport` injects a bare `<slug>`. It resolves only through the App namespace's DNS search path (ADR041 D2), so from `bex-build` it did not resolve at all. A build Job outside its App namespace now gets `dnsConfig.searches: [<ns>.svc.cluster.local]`.

**What shipped (operator only; no REST/GraphQL/MCP/dashboard change):**

- `reconcileBuildReach` adds to the datastore grant a rule and a `<ws>/<app>-build-service-ingress` policy. They reach the serving pods (`app.bex.co/app In [paid web/private services]` AND `app.bex.co/app-id Exists`) of the build's own workspace, in its isolation scope, on any TCP port. Workers, cron jobs, static sites and free web services are never targets, and a static site's own build gets no service reach. All three policies are removed at the build's terminal observation.
- Docs: [ADR022 § Build reach to the owning workspace's services](../../../docs/ADR022-tenant-isolation.md#build-reach-to-the-owning-workspaces-services-w9m495-2026-11-19) (threat review and invariants), ADR018 Pre-Deploy row (gap closed; kpack remains, w1/121), ADR041 D2, ADR043, ADR060, `build-boundary.yaml`, `lego/operator/AGENTS.md`.
- Tests: see [t003](done/t003.md). Six mutations each failed the suite.

**t004 must check on production once the pin carries this milestone's commit** (`git merge-base --is-ancestor <commit> <pin>`, and `bex-system` runs that operator image):

- In `bex-canary`, create a paid `private_service` fixture that serves a marker on `$PORT`. Then create a web service (Blueprint, or an `envVars` literal equal to the fixture's `<slug>:<port>`) whose `buildCommand` runs `curl -fsS http://$CMS_HOSTPORT/` and fails without the marker. The build passes, and the service goes live.
- While it builds: `bex-build/bld-<srv>-datastore-egress` has two rules, and `<srv>-build-ingress` and `<srv>-build-service-ingress` exist in `bex-canary`'s namespace. The build Job's pod has `dnsConfig.searches: [<ns>.svc.cluster.local]`. All three policies are gone after the build.
- Negative control: a build in another workspace (e.g. `bex`) probing the fixture by `<slug>.<ns>.svc.cluster.local:<port>` and by pod IP times out.
- Delete every fixture by id (`GET` → 404) and log the QA session out.
