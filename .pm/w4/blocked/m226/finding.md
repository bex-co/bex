# Stale Key Value configuration skips a persistence data-deletion confirmation

**Severity:** major functional data-loss consequence, with a minor stale-policy symptom from the same read gap. **Source:** continuous qa-find-bugs w4, a881, 2026-10-17 UTC. **Research main:** b9d3cdf6c3c8bc58b7ea5d6868243744db79b11c. This ships the filing only.

## Ordinary two-client reproduction

Owned Free/private Valkey8 **red-db9m0hcflioc739mcl5g / qa-20261017-a881-policy**, workspace tea-d98210cbbpdc73dcrkvg, created at11:47:18.081 with allkeys-lru and Journal + Snapshot. Native operations, complete narrow independent reads, successful detail-poll requests/responses, typed-dialog trees, timed UI values and data-plane receipts are in [evidence.json](evidence.json).

The two clients are ordinary separate dashboard documents in one authenticated browser context, each with its own Apollo client. B changes only this owned store, then closes; A returns to the foreground. No response, error, clock or transport delay was injected. All data-plane commands target this store's own pod and QA-prefixed keys; credentials stay in its existing process environment and never enter the capture.

### Desktop: Journal + Snapshot shown, Off actually active

1. Fresh1440×1000 A shows Journal + Snapshot at11:52:40.678. Keep it open.
2. B → Persistence mode → Off → Save. The correct **Delete all data and change persistence?** dialog requires `sudo clear key value qa-20261017-a881-policy`. Type it and confirm. SetKeyValuePersistenceMode(off) succeeds at11:53:41.490. This intentional first discard is expected and is not the finding.
3. A's normal KeyValue poll at11:54:10.419 returns HTTP200/available, updatedAt11:53:59Z. A still shows Journal + Snapshot at11:54:11.614. The independent complete read at11:54:12.103 reports persistenceMode=off. Runtime confirms appendonly=no and save=""; the expected initial clear leaves the old QA keys absent.
4. Write **new** permanent and expiring strings, a hash and a list while Off is active. The complete CLI receipt at11:54:51.144 reads back `qa-a881 OFF-only new marker 🐟`, the corresponding expiry/hash/list and the exact expiry deadline. These writes happened after the confirmed initial clear.
5. In untouched A, Edit Persistence mode starts from Journal + Snapshot. Choose Snapshot only → native Save changes. At11:55:14.011 SetKeyValuePersistenceMode(snapshot) succeeds directly. No typed deletion dialog was filled or accepted for this save; dialog count iszero. The card's general text still says durable-to-durable changes preserve data and Off boundaries discard it, but the displayed starting mode is wrong.
6. Available returns at11:55:32.529. Runtime at11:55:38.932 has snapshot saving enabled, appendonly=no, and **both strings absent, hash empty, list empty, expiry=-2**. The new Off-only markers were discarded by the actual Off-to-Snapshot transition.

### Fresh mobile: Snapshot only shown, Off actually active

1. Fresh390×844 A at11:55:58.661 shows Snapshot only. B explicitly confirms Off; its native save succeeds at11:56:02.914.
2. A's normal HTTP200 KeyValue poll at11:56:27.641 returns available/updatedAt11:56:12Z. A still shows Snapshot only at11:56:29.015; an independent read at11:56:29.779 says off. The viewed screenshot `.playwright-mcp/qa-a881-mobile-stale-persistence.png` captures this old saved label. Document width is390.
3. Confirm the expected empty Off runtime, then write distinct **new** mobile markers. At11:57:06.513 the CLI reads back `qa-a881 MOBILE OFF-only marker 雪`, the expiring string, hash/list and exact deadline, with appendonly=no and save="".
4. A's Edit starts from Snapshot only; choose Journal + Snapshot. Native Save directly submits SetKeyValuePersistenceMode(journal-snapshot) at11:57:21.573, without the typed deletion confirmation. At11:57:38.106 the store is available.
5. Runtime at11:57:43.469 has appendonly=yes but **all these new markers absent**. This independently repeats the consequence with the reverse durable target and a fresh document.

The deletion is Bex's documented actual Off boundary, not failed durable conversion. The defect is the old displayed/decision mode causing the later change to miss the existing transition-specific confirmation. This is an ordinary configuration workflow; no permission or authentication defect is claimed.

