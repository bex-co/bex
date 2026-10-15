# PostgreSQL upgrade-target floor dry-run recheck — sweep309

[Complete native commands, API-only dry-run requests/responses, public readbacks and cleanup](journey-results.json) · [finite ownership-guarded driver](journey-driver.py.txt) · [focused source test events](source-tests.json).

Published Bex0.3.2/a6350da0 creates one fresh Free PostgreSQL13 fixture with explicit plan free, connection-pool none and Frankfurt region. Native get and REST detail eventually report Available/version13. The unchanged Render2.27/a764810a7682 pin is the CLI oracle. The pinned native update command exposes no version or dry-run flag, so every version validation below is explicitly API-only. Full native argv/results are captured; the native mutation wire and response headers are not intercepted. API-only requests retain their exact method/path/body, workspace, pinned User-Agent, HTTP status and complete response, with Authorization redacted.

## Existing w4/308 is still unaccepted live

The single owned database receives fourteen finite API-only dry runs, seven through REST PATCH and seven through MCP update_postgres. Every request explicitly sends dryRun:true. Both surfaces accept13→14 and13→15: REST returns200, and MCP returns200 with a successful tool result, rather than the new named refusal POSTGRES_UPGRADE_TARGET_UNSUPPORTED. These four outcomes retain the existing [w4/308](../../../w4/blocked/308.md) live gate. No real version write is sent and no new issue is created.

The other ten cases pass: each surface accepts a supported16 preview and an omitted-version rename preview; equal13 is refused as POSTGRES_VERSION_NOT_NEWER, while unknown12 and99 are refused as POSTGRES_VERSION_UNKNOWN. MCP's normal HTTP200 error envelope is classified using its tool-error flag and code, not HTTP status alone. The accepted supported-target control establishes validation acceptance only; it does not establish a real upgrade, target SQL version or data retention.

After every request, the visible readback projection of id, name, plan, version, status, public intent, replicas, pool, disk, parameter overrides, HA and autoscaling stays unchanged, with Available/version13. UpdatedAt and hidden Kubernetes generation/spec are not exposed by this projection. These public reads do not independently prove every persisted field or absence of side effects. No connection-info endpoint, sensitive CLI flag or SQL session is used; actual SQL version/data contents and direct data-endpoint teardown remain unverified.

Published Bex and unchanged pinned Render then each perform one actual native rename, by the exact owned immutable ID, without a version flag. Both exit0 and read back their new name with Available/version13. This is a positive omitted-version native control on one shared owned fixture; it is not a second Render-created database or native CLI upgrade coverage.

## Source and limits

Source main fc4ff2b66 contains e7ae49c17. Twenty-four named checks across three top-level tests pass with no skips or failures: TestSetVersionValidationAndBackupGuard, TestUpgradeTargetBelowFloorRefusedEverywhere and TestSetVersionRunsTheUpdateChecks. The source validator refuses known newer targets below16 before a patch, retains unknown/equal/in-progress/unobserved/backup ordering, and its cross-surface regression checks unchanged spec/generation. Those source assertions are distinct from the bounded live public readbacks. This is a focused package selection, not the full backend or real-database suite, and the serving artifact revision is unobserved. No deployment-cause or regression claim is made.

The original w4/308 acceptance also requires real refused mutations, browser picker checks and a seeded13→16 SQL retention control. None is recertified here; the item remains blocked. Its creation catalog13–18 must remain available. The prior [w5/m116](../../../w5/done/m116/README.md) shared real/dry-run validation requirement is preserved; these results do not establish a separate preview-versus-real drift. No production implementation changes are made.

## Cleanup

Own parent dpg-db8iaedeelgc73d2i9p0 is deleted by native CLI exit0 after enumerating no read replicas and no pooler. REST returns404, native Postgres inventory is empty, original six service IDs remain, service quota is6 used/0 terminating and PostgreSQL quota0 used/0 terminating/limit1. No other resource or version write is created. Private copied Render configuration and request-body captures are removed; the continuing hunt retains only its isolated authentication and ownership ledger.
