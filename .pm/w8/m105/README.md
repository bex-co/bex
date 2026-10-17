# w8 · m105 — Make the Jekyll static quickstart build normally

**Worker:** worker8 **Goal:** the documented Jekyll source builds and publishes without authored tool/dependency installers. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [Supply reviewed Ruby and Bundler for Gemfile static builds](t001.md) | 75m | — |
| t002 | [Install Gemfile dependencies before authored static commands](t002.md) | 45m | t001 |
| t003 | [Audit shared static callers versions roots and cache behavior](t003.md) | 35m | t002 |
| t004 | [Verify documented Jekyll and static dependency parity](t004.md) | 20m | t003 |
| t005 | [Simplify the changed static tool and dependency paths](t005.md) | 15m | t004 |
| t006 | [Test actual generated Ruby static builds and neighbors](t006.md) | 50m | t004 |
| t007 | [Accept unchanged native quickstart and close the milestone](t007.md) | 20m | t005, t006 |

**Total:**7 tasks/260m. [Finding](finding.md) · [complete evidence](evidence/cli-sweep430/README.md).

## Definition of done

The unchanged documented Jekyll source and authored command build/publish on fresh native Free static fixtures from both clients without authored runtime/dependency installers. Actual reviewed Ruby3.3 selection, Bundler/Jekyll dependency use, source-lock preservation and real generated HTML/HTTP are proved. Meaningful generated-image/static tests cover supplier closure/version/root/skip/PATH/env/cache and other package-manager/runtime/buildless/publication neighbors, failing when Ruby supply or Gemfile installation is removed. Shared settings remain coherent; exact cleanup preserves baseline/current inventories, quotas and zero termination and removes own material. Existing patch mapping and unexercised architecture/hosting limits remain explicit. Code alone does not close the milestone.

## Source + Goal linkage

- **Source:** Authorized ordinary functional CLI sweep430,2026-10-17: both native first static Jekyll builds fail missing Bundler; Ruby-only controls fail missing dependencies; frozen bundle-install additions produce real sites. Actual shared build exports and local real-project controls isolate the prerequisites.
- **Goal linkage:** [ADR008](../../../docs/ADR008-vision.md), [static hosting ADR029](../../../docs/ADR029-static-sites.md), [build reliability ADR060](../../../docs/ADR060-build-worker-reliability-and-performance.md), [Render parity ADR018](../../../docs/ADR018-render-parity.md): a documented ordinary static-site quickstart should work without owner-supplied platform tooling.
- **Expected outcome:** Jekyll projects build through normal CLI static commands, respecting supported Ruby selection and dependency/skip behavior.
- **Why now:** Two matching original failures and incremental controls establish a distinct static Ruby/Gemfile gap. Prior npm/Yarn and primary-Ruby fixes do not schedule it. Coordinate shared static changes with [m90](../blocked/m90/README.md), preserving accepted earlier scopes.
- **Render parity:** Included because shared static build behavior reaches REST/GraphQL/MCP/Blueprint/dashboard/native consumers. No CLI fork, runtime-selection API or general buildpack product is proposed.
