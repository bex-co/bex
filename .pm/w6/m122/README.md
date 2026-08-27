# w6 · m122 — The Events tab silently hides five event types the API emits, because "all types" means the dashboard's catalog rather than what the feed returns

**Worker:** worker6 **Goal:** the Activity feed renders every event the API returns, and a future backend vocabulary addition cannot silently vanish from it **Status:** todo

## Tasks (in order)

| id   | title                                                                                     | est | depends_on |
| ---- | ----------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Make the Events tab fail-open: the catalog governs grouping and labels, not visibility      | 45m | —          |
| t002 | Catalogue the five missing types — groups, labels, icons, en + zh                           | 45m | t001       |
| t003 | Cross-boundary drift guard: backend vocabulary ⊆ dashboard catalog, enumerated from Go      | 45m | t002       |
| t004 | Render parity                                                                               | 20m | t003       |
| t005 | Simplify                                                                                    | 20m | t004       |
| t006 | Test coverage                                                                               | 30m | t004       |
| t007 | Closeout                                                                                    | 10m | t005, t006 |

## Definition of done

- **The hidden event becomes visible.** On `https://dashboard.bex.co/services/srv-da7o6ovvqdcc73bpn9hg/events`, the `custom_domain_verified` event stamped `2026-08-27T11:54:07Z` renders in the Activity feed, positioned between the two `custom_domain_added` rows at `11:54:08Z` and `11:52:55Z` that already render today. (If retention has aged it out by then, generate a fresh one: add a throwaway custom domain and let ownership verify.)
- **It becomes selectable.** "Filter events" lists "Custom domain verified" as an option, and lists the four `disk_*` types. This run's live capture of that control returned exactly three domain options — "Custom domain added", "Custom domain removed", "Platform subdomain updated" — and zero disk options, out of 62 checkboxes total.
- **The fix survives the NEXT vocabulary addition, not just today's five.** An event type present in the API response but absent from the catalog still renders, carrying the generic `services.eventsTypeServiceChanged` label. Verify by feeding the feed a synthetic unknown type — this is the bullet that separates a real fix from one that merely clears the current backlog.
- **Drift fails the build.** `cd dashboard && yarn test` goes red when a type is added to the backend vocabulary without reaching the catalog. Demonstrate it by adding a throwaway constant and watching the test fail, rather than asserting it passes.
- **The count badge counts the feed, not the intersection.** The badge beside the Activity title reflects what the API returned for the window. It is `visibleEvents.length` today (`services.$serviceId.events.tsx:122-126`). _Note: this one is derived from reading the code — the two totals captured this run came from different time windows, so the undercount was not measured live. Confirm it before and after._
- **The metrics event-markers surface is confirmed unaffected**, not assumed — `timeline.ts` is fail-open, so it should already be correct; open it and check.
- **Select-all still means all.** After the fix, the filter's select-all control admits the newly visible types rather than re-hiding them.

## Source + Goal linkage

- **Source:** live `/qa-find-bugs` hunt of `dashboard.bex.co`, **56th run**, 2026-08-27, journey 3/5 (service events). Workspace `tea-d98210cbbpdc73dcrkvg`, service `qa-20260826-webhook-renamed` (`srv-da7o6ovvqdcc73bpn9hg`). Read-only apart from reading a pre-existing `qa-` service.

  REST `GET /v1/services/srv-da7o6ovvqdcc73bpn9hg/events?limit=100&startTime=<now-72h>` returned 39 events:

  ```
  custom_domain_removed 6 · custom_domain_added 7 · custom_domain_verified 1 · deploy_ended 5
  build_ended 5 · build_started 5 · deploy_started 5 · service_environment_changed 3
  server_failed 1 · display_name_changed 1
  ```

  The single `custom_domain_verified` is stamped `2026-08-27T11:54:07Z`. **Its immediate neighbours both render on the page** — `custom_domain_added @11:54:08Z` and `@11:52:55Z`, shown as "Custom domain added / 7h". That is what rules out the innocent explanation that the UI simply queried a narrower window.

  Then, from the live "Filter events" control, read out of the accessibility tree: 62 checkbox options, of which the domain-related ones are exactly `"Custom domain added"`, `"Custom domain removed"`, `"Platform subdomain updated"`. No "Custom domain verified"; no disk options at all. So the type is not merely unrendered — it is **unselectable**, and no user action can reveal it.

- **Root cause — two lines, one file.** `dashboard/src/routes/services.$serviceId.events.tsx:82-84` seeds the default selection from the catalog:

  ```ts
  const [selectedTypes, setSelectedTypes] = useState<Set<string>>(
    () => new Set(SERVICE_EVENT_TYPES),
  );
  ```

  and `:106-108` renders the intersection:

  ```ts
  const visibleEvents = events.filter((event) => selectedTypes.has(event.type ?? ""));
  ```

  `SERVICE_EVENT_TYPES` (`dashboard/src/features/events/service-event-catalog.ts:90-92`) is derived from `SERVICE_EVENT_GROUPS`. The filter is therefore **fail-closed**: a type the backend emits but the catalog omits is invisible *and* absent from the control that would let you re-enable it.

