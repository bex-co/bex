# Static-site clear transition — sweep180, 2026-10-13 UTC

**Result: the existing m59 live defect remains.** Both native clients save the exact empty-command/changed-publish-path payload successfully, but both sites created with a build step fail their resulting deploy with `dispatching build: build: native runtime node requires buildCommand`. Their old generated marker pages/assets remain HTTP200. Equivalent sites created for direct publication, then built and cleared, succeed through both clients: Live, exact repository index at the recorded deploy commit, and generated version.txt404.

No duplicate issue or new implementation task: m59 already schedules this failure and current source contains its repair. Three targeted existing backend tests pass at 624669b0349173ab035e58dd468d4dcdd56afb61. The live platform revision remains unknown; observed failure is not proof that this checkout fix deployed or that it needs another code change.

## Native journey and wire

[Four fixtures with full native/runtime receipts](native-fixtures.json), [four actual native clear commands and 20 complete native HTTP captures](native-wire.json), [versions/context and cleanup](metadata-cleanup.json).

1. In isolated bex-canary human-OAuth configurations, create fresh Free static sites against https://github.com/bex-co/bex main, root examples/static-site. For each client create one with build command `mkdir -p dist; printf '%s' 'MARKER' > dist/index.html; printf '%s' 'MARKER_asset' > dist/version.txt` and publish directory dist. Create one for direct publication with no build command and publish directory dot. Exact native commands/nonces are retained in fixtures.
2. Require initial build-created Live with both exact marker files. Require initial direct-created Live with source index and asset 404. Add the same build step/dist path to each direct-created site; require Live and both marker files.
3. Run `bex services update OWN_ID --build-command '' --publish-directory . --confirm -o json` or the equivalent unmodified Render command against Bex. All four saves exit0 with empty stderr: Bex built 5.016s/direct 7.331s; Render built 6.919s/direct 4.472s. Each sends exactly one PATCH with `{serviceDetails:{buildCommand:"",publishPath:"."}}`, receiving HTTP200; surrounding four GETs succeed. No rewritten requests, no harness refusal. Captures mark QA owner email; Authorization is never journaled.
4. Created-built sites become build_failed through both clients; public bodies equal their previously generated markers and both old assets remain 200. Created-direct controls become Live, exact source page and asset 404. Native deploy counts are two for each born-built site and three for each born-direct site, consistent with one rollout per mutation.

[Fixture/poll driver](fixture-driver.py.txt), [finite exact-owned PATCH capture driver](capture-driver.py.txt), [GraphQL read driver](strategy-read-driver.py.txt), [cleanup driver](cleanup-driver.py.txt). Each proxy permits only owned-resource reads, exact owned clear PATCH and selected-workspace project reads needed by the unchanged client's hydration. It forwards original path/body/response bytes; recorded copies mask private values. All children/proxies/copied configs are reaped or removed.

## Diagnosis and preserved controls

[GraphQL reads before control clears](strategy-before-control-clear.json) and [after all clears](strategy-after-all-clear.json) are API-only diagnosis, not native write coverage. Both failing sites read runtime empty, buildCommand empty, publishPath dot and builder native. Both successful final controls read the same commands/path/runtime with builder auto. This corroborates the original creation-history mechanism. It does not exercise GraphQL/MCP/dashboard writes or a platform deployment SHA.

[Existing targeted tests](local-existing-tests.json) pass TestClearedStaticBuildCommandPublishesDirectly, TestSetBuildCommandOnStaticSite and TestGraphQLSetBuildCommandRoundTrip under Go1.26.8/GOWORKoff. These use unchanged current code and local fake services; no full backend/operator suite or live-deployment claim. Source already uses setBuildCommand at build_strategy.go:77, settings.go:387 and service.go:4065, plus operator directStaticPublish:1465 for legacy native-builder records. Preserve that landed repair and its existing shared-caller audit.

[Own build progress diagnostic](build-progress-diagnostic.json) confirms real native compilation; queued/rollout waits are retained as convergence, not new issues. [One TLS read timeout](excluded-read-timeout.json) occurred before a deploy-list poll; it was retained and excluded, with a later fresh read succeeding. No mutation was replayed. Pending/failed-content preservation is established for the two failed clears by exact old marker/asset bytes; successful publication's full pending interval was not continuously watched.

## Cleanup and remaining work

[Final read-only reconciliation](final-cleanup.json), [marked redactions](redactions.json): all four exact owned IDs deleted; ten owned deploy children enumerated. Native deletion acknowledgments followed by parent detail404, list absence and former public runtime404. Initial global termination counters lagged and are retained; final quota is services used 6/terminating 0, PG/KV used 0/terminating 0, native list exactly the baseline six. No local container or Kubernetes descendant inspection. Copied Render configs and capture servers removed.

m59 stays blocked on fresh passing build-created acceptance after the deployed behavior changes, existing legacy-record recovery, explicit-strategy/omitted-field neighbors and GraphQL/MCP/dashboard writes. Render production remains untested. This hunt filed evidence and performed owned cleanup; it implemented or shipped no product changes.

The initial archive cleanup check counted its capture-driver filename as a proxy directory because they share a prefix. Restricting that count to directories confirms zero copied proxy configs; the fixture/capture cleanup itself had completed. This was an archive check correction, not a product failure.
