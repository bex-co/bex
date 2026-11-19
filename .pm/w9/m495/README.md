# w9 · m495 — Build-time private network reach to workspace web & private services (`fromService` `host`/`hostport`)

**Worker:** worker9 **Goal:** a build in a workspace's canonical namespace can reach its own workspace's eligible `web_service` and `private_service` pods (honoring environment network isolation), closing the open gap recorded in ADR018 `Pre-Deploy Command` and w8/m129. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | Threat review & ADR022 addendum: extending per-build NetworkPolicies to own-workspace web/private service pods | 30m | — |
| [t002](t002.md) | Extend `build_datastore_reach.go` to admit TCP traffic from this App's build pods to eligible `web_service`/`private_service` pods in its canonical workspace namespace | 1h | t001 |
| [t003](t003.md) | Operator unit & envtest coverage: own-workspace reach allowed; isolated-environment, cross-workspace, and platform-namespace probes denied | 45m | t002 |
| [t004](t004.md) | Production verification (build querying a sibling private service via `fromService` `hostport` + cross-workspace negative control) and update ADR018 & ADR022 | 45m | t003 |
| [t005](t005.md) | Simplify | 20m | t004 |
| [t006](t006.md) | Test coverage | 30m | t004, t005 |
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