### Same producer: stale Maxmemory Policy

On fresh mobile A, save volatile-ttl normally and wait for it to run. B then saves allkeys-lru at11:51:07.133. A receives native successful config_restart polls and an available poll at11:51:19.111, while its field remains volatile-ttl. An independent read at11:51:19.737 reports allkeys_lru. At11:52:37.558 Edit still starts from volatile-ttl with no-op Save disabled; Cancel sends no write. Reload at11:52:40.678 correctly shows allkeys-lru. The viewed `.playwright-mcp/qa-a881-mobile-cross-tab-policy.png` and timed observations support this minor symptom; no wrong runtime policy is claimed.

## Passing controls

- Canceling a policy draft and entering Edit without a change send zero SetKeyValueMaxmemoryPolicy calls. Saved noeviction and volatile-ttl appear correctly after the local mutation/refetch and in runtime CONFIG GET. Toasts say Updating; the page shows Restarting during the carrying update. An ordinary command attempted while the container was being replaced failed transiently, then the ready read succeeded; this is the advertised interruption, not a separate bug.
- Journal-mode policy changes preserve the original Unicode string, empty list element, hash newline, expiring string and exact PEXPIRETIME deadline. A pod replacement was recorded for noeviction. One other control's pod-UID read was blank; its successful data/config read does not prove that UID.
- Fresh Journal + Snapshot → Snapshot only at11:58:41.310 is a genuine durable conversion. The11:59:04.094 runtime receipt retains the control strings, hash/list and exact expiry deadline1792238896057, with appendonly=no. This isolates the stale-Off classification from the older stale-AOF runtime defect.
- Fresh Snapshot → Off shows the typed deletion dialog and requires the phrase at11:59:25.606 before accepting Off at11:59:27.496. Reseed only QA data afterward. From a freshly reloaded **Off** view, Journal + Snapshot → Save at12:00:16.001 correctly opens the typed confirmation, disabled without the phrase. Cancel and then row Cancel send zero persistence writes and restore Off. Runtime at12:00:19.287 preserves the new strings/hash/list/deadline and the same pod UID.

## Root cause and bounded fix

- `dashboard/src/features/keyvalue/api/keyvalue.graphql:30–49`: the main **KeyValue** detail document selects status, updatedAt and identity, but omits maxmemoryPolicy and persistenceMode. The actual successful native poll receipts have exactly this shape; neither omitted field appears.
- `hooks/use-key-value.ts:34–66` owns this read and uses the shared converging3s/settled30s cadence. The old comment claiming it stops when settled is outdated; the implementation and observed native polls continue. `routes/keyvalue.$keyValueId.tsx:100,250–262` mounts both editors and uses this hook as their onChanged status refresher.
- `hooks/use-set-key-value-maxmemory-policy.ts:39–70` and `hooks/use-set-key-value-persistence-mode.ts:30–60` each read a focused document with cache-and-network, without polling, then refetch only after a save **in that client**. Their old fields remain cached when the main document refreshes other fields. No missing native focused read was invented: the observers captured the main detail polls; no focused-mode response refreshed A during the recorded interval.
- `components/key-value-persistence-mode-section.tsx:51–74` decides whether to ask for deletion confirmation using the focused hook's mode: `mode === "off" || next === "off"`. The existing pending id/from/to check and confirmationCurrent comparison also depend on that same mode. In A it is a known but old durable value, so neither guard sees Off and requestSave calls save directly.
- `components/key-value-maxmemory-policy-section.tsx:37–87` and `key-value-persistence-mode-section.tsx:107–118` feed those values into the shared EditableFieldRow. That row deliberately keeps an editing draft and follows persisted props outside editing. There is no separate committed-value state to remove. Preserve mounted drafts and dirty/Cancel/focus behavior.
- Backend `lego/backend/internal/keyvalue/graphql.go:50–51,227–240` already exposes both String fields and routes the setters through UpdateKeyValue. `service.go:846–858` writes the supplied normalized mode. `operator/internal/controller/keyvalue_persistence.go:146–153` intentionally discards when source or target is Off. Fresh reads and CONFIG GET confirm the actual Off state, so the backend/operator executed the submitted transition correctly.
- Actual native extensions and dashboard lockfile identify Apollo Client **4.3.3**. Primary package source under `.playwright-mcp/qa-a720-apollo-source/package`: `cache/inmemory/helpers.js:5–20` keys by typename/id; `core/QueryInfo.js:92–132` writes returned query fields; `cache/inmemory/entityStore.js:63–104` merges incoming fields and dirties changed incoming properties. Polling status alone cannot update omitted configuration properties. Common apollo/cache.ts has no KeyValue override. Actual source excerpts/hashes are durable in evidence.json; locally installed4.1.3 is not deployed-code evidence.
- Generated KeyValueQuery has neither configuration field, whereas the two focused query types expose nullable string values. The schema can already express the fix: select both fields in the main document and regenerate its definitions. The UI normalization helpers map known underscore wire values onto hyphen options and must remain unchanged.