- **Exhaustive diff — five types, counted.** `lego/backend/internal/events/service.go` declares 60 `Type… = "…"` constants; `SERVICE_EVENT_GROUPS` lists 59 strings, of which 4 are group `key:` values rather than types. Emitted by the backend, absent from the catalog groups **and** from `LABEL_KEYS`:

  ```
  custom_domain_verified   <- verified live, above
  disk_attached  disk_detached  disk_restored  disk_updated
  ```

  All five are genuinely reachable, not dead constants — `eventTypes` (`service.go:245`) maps audit verbs onto them and `service.go:655` does `ev.Type = eventTypes[r.Verb]`:

  ```
  service.go:281  "apps.VerifyDomain"        -> TypeCustomDomainVerified
  service.go:275  "apps.AddDisk"             -> TypeDiskAttached
  service.go:277  "apps.DeleteDisk"          -> TypeDiskDetached
  service.go:276  "apps.UpdateDisk"          -> TypeDiskUpdated
  service.go:278  "apps.RestoreDiskSnapshot" -> TypeDiskRestored
  ```

- **The target behaviour already exists one file over — this is the control.** The same codebase filters the same event stream twice with opposite failure modes. `dashboard/src/features/events/lib/timeline.ts:20-41` (the timeline component, and via `features/metrics/lib/chart-events.ts` the metrics event-markers) is fail-**open**: `if (filter === "all") return true;`, and its final branch returns true for anything in neither `DEPLOY_TYPES` nor `LIFECYCLE_TYPES`. An unrecognised type survives every branch. So "render everything; let the catalog govern grouping and labels" is already the house style and the Events tab is the outlier — which is also the strongest evidence for **which** side is correct, rather than a bare "make them consistent".

  Also already fail-open: `serviceEventLabelKey` (`service-event-catalog.ts:153-155`) is `LABEL_KEYS[type] ?? "services.eventsTypeServiceChanged"`. The label layer anticipated the unknown type; only the visibility filter did not.

- **Why three guards missed it — none crosses the Go/TypeScript boundary:**
  - `scripts/events-verify.sh` — a backend E2E against the CAPD cluster asserting correspondence/ordering/cursor/redaction, with a hardcoded type list at `:302`. It never opens the dashboard; grepping it for `dashboard`, `catalog`, `SERVICE_EVENT` returns nothing.
  - `TestEventSurfaceParity` (`lego/backend/internal/api/events_surface_test.go:116`) — holds REST/GraphQL/MCP to one `Service.List`. Three surfaces, all backend.
  - `dashboard/src/features/events/components/__tests__/service-event-filter.test.tsx:9` — `useState(() => new Set(SERVICE_EVENT_TYPES))`. **Self-referential:** its notion of "all types" *is* the catalog, so it can never notice the catalog is short. It asserts the group checkboxes render, which they do.

  So the feed has three guards and its fourth surface has none. `git log -6 -- dashboard/src/features/events/` shows the last change is `afe614f1` (w7/m66) — the catalog has not been touched since, which is exactly how types added afterwards drifted in.

- **Precedent — extend, do not re-litigate.** `.pm/w7/done/m66/` built this catalog. Its DoD item 5 reads *"**Every new type** renders end to end: it appears in the dashboard Events filter under the right group, with an icon, an i18n label (en + zh)…"*. m66 satisfied that for **its own** types; the wording scoped the guarantee to "every new type" rather than "every emitted type", and nothing enforces it for types added later. This is a drift-after-m66 gap, **not** a regression of m66's shipped work.

- **Goal linkage:** [docs/ADR006-bex-api.md](../../../docs/ADR006-bex-api.md) and the events package doc, which calls the feed a *"TRUTHFUL 1:1 record of what happened to a service"* (`scripts/events-verify.sh` header). A UI that intersects that record with a stale client-side list breaks the property the backend goes to considerable lengths to guarantee.

- **Expected outcome:** the Events tab shows every event the API returns, and the next vocabulary addition cannot silently disappear from it.

- **Why now:** `custom_domain_verified` is the event telling a user their domain-ownership check passed — the most-awaited signal in the ADR005 custom-domain journey — and it is invisible on the surface built to show it. The four disk types make the entire persistent-disk lifecycle ([ADR082](../../../docs/ADR082-persistent-disks.md)) invisible in the activity feed. Both are surfaces users check precisely when something has gone wrong.

- **Render parity:** included (t004). Render's Events page shows the types its API emits, so this restores parity rather than adding a divergence. Confirm REST/GraphQL/MCP need **no** change — they already return these types correctly, the defect is purely dashboard-side — and record that finding in the task rather than assuming it.

- **Blast radius:** `SERVICE_EVENT_TYPES` has exactly **3** consumers — `services.$serviceId.events.tsx:83` (default selection), `service-event-filter.tsx:15,52,68,87` (option list, tri-state, select-all), and `__tests__/service-event-filter.test.tsx:9`. The select-all at `service-event-filter.tsx:87` (`new Set(SERVICE_EVENT_TYPES)`) carries the same fail-closed assumption and must move with the fix, or "select all" will re-hide the newly visible types. `filterTimelineEvents` is a **separate** filter and is already correct — do not "unify" them without checking, since one is fail-open by design.

- **Adjacent classes:** an event with a missing/empty type (`event.type ?? ""`) must not become visible-but-unlabelled noise — decide where it lands; an event outside the queried window stays excluded (that is `timeline.ts`'s window check, and it is correct); and a type the backend *removes* must not break the catalog, so the guard asserts backend ⊆ catalog, **not** equality.

- **Unverified this run — carried as work, not presented as observation:** the four `disk_*` types were confirmed *reachable* by reading `eventTypes` (`service.go:275-278`) but **none was observed live**, because the QA workspace has no service with a persistent disk; the count-badge undercount is code-derived only (see the DoD note); and only the `en` locale was grepped — whether `zh` carries any of the five strings was not checked.
