# Datastore log search loses startup typing on both callers

Why: a usable-looking log search silently clears text entered before its handlers attach.

**Severity:** minor. Functional QA a850, 2026-10-17 UTC, source `4a20949ef`. Extend m219 as its tenth observed root with local gate t015 and shared-caller audit t016. This is filing only.

## Reproduction

Fresh-load Key Value `/keyvalue/{id}?tab=logs` at1440/390, hold the fetched real entry, type into **Search logs…**, then release it unchanged. Repeat mobile. Run the same desktop/mobile probe on Postgres `/databases/{id}?tab=logs`, using its actual label **Search database logs**.

[Exact scripts and complete receipts](../evidence/m219-a850-datastore-search-startup.json) preserve all five failures. Each field is effectively enabled and writable without React props, accepts trusted input, retains it into initial React-props samples, then clears on the same node. Each case has20 matching node samples and an unchanged HTTP200 entry, `index-pJFig2Dm.js`,1,954,259bytes. Ready typing persists and writes the expected `text` URL filter. Separate ready URL controls retain the exact text and show **No matching logs** for both kinds, without console errors.

Viewed supplementary `.playwright-mcp/qa-a850-datastore-{before,after}.png` images contain only the input parent. No datastore log contents or credentials were captured. The first PG probe used the KV label and is excluded; it proves no absence control.

## Cause and target

`dashboard/src/features/logs/components/datastore-log-viewer.tsx:125–126` seeds `text` and `committedText` from URL text. Its single Input at :300–306 is controlled by `text` with no readiness gate. A pre-handler native event cannot update that state; a later controlled update restores its empty seed. Pinned React19.3.1's actual production `updateInput` path was reopened and agrees with that mechanism. The captures do not isolate the exact later render or prove the deployed React version.

Reuse the existing hydration hook to disable this Input natively until ready, preserving its visible geometry. Keep `changeText`'s300ms debounce (:155), URL echo/adoption, `queryFilters` → `useLogHistory` (:190–194), instance discovery, relative-window anchoring, custom ranges, paging, matching and empty/error/refusal states unchanged. Global Input/hook and REST/GraphQL/MCP semantics do not change.

This is one Input site with two production mounts: `routes/keyvalue.$keyValueId.tsx:195` and `routes/databases.$databaseId.tsx:228`. Both were live-driven. Render aliases `/r/{id}` and `/d/{id}` redirect to those respective canonical pages while retaining `?tab=logs` and other suffixes. Audit them in t016; do not claim they were replayed. Service LogFilterBar and DeployLogPanel remain separate earlier roots.

## Dedupe, controls and limits

All86 open/blocked milestone titles (unchanged since a846), distinctive searches across open/blocked/done, DO_NOT_DO, recent product history and targeted viewer history show no startup gate. m219's a838 explicitly left DatastoreLogViewer separate. Preserve228's URL retention,319's commit-time window anchoring and195's honest no-match state. This does not allege those ready-state fixes regressed.

[Render logging documentation](https://render.com/docs/logging) describes search/filter behavior; no authenticated Render startup comparison was run. ADR010/ADR018 and the datastore ADRs govern this existing surface. Other resource contexts, nonempty initial text, range/instance changes, error/paging states and locales remain explicit audit/regression work.

The separate owned Free Rust control reached Live and public200 with the exact27-byte Unicode message; its explicit build/start commands do not prove default-command or companion-toolchain behavior. Native deletion succeeded; service/deploy404, all seven baseline ID sets restored, exact App absent across namespaces, serving/build children gone, public404. Exact-session logout succeeded, whoami401/cookies0/one blank page and own handles removed. Datastore searches made no resource changes; the wider observation window includes only deletion of that owned Rust fixture.