**Concrete correction:** include maxmemoryPolicy and persistenceMode in the existing detail polling owner's KeyValueDocument, with offline codegen and realistic query fixtures. Its normalized response can publish both fields to the already-mounted focused observers. An equivalent bounded publication is acceptable, but do not add two independent timers, change global cache policy or infer configuration from status. Keep one detail owner; breadcrumbs remain a secondary observer. The main loader now reads these already-supported nonsensitive fields too; no backend/schema expansion is needed.

**Decision/pre-settle behavior:** a successful detail response carrying off must update the saved-mode baseline and the existing requestSave/confirmationCurrent decisions before a subsequent user action. Preserve an open draft, while classifying its eventual Save against the delivered mode; an arriving changed mode invalidates any already-pending confirmation. Unknown or failed configuration reads must not fabricate a durable default or make an unknown mode writable. Keep explicit load/refusal/retry handling for not-found, forbidden, unauthenticated and transport/timeout errors; do not reinterpret them as Off or successful preservation. Cached last-good display is not evidence that a failed read verified the current transition.

This target is the proven post-successful-poll gap. Simultaneous mutations after the latest delivered read, atomic revision preconditions and a general compare-and-swap API were not tested and are not silently promised by adding fields. t002 must record that boundary. Preserve intentional Off discard and genuine durable conversion; do not repair the UI by disabling Off or changing the operator's data contract.

## Shared scope, aliases and tests

- **Two production useKeyValue callers:** the detail page at routes/keyvalue.$keyValueId.tsx:100 (poll owner) and dashboard-breadcrumbs.tsx:189 (poll:false). The same main document is used by the detail route loader at:76. Exact-word document search gives these two production importer files, plus generated definitions.
- **One production caller/mount for each configuration editor:** its own component calls its focused hook; the detail page mounts each once. Both share the main normalized KeyValue object. KeyValues list reads select other metadata and should preserve, rather than overwrite or fabricate, these options.
- Canonical `/keyvalue/:id`, Overview and its `#maxmemory-policy`/`#persistence-mode` sections share this path. `/r/:id` redirects through common/lib/render-alias.ts. Metrics/Logs tabs use the same resource owner. Source-traced aliases and tabs were not independently live-certified here.
- Web/static/cron/worker/private use Service settings, and Postgres uses separate Database queries; they are outside this producer change. Networking, connection-info, plan purchase and suspend wording stay out of the fix. In particular preserve w4/247's correct mode-independent suspend warning rather than making it depend on a possibly stale field.
- Current persistence component tests mock the focused hook. `components/__tests__/key-value-persistence-mode-section.test.tsx:30–64` explicitly updates hookState.mode to invalidate a confirmation, proving the consumer's check when the new mode reaches it. The wire test also substitutes modeData. Neither tests a real main detail poll that omits the field. Add real Apollo cache/defaultOptions coverage with the actual query documents and section decisions, not another mocked prop-only test.

## Dedupe and original guarantees

Searched all9,693 PM Markdown files including done/blocked, indexed278 non-done READMEs, distinctive query/hook names and configuration/stale/poll terms, and reviewed DO_NOT_DO. Pulled main successfully; last40 product commits and targeted document/hook history contain no main-detail configuration selection or equivalent freshness repair. No open owner or already-shipped deploy-lag fix was found. The existing fields/setters themselves work.

This is a read/decision freshness residual, not recurrence of the stale-AOF conversion mechanism fixed by w4/done/m153. Its full original DoD is disposed as follows; nothing below is a claim that untested old guarantees passed this run:

| Original m153 guarantee | This pass's evidence/disposition |
| --- | --- |
| Journal→Snapshot preserves baseline/counter/deadline and disables AOF | Fresh durable control preserves strings/hash/list/deadline and disables AOF. Original counter fixture and external TLS client were not replayed. |
| New Snapshot writes→fresh Journal conversion preserves those writes and enables AOF | Not replayed as a genuine durable-to-durable control. The mobile failing save actually starts from Off, despite the stale label; do not count it as a regression of this operator guarantee. |
| Stronger Snapshot+SAVE marker/counter23→noeviction restart→fresh Journal retains them | Not replayed. Journal-mode policy restart is a separate passing control and does not recertify this stronger sequence. |
| REST/GraphQL/MCP/UI coherent applied persistence, truthful transition state and wire normalization | GraphQL and runtime agree; local Updating/Restarting and underscore/hyphen controls pass. Untouched UI configuration is stale after a successful poll: this residual is the new scope. REST/MCP writes/reads were not independently replayed in a881. |
| Rename preserves identity/endpoints, then exact deletion/session cleanup | Rename/endpoints not retested. Owned deletion, baseline restoration, exact cluster checks and session revocation pass. |

Other related owners:

- **w6/done/m127, w4/done/066 and w7/done/007** made Free persistence and the two post-create editors available. Current create, native setters and runtime controls pass; this does not reopen their API/default work. They did not implement freshness after another client's save.
- **w4/done/046** normalizes policy underscores; the current local noeviction/volatile-ttl/allkeys-lru controls work. A missing selected field in the ongoing detail poll is a different cause from a blank option.
- **w4/done/m137 and w1/done/m166** own truthful restart state/readiness. Captured status and successful ready runtime controls do not indicate their original endpoint-readiness defect. We did not run their public-TLS cadence gate.
- **w4/done/247** owns suspend wording, which is mode-independent and remains appropriate. This reproduces a persistence **change** without its transition-specific typed confirmation, not a false suspend sentence.
- **w4/blocked/281** is Auto-Deploy's redundant local-state copy; this field follows its prop, but its focused query never receives the updated property. **w4/m225** is service rename response ordering; no old response is delayed here and the missing-property detail document is separate.

[Render's Key Value documentation](https://render.com/docs/key-value) describes policy selection and says Free has no persistence; [its Free documentation](https://render.com/docs/free) warns restart can lose Free data. Bex deliberately supports durable Free storage, as ADR021/w6m127 record. Neither public page establishes authenticated Render cross-client freshness. [Valkey persistence documentation](https://valkey.io/topics/persistence/) describes AOF/RDB; Bex's explicit Off-boundary reset and typed UI confirmation are the local contract exercised here.

## Cleanup, evidence and limits

Native DeleteKeyValue succeeded at12:00:27.924. At12:00:38.505 its REST read returned404 and all seven inventory-ID sets exactly matched baseline. Physical cleanup at12:01:52.761452 found zero owned KeyValue CR, StatefulSet, Service, exact data PVC, TLS certificate/Secret names, and exact keyvalue-label pod/job/CronJob/PVC/Secret/network-policy matches across namespaces. Secret output was restricted to metadata names; no values enter the capture. No foreign or platform mutation was performed.

The pass's own session was revoked; whoami401, cookieszero, one blank page at12:00:44.273 and owned handles removed. Passwords, connection strings and Secret data are absent from the artifact. Both local screenshots were viewed and hashed; they supplement durable DOM/API/data-plane evidence.

Browser network slice:158 nonstatic records after ID9777,150 HTTP200 and8 HTTP204; no failed records. Console errors:zero. Runtime samples are complete parsed CLI command outputs without credentials. The first seed filename was reused for the later desktop Off seed; its earlier baseline is supported by preserved noeviction-ready data and the original values file, not mislabeled as a second original seed capture. The later Off seeds have distinct durable names.

Live limits: one Free/private Valkey8 store, ordinary normal-sized synthetic data, English, desktop/mobile. Public TLS, paid/version7, active TTL expiry, pressure eviction, power loss, backend failures, Blueprint changes and simultaneous post-read writes are not live-certified. No product fix, cybersecurity probe, paid action or external message occurred.
