# w3 · m635 — Build toolchain pins are fresh again and #201 closes

**Worker:** worker3 **Goal:** every pinned build-toolchain image equals its upstream at review time, the native Bun 1.4 and Ruby 3.3 / 3.4 / 4.0 lines carry the current patches, and the freshness monitor closes #201 on its own run. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Re-resolve the drift and list what moves | 20m | — |
| t002 | Bump the native Bun and Ruby lines | 45m | t001 |
| t003 | Refresh the Paketo builder and stack images | 45m | t001 |
| t004 | Adopt the rebuilt native-common-bookworm image | 30m | t001 |
| t005 | Qualify the new images on dev-3 | 60m | t002, t003, t004 |
| t006 | Production check and #201 closes | 30m | t005 |
| t007 | Simplify | 15m | t006 |
| t008 | Test coverage | 30m | t006 |
| t009 | Closeout | 10m | t008 |

## Definition of done

1. At closeout `python3 scripts/lib/toolchain-freshness.py resolve` reports no drift for the entries #201 listed (drift newer than the review opens its own issue).
2. #201 is closed by the scheduled freshness run, not by hand.
3. On production a `BUN_VERSION=1.4` build logs Bun 1.4.5 and a Ruby 3.4-line build logs the new patch, both serving 200, and the ClusterBuilder is Ready on the new builder.

## Source + Goal linkage

- **Source:** `/pm-brainstorm one render aspect for w1` (2026-11-25), each claim re-checked by an independent verifier agent the same day before filing, item 1. GitHub issue #201, open since 2026-11-19 with no board owner. Precedent: [w6/m291](../../w6/done/m291/README.md) (#105).
- **Goal linkage:** [ADR008 § Render parity first](../../../docs/ADR008-vision.md#render-parity-first) and build supply-chain freshness ([ADR060 D7](../../../docs/ADR060-build-worker-reliability-and-performance.md)). The parity case rests on Bun: Render's `BUN_VERSION=latest` "Uses the most recent stable release of Bun for every deploy", and a lockfile-only Bun service of unknown age builds on the 1.4 line's pin (`nativeversion_test.go:1014`), which is 1.4.4 while Bun 1.4.5 is out. Render's docs say nothing about how a partial Ruby version resolves, so no Ruby parity claim is made here.
- **Expected outcome:** native Bun and Ruby builds use current releases, the Paketo stack carries upstream fixes, and the review issue is closed.
- **Why now:** the issue has sat for six days with every observed digest already superseded; w9/138 deliberately left the 3.4 line on 3.4.12 until a real-build qualification, which this milestone runs.
- **Render parity omitted:** no REST, GraphQL, MCP or dashboard change; pins only, as in m291.
