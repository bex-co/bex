# a815 — Environment Groups search accepts text before startup, then clears it

**Severity:** minor for this search field (lost filter intent); m219's creation-draft loss remains major. **Observed:** 2026-10-17 00:48–00:54 UTC. **Research HEAD:** `5e18f96cf7d8359b4d77c64893397cd879586f77`. This extends the existing mechanism and milestone, rather than filing another bug owner.

## Reproduce and evidence

Fresh-load `https://dashboard.bex.co/env-groups` at1440px and390px. Hold only the already fetched, real HTTP200 startup entry `https://dashboard.bex.co/assets/index-D60ymwlM.js` (1,952,882 bytes), using the exact factory in [durable evidence](../evidence/m219-a815-search-startup.json). The enabled, writable **Search environment groups by name** input is present before React props/handlers. Focus and insert the synthetic marker with native keyboard input, then release the original script response unchanged.

| Capture | Width | Accepted trusted text | After startup | Ready control |
| --- | --- | --- | --- | --- |
| 00:48:50.124Z | 1440 | `qa-a815-filter` | empty | `qa-a815-filter-ready` retained |
| 00:50:22.891Z | 390 | `qa-a815-mobile` | empty | `qa-a815-mobile-ready` retained |
| 00:54:25.178Z | 1440 | `qa-a815-screenshot` | empty | `qa-a815-screenshot-ready` retained |

Each input event is trusted and occurs without React props. All 20 sampled nodes are the original node; its value later becomes empty. There is no replacement input or synthetic input event. After handlers are ready, typing persists; the screenshot replay's complete accessibility tree shows **No matching environment groups**, the exact query and Reset search controls. The third capture repeats desktop for screenshots; it is not another independent surface. Supplementary, inspected screenshots: `.playwright-mcp/qa-a815-search-typed.png` and `qa-a815-search-cleared.png`. The durable artifact contains all eight full DOM/control/cleanup receipts and the exact probe code. No API mutation or resource creation was submitted. Final inventory matches all seven baseline resource families, no console errors/HTTP failures were recorded, and this run's session ends with whoami401, zero cookies and one about:blank page.

**Expected:** search must not accept text that it cannot retain. Keep its geometry visible but uneditable through SSR/initial hydration; enable it once handlers can preserve native typing. Ready filtering and Reset search keep their current semantics. This does not claim every normal-speed visit loses input.

## Source, mechanism and scoped fix

- `dashboard/src/routes/env-groups.tsx:47–73` prefetches groups, services and scope data. The populated list/search can therefore be server rendered before the entry script executes.
- `env-groups.tsx:90–97` initializes workspace-scoped controlled search to an empty value. `:172–178` passes that state into Input and its onChange without a readiness gate. A pre-handler native event cannot update this component state.
- `dashboard/src/common/components/ui/input.tsx:5` is a native input passthrough. The actual pinned React controlled-input implementation was opened for [a797](finding.md): hydration preserves the DOM value, while a later ordinary controlled update writes the empty state. The same-node sequence agrees with that mechanism. No claim independently identifies the deployed React version or pins the update to one query response.
- Add the existing `useIsHydrated` readiness check locally to **EnvGroupsPage's search input**, alongside the three already scoped creation roots. Preserve the workspace-id reset policy, literal trimmed case-insensitive matching, Reset controls, initial/loading/error/empty states, and immediate usability during client-side navigation. No global Input/Textarea rewrite, new readiness framework, API shape change or authorization-policy change is required.

## Blast radius and passing comparisons

An import-resolving TypeScript JSX AST census of production TSX (excluding tests) finds **103 canonical Input sites in59files, 8 Textarea sites in8files, 12 native input sites in7files and 2 native textarea sites in2files: 125 direct syntactic sites**, with zero unresolved named Input/Textarea imports. These are not runtime input counts. Wrapper callers must be expanded separately, then classified as SSR-editable, gated/client-only, readonly or uncontrolled before proposing broader work. The complete path/line/prop-name census is durable. The seven existing `useIsHydrated` calls in six files remain part of t004's regression audit. Shared primitives stay unchanged; the observed fix is allowlisted to four roots.

Project comparison at1440px: `/project/prj-d9dgeo0bd9nc73a0vh1g?env=unassigned` has no accessible **Search resources in Unassigned** field during this captured startup hold or the short post-release sample. This is absence evidence, not proof of retained pre-handler input. A separate later ready capture types `qa-a815-project-ready`, retains it and publishes the matching URL query. The project loader preloads project metadata; `environments-panel.tsx` waits for resource lists before that card. Its ProjectResourceFilters component has two production mounts (`environment-card.tsx:333`, `environments-panel.tsx:310`). Other project contexts and mobile remain unverified. Preserve w4/done/266's separate synchronous URL-filter draft fix; its after-handler event/route timing defect is not this startup trigger. The Postgres and webhook controls from a800/a805 also remain outside the local readiness changes.

The list route is `/env-groups`; a bounded route-source search finds detail back-links to it and no separate list alias. The create dialog mounted on the list/detail route is a separate audit surface. Additional search fields, wrappers, aliases, widths, source tabs, client-side navigation and nondefault languages are not claimed as observed failures. Existing catalog recovery, validation, read errors, timeouts and session/permission handling retain their current behavior; m218 still owns catalog Retry.

## Dedupe, parity and work

Re-read DO_NOT_DO, searched open/blocked/done notes and scanned **82 actual open/blocked milestone titles** (excluding evidence and done directories). m219 already owns pre-handler controlled-input loss and is extended. w4/done/266 concerns Project filter changes after handlers attach; w5/done/m41 concerns populated Project overview parity; w4/done/265 concerns palette lifecycle. Other search hits concern environment-group copy or dotenv parsing. None owns this list-search startup window. The latest40 product commits and targeted EnvGroupsPage/ProjectResourceFilters history contain no already-landed readiness fix; this is not filed as deployment lag.

[Render's primary environment-variable documentation](https://render.com/docs/configure-environment-variables#environment-groups), checked 2026-10-17, documents the Environment Groups list and management, but does not specify list-search hydration timing. No authenticated Render startup comparison is claimed. ADR008/ADR018's dependable hosting dashboard and ADR032's group organization support retaining accepted search intent; no anti-goal excludes this fix.

Add t009 (15m), enlarge t004's caller audit to30m, parity to20m and meaningful staged tests to45m. Final milestone: **9tasks, 3h30**. This filing supplies evidence and researched work only; no product fix or live closure is claimed.
