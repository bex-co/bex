# w4 · m225 — Publish a saved service name after older detail reads

**Worker:** worker4 **Goal:** a completed service rename keeps the authoritative saved label visible after an older detail response arrives. **Status:** blocked — t001–t005 done 2026-10-17; t006 (owned overlap replay on a deployed build at 1440/390, then cleanup) open

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Publish the service name after pre-write reads settle — **DONE** | 45m | — |
| t002 | Audit shared readers, route aliases and existing ordering controls — **DONE** | 25m | w4/m225/t001 |
| t003 | Render parity — **DONE** | 15m | w4/m225/t002 |
| t004 | Simplify — **DONE** | 10m | w4/m225/t003 |
| t005 | Test coverage — **DONE** | 35m | w4/m225/t003 |
| t006 | Closeout | 20m | w4/m225/t004, w4/m225/t005 |

Six tasks, 150m total (2h30m). Minor functional display defect: the rename commits correctly; a later poll or reload recovers the label. No persisted rollback or runtime interruption was observed. Implemented 2026-10-17; awaiting the deployed-build replay (t006).

## Definition of done

Repeat the owned Free web journey and browser-local response-delay probe in [finding.md](finding.md), starting from fresh 1440×1000 and 390×844 Settings documents. The probe delays one real successful pre-write Server response without changing its data.

- Save a distinct Service Name while that response is held. Release it after SetDisplayName succeeds. After publication completes, the Service Name field, header, sidebar/breadcrumb and document title show the label returned by the final authoritative post-write read, without reloading. The older label must not be presented as the completed saved state. Capture the actual overlapping request and success; a replay without overlap does not close this gate.
- Repeat with another distinct label. Preserve the recorded normal controls: Cancel and exact-value restoration send no rename; a padded ordinary Unicode label reaches the wire trimmed; a 100-code-point emoji label survives save and fresh mobile reload; clearing returns the original name. Global search still finds the renamed resource by the new label, immutable original name and typed ID, and the home link still opens the same service.
- Renames retain the immutable ID, original name, slug, public URL, running marker/process start time, revision and single original deploy. No rebuild, restart or deployment is introduced by label publication.
- Complete t002/t005's source-traced compatibility work: overlapping Services reads, multiple detail observers, failed refresh and read-only retry, newer authoritative edits and resource/workspace changes. These are required implementation/test controls, not additional live failures claimed by a879. Keep mounted drafts and pending/ready geometry stable.
- Required dashboard checks and the two-breakpoint live replay pass. Delete each owned fixture and its exact cluster objects, restore the seven starting inventory-ID sets, and revoke only that run's QA session before closeout.

## Source + Goal linkage

- **Source:** continuous functional qa-find-bugs w4, a879, 2026-10-17 UTC. [Reproduction, research and dedupe](finding.md), [durable exact mutation/narrow-read receipts, UI observations, probe sources and cleanup](evidence.json). Research main: 46084c87396de84b1c0ee1b94ce8abbb2691bb41. No deployed Git SHA was established.
- **Goal linkage:** [ADR008](../../../../docs/ADR008-vision.md) dependable hosting, [ADR006](../../../../docs/ADR006-bex-api.md) truthful service projections, and [ADR020](../../../../docs/ADR020-identifiers.md) stable resource identity during human-label changes.
- **Expected outcome:** after a rename, users see the latest authoritative label instead of an older successful read's label next to a success toast naming the new one.
- **Why now:** four successful overlapping-read replays, including fresh desktop/mobile captures, isolate the missing publication ordering. Ordinary saves, label validation, search aliases and runtime controls pass.
- **Scope:** allowlist the service-name write completion path. Audit its one production hook caller and shared Settings mount, 15 useServer consumers and nine useServices consumers. Preserve existing cron/project ordering and unrelated setting writers; no global cache, polling or API policy change is proposed.
- **Render parity included:** a tenant-facing rename completion changes. Render documents PATCH service.name; authenticated Render response ordering was not tested. The backend already accepts and returns the intended label; REST/MCP writes were not live exercised in this pass.

## Limits

Only an owned Free image web service was live renamed. Static, worker, private, cron, datastore names, Chinese copy, foreign edits, backend refusal classes and refresh failures were not live probed. The old-response delivery was controlled locally; this is not a claim of naturally slow production traffic. No cybersecurity work, paid fixture, external notification or product implementation is part of this filing.

## Implementation record (2026-10-17)

- **t001:** `useDisplayName` publishes after `SetDisplayName` commits, in the project rename's order (w4/m186). It settles every in-flight Server read for this service and every in-flight Services list (`refetchQueries` with an `onQueryUpdated` filter, each result `allSettled`), then issues one network-only, `queryDeduplication: false` Server read, and only then calls `router.invalidate()` and toasts success. `busy` holds the editing row and draft until then. A failed re-read is a warning with a read-only Try again (never a second rename); until a retry publishes, the row is locked (`disabled` with a reason) rather than offering edits over a stale label. A cleared label's warning names the immutable name from the mutation result, not the id. If the row unmounted (the customer left), publication still refreshes the cache but toasts and invalidates nothing on the page they moved to; Settings now keys the row by service. Settings' own immediate `router.invalidate()` and the row's `onChanged` were removed as redundant.
- **t002 census/disposition:** one `useDisplayName` caller (DisplayNameRow), one mount (ServiceSettingsPage, also `/static/$id/settings`; `/web`, `/pserv`, `/worker`, `/cron`, `/static` aliases redirect there). Server and Services are the two documents selecting `Service.displayName`; idle observers of either take the label from the normalized mutation result, in-flight ones are settled. The service-detail loader's breadcrumb warming reads are one-shot `client.query` calls made on navigation, not observers, so they cannot be awaited without a global change; they only race a rename when a navigation coincides with it, and the next poll corrects them. SyncBlueprint's authoritative name updates are untouched. Postgres/Key Value renames have their own hooks. Mutation refusals keep their existing error toast and draft.
- **t003:** ADR018 records the publication order and read-only retry; the API is unchanged.
- **t004:** three /simplify passes (refetch narrowed to in-flight reads, unmount guard, unpublished lock, keyed row, name fallback, tighter tests). A shared "settle, then read" helper was deliberately not extracted: five call sites (project rename, cron, environments, move-to-project, this) differ in filters and side effects, and the finding allows extraction only with their audits. It is a candidate for a dedicated dedupe pass.
- **t005:** `use-display-name-publish.test.tsx` runs the real hook on a real Apollo client with the app's cache and defaults; only the network is faked, and Server responses are filled from the query document. An older Server and Services read held across the write keeps the rename busy, with no authoritative read yet, until released; the saved label then wins on both documents with exactly one more Server read. It fails against the old hook and with the settle step removed. Also covered: an idle rename makes one read and refetches no list; a failed re-read warns, locks the row and recovers by Try again with a single rename; an unmounted row toasts and invalidates nothing. The hook unit and row tests are adapted. Dashboard suite (5165 tests), lint and typecheck pass.
