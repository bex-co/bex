# Static build-command adapter acceptance — sweep257

2026-10-15 UTC, Bex production, isolated human-QA workspace bex-canary. Source `6255ed8b2ca67c8c0f85fd0eadc19bedbbe62459`; deployed backend/operator revision unknown. Published Bex v0.3.2 and unmodified Render v2.27.0 / a764810a768202704e7206eb7b87a47211fcd98e each create an equivalent fresh Free static site. Both tested adapter transitions pass; no new finding.

[Complete native commands/output, API requests/responses, deploy correlation, HTTP observations and cleanup](owned-native-api-runtime-cleanup.json), [driver](fixture-driver.py.txt), [four exact recorded-commit source comparisons](exact-source-comparisons.json), [versions/context](metadata.json), [redactions](redactions.json).

Both sites use public bex-co/bex main, root examples/runtime-version/node and publish directory `.`. No runtime, Dockerfile or builder is declared. The initial build command writes only a unique harmless JSON marker into package.json in the disposable build workspace. Both initial deploys become Live and public package.json returns HTTP200 with exactly its own marker.

On the Bex-created fixture, an actual GraphQL setBuildCommand(id,command:"") mutation clears the build command. On the Render-created fixture, an actual MCP update_service call with explicit workspaceId/serviceId and buildCommand:"" does the same. These are API-only writes, not CLI or dashboard edit coverage. Each writes only the command; publishPath remains `.`. Successful responses and independent GraphQL reads show command empty and builder auto. Each save opens one distinct deploy that becomes Live and publishes package.json byte-for-byte equal to the original file at that deploy's recorded Git commit, with no generated marker. The first MCP post-clear HTTP capture still has the old marker despite a Live history row; a later read proves convergence. The initial read is retained, not counted as an exact-content pass or a new defect.

After convergence, each actual native client runs deploys create ID --wait --confirm -o json. Both exit0 within the240-second QA bound, print valid JSON for exactly their one new Live deploy, and subsequent public package.json again equals the recorded commit's original bytes. This proves the cleared configuration persists beyond the adapter response and its initial rollout.

| Fixture client | Clear surface | Readback | Subsequent native waited deploy |
| --- | --- | --- | --- |
| bex | GraphQL | HTTP200 / empty command / auto builder / publish . | 0 / Live / 32.114s |
| render | MCP | HTTP200 / empty command / auto builder / publish . | 0 / Live / 32.081s |

All six deploy children are enumerated before deleting their exact owned parents; [all six child detail routes also return404](child-detail-absence.json). Both native deletes exit0; service detail and former public package.json return404, native lists omit both fixtures and retain all six baseline services. Final quota is services used6/terminating0; managed Postgres/Key Value remain unused. No direct live Kubernetes descendant inspection or native HTTP interception was performed. Per-command comparison configs are removed.

This partially accepts existing m59/t003. Dashboard writes, complete creation-history/legacy-record recovery and explicit strategy/omitted-command neighbors in t005 remain unverified here. The original m59 native compound command/publish-path clear is separately accepted by sweep184; this sweep keeps publishPath stable to isolate empty command intent. Render production was not contacted; MCP update_service is a documented Bex extension. No product fix, duplicate issue, commit or push is produced, and the milestone remains blocked.
