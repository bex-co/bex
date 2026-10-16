# Native Key Value persistence resets and identical saves — sweep345

[Results](results.json) · [complete native data/configuration, API and cleanup receipts](native-data-API-cleanup.json) · [actual disk-backed engine tests](source-events.jsonl) · [source runner](source-runner.json).

Source c84c348b0e4a2248ee0f80a09cc15165275e14d7 follows a scoped pull/rebase with owned changes restored; it adds only another QA note since sweep344. Serving revision is unobserved. Published Bex0.3.2/a6350da0 creates one fresh explicit Free Frankfurt Key Value with journal_snapshot and queue/noeviction. Native exit0, returned ID/name/workspace/plan and empty pre-inventory prove ownership. Native comparison uses unchanged Render2.27.0/a764810a7682 against Bex, never a Render account. Actual kv-cli connections execute the local redis-cli under a bounded real PTY.

## Data and process results

1. Bex writes a nonce/Unicode marker and counter1 in journal mode; Render reads the exact six-key projection (current marker/counter, other stages absent). Native INFO reports AOF enabled and records process identity.
2. Bex changes persistence to off with --confirm. After config_restart converges to available, Render reads all six scratch keys absent. A new off marker/counter is written through Render and read exactly through Bex; INFO reports AOF disabled and a different process. This is the documented intentional reset, not accidental data loss.
3. Render changes off to snapshot. After convergence, Bex sees all six scratch keys absent, including the old journal marker and newer off marker. A third marker/counter written through Bex is read exactly through Render. INFO reports AOF disabled and a third distinct process. No historical durable data reappears.
4. Both clients save the identical snapshot mode. A measured60.569-second later INFO retains the exact snapshot process, and both clients still read the exact snapshot marker/counter with old markers absent. Identical saves do not roll this observed fixture. Snapshot durability across an unrelated restart is not certified.

All7 native data comparisons,3 marker seeds and4 INFO runtime checks pass. Three API-only REST/GraphQL/MCP reads agree on the exact owned ID/name, Free/snapshot and available; MCP includes workspaceId. Successful native HTTP mutation bodies were not independently wire-captured: actual commands/output/exit/timings and full ordinary API readback receipts are retained, with that scope explicit.

## Source contract and controls

[Render's persistence documentation](https://render.com/docs/key-value#data-persistence) describes a restart and data reset when moving to or from Off. Bex deliberately permits persistence on Free (ADR021), unlike Render's paid-only persistence. No paid fixture or browser confirmation flow is exercised. The native --confirm changes only this disposable store's harmless data.

Backend keyvalue/service.go:835–857 validates and normalizes persistence input, shared by the native REST PATCH path. Operator controller/keyvalue_persistence.go:146–156 commits Off intent before removing its exact managed durable files, then bootstraps a durable target without an older dataset. The retained Go test command enables BEX_TEST_VALKEY=1 and executes only journal-snapshot_to_off and off_to_snapshot in TestValkeyPersistenceEngine/all_mode_pairs on both pinned engines7/8. Four leaf transitions/9 total named events pass,0fail/0skip, with real Docker volumes. Test-owned cleanup runs through its registered exact-ID handlers; no cluster or global Docker prune is used. This is a focused execution, not a full operator suite or additional product test code.

The prior [w4/m153 durability fix](../../../w4/done/m153/README.md) already passed durable-to-durable hosted handoffs. This ordinary native journey adds the Off reset directions and delayed identical-save control; it does not reopen or duplicate that finding. The new upstream w4/320 note's references to our completed m73/m77 paths are repaired without changing its finding.

## Setup limits and cleanup

Native create is private. API-only public:true on the exact owned fixture enables the normal external connection path, then readiness is awaited. An early publish guard and an early seed guard stop while provisioning/restarting, before either relevant intent/mutation; they are excluded local scheduling attempts, not failures or repeated creates. Valid connection commands execute only after available. Temporary Render config copies are removed after each call. A separate cached TLS PING diagnostic passes before deletion; it is API-only data-client evidence, not native coverage. Credential-bearing connection-info strings are omitted before ledger and kept only in one owner-only temporary file, which is removed after teardown.

Native Bex deletes the exact standalone owned store, exit0. Detail404, empty native Key Value list, original six services and quota Key Value0/terminating0 prove cleanup. The cached former endpoint no longer answers PONG. Physical Kubernetes/PVC/Secret inventories and same-time shutdown duration are unobserved. No pre-existing resource is mutated, and the isolated QA login remains for the continuing authorized loop. There is no new confirmed defect; this evidence remains uncommitted.

Helpers: [finite native journey](finite-native-driver.py.txt), [isolated client functions](isolated-client-helpers.py.txt), [shared state](shared-state-driver.py.txt), [cached TLS diagnostic](cached-TLS-diagnostic.py.txt), [bounded PTY](bounded-PTY-helper.py.txt).
