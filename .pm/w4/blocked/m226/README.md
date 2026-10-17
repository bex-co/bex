# w4 · m226 — Refresh Key Value configuration before classifying persistence changes

**Worker:** worker4 **Goal:** an open Key Value page receives current configuration through its detail refresh, so an Off-to-durable change requires the existing data-deletion confirmation. **Status:** blocked — t001–t005 done 2026-10-17; t006 (owned two-client replay at 1440/390 on a deployed build, then cleanup) open

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Refresh configuration through the existing detail polling owner — **DONE** | 40m | — |
| t002 | Audit both configuration editors and persistence decision state — **DONE** | 30m | w4/m226/t001 |
| t003 | Render parity — **DONE** | 15m | w4/m226/t002 |
| t004 | Simplify — **DONE** | 10m | w4/m226/t003 |
| t005 | Test coverage — **DONE** | 40m | w4/m226/t003 |
| t006 | Closeout | 25m | w4/m226/t004, w4/m226/t005 |

Six tasks, 160m total (2h40m). One shared configuration-read gap: **major** when stale persistence skips the required typed deletion confirmation and new Off-mode keys disappear; **minor** when Maxmemory Policy keeps an older value. Implemented 2026-10-17; awaiting the deployed-build replay (t006).

## Definition of done

Replay [finding.md](finding.md) using fresh owned Free/private Valkey8 stores and ordinary independent dashboard clients, without delaying or replacing any response.

- Desktop: leave client A open on Journal + Snapshot. In B, explicitly confirm the change to Off. After A's next successful native detail poll observes the settled store, its saved persistence value is Off. Write new QA markers while Off is active. A → Snapshot only → Save opens the existing typed `sudo clear key value <name>` confirmation before issuing SetKeyValuePersistenceMode. Cancel sends no mutation and preserves those new markers.
- Fresh390×844: repeat from Snapshot only with target Journal + Snapshot. After the successful detail poll, A receives Off; its attempted change likewise requires the typed confirmation. Cancel preserves the new string/hash/list/expiry data. A confirmed destructive change may clear data, as documented; an apparent durable-to-durable edit must not conceal the actual Off boundary.
- Repeat the policy case: A displays volatile-ttl; B saves allkeys-lru. After the next successful existing detail poll, A displays allkeys-lru (recommended for caches), and Edit/no-op/Cancel use that current baseline without reload. Do not add a separate timer for each editor.
- Preserve the observed controls: normal Cancel/no-op sends no policy write; local policy saves show Updating/Restarting and reach runtime; journal-mode policy restarts preserve values and exact expiry deadlines; a fresh Journal-to-Snapshot conversion preserves the recorded data; a fresh Off view requires the typed confirmation, and Cancel sends no mutation or restart. Keep mounted drafts and the ready/pending editor geometry.
- Complete t002/t005's source/test compatibility work for open drafts/dialogs, unknown modes, errors, resource changes and one polling owner. Relevant dashboard checks pass. Delete each owned store and exact cluster artifacts, restore all seven baseline inventory-ID sets, and revoke only that run's session before closeout.

## Source + Goal linkage

- **Source:** functional qa-find-bugs w4, a881, 2026-10-17 UTC; [finding and source research](finding.md), [complete safe native/API receipts, runtime commands/results, probe sources and cleanup](evidence.json). Research main: b9d3cdf6c3c8bc58b7ea5d6868243744db79b11c. The deployed Git SHA was not established.
- **Goal linkage:** [ADR008](../../../../docs/ADR008-vision.md) dependable hosting and [ADR021](../../../../docs/ADR021-keyvalue-management.md) truthful managed-data persistence. The existing confirmation distinguishes an intentional clear from an ordinary durable conversion.
- **Expected outcome:** current configuration reaches both editors when the page refreshes, and persistence decisions use that delivered mode rather than an old focused-query snapshot.
- **Why now:** desktop and fresh-mobile clients receive successful Available polls but retain the old mode; each then submits an Off-to-durable transition without the typed confirmation, losing newly written QA keys. The ordinary durable-conversion and fresh-Off confirmation controls pass.
- **Scope:** add the two existing configuration fields to the detail read and regenerate its definitions, or an equivalently bounded single-owner publication. Audit both useKeyValue consumers, the route loader and the two configuration hook/component pairs. No operator conversion, global cache policy, networking behavior or API contract expansion is proposed.
- **Render parity included:** configuration visibility and persistence confirmation are tenant-facing. Render's Free Key Value has no persistence; Bex's durable Free tier is an intentional documented divergence. Authenticated Render timing was not tested.

## Limits

Live-tested one Free/private Valkey8 store, canonical route, English UI, desktop and mobile independent clients. REST/MCP writes, public TLS, paid tiers, Valkey7, failures, Blueprint changes and simultaneous changes after the latest delivered poll were not exercised. This filing repairs the proven stale state after successful detail polling; it does not claim atomic compare-and-swap semantics for concurrent settings writes.

## Implementation record (2026-10-17)

- **t001:** the polled KeyValue detail document now selects `maxmemoryPolicy` and `persistenceMode` (offline codegen from the backend schema dump). The existing detail poll owner (`useKeyValue`, converging 3s / settled 30s, visible tabs) therefore publishes both to the normalized `KeyValue:<id>` entry that the two editors' focused queries observe. No new timer, global policy or backend change. The persistence editor's existing `requestSave`/`confirmationCurrent` decisions now see a mode another client set to Off, so a change to a durable mode asks for the typed `sudo clear key value <name>` confirmation; the policy editor edits the current policy. Because the poll now carries configuration, a save first settles any in-flight detail read for that store (`settleKeyValueDetailReads`) and then re-asserts its saved value from the mutation result, so a poll sent before the write cannot restore the old mode. A confirmation opened against a mode that has since changed is discarded rather than reopening if the mode returns. Unknown or failed reads keep today's blank/disabled handling.
- **t002 census/boundary:** `useKeyValue` has two production callers (the detail page, the poll owner; breadcrumbs with `poll:false`) and the route loader also reads the document; each editor has one mount on the detail page (`/keyvalue/$id`, `/r/$id` alias). Open drafts keep their value and are classified against the delivered mode on Save. Limit: this is not a compare-and-swap; a change made after the latest delivered read is still classified against it (recorded in ADR021). Follow-up candidate: the editors could read both fields from the detail view and drop their focused queries; not done here because it touches many mocked tests.
- **t003:** ADR021 records the refresh, the save ordering and the compare-and-swap limit.
- **t004:** three /simplify passes (in-flight read settling and value re-assertion after saves, void confirmations cleared, stale query comments fixed).
- **t005:** `key-value-config-refresh.test.tsx` mounts the real poll owner beside both real editors on a real Apollo client (app cache/defaults, fake network and clock). After another client sets Off and allkeys-lru, the next poll shows both, and Snapshot only then opens the typed confirmation with no write sent; it fails against the old document. A detail poll sent before a policy save and released after it does not revert the saved policy; it fails without the settle step. `key-value-persistence-mode-section.test.tsx` covers a voided confirmation not reopening. Existing wire/hook tests gained a client stub. Dashboard suite (5168 tests), lint and typecheck pass.
