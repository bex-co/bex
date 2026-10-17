# Deployment-history search loses pre-handler typing

Why: a usable-looking history search accepts text before startup finishes and silently clears it as the controlled page initializes.

**Severity:** minor. QA a841, 2026-10-17 UTC. Extend m219 through t013; research main e36b116e5dca0a1f757c53f640ad01cf8e337181.

Fresh-load an owned populated /services/{id}/deploys at1440×844 or390×844. Fetch and hold the real startup entry, type through the browser keyboard, release it unchanged, and sample20 times. [Durable evidence](../evidence/m219-a841-history-search-startup.json) contains exact scripts and complete receipts for desktop/mobile/repeated-mobile, source census, ready controls and cleanup.

All three captures expose one enabled Search loaded deploys and commits field without React props. It accepts the trusted marker, then becomes empty on the same node; all20 node-identity samples agree. Every fetched entry is HTTP200/1,952,882 bytes. The first sample with React props is already blank: the150ms sampling does not isolate the precise clearing step. No particular query or deployed React version is claimed.

The repeated case has a settled Live deploy and no console error. Ready typing remains after an additional1.2s wait; q=qa-a841-history-repeat-ready appears in the URL and the page truthfully shows No loaded deploys match your search. Both .playwright-mcp/qa-a841-history-{before,after}.png images were inspected. Metadata/row loading differences around the search are separate from the unchanged input node.

**Target:** keep the search visibly in place but natively uneditable until handlers attach. Ready editing, validated URL filters, loaded-only matching/counts and status filtering keep their current behavior.

## Located cause and scope

features/deploys/components/deploys-list-page.tsx:121 seeds localFilters from URL filters. Its one Input site at :368–376 binds search and calls changeFilters, without a readiness gate. A native event before handlers cannot update that local state or URL. The controlled binding restores the empty URL seed during startup. The pinned React path already documented for a837 is consistent; this pass does not identify an exact update call.

Reuse the local hydration gate on this Input only. DeploysListPage has two production JSX mounts: routes/services.$serviceId.deploys.index.tsx:27 and static.$serviceId.deploys.index.tsx:29. The static route is intentionally reachable without a sidebar tab. Audit both adapters, compute-type callers and legacy Render prefix redirects, preserving suffix/query behavior. Web was live-probed; private/worker/cron and static are source-scoped. Postgres/Key Value do not use this history component.

Do not change global Input, useIsHydrated, local/URL echo handling, loaded matching, pagination, row actions or REST/GraphQL/MCP semantics. Preserve the already-landed05a7c4e3a fix for284's ordinary list → detail → Back filter retention. That after-handler journey was not replayed here and is not alleged regressed.

## Controls, ownership and verification

Qualified controls in the artifact: a839's Project-create Name and a840's member search have no input before startup at either width, then retain ready typing. Project Cancel/reopen resets its draft; ready member search has an honest no-match state and clears back to its table. No writes were sent by these controls. Absence before startup does not prove pre-handler retention or all conditions safe.

Distinctive open/blocked/done searches, all84 open/blocked milestone titles, DO_NOT_DO, recent40 product history and targeted component history show no history-search readiness fix. m219 is the existing owner; m204, m216 and284 address different post-handler states. [Render logging docs](https://render.com/docs/logging) describe individual deploy logs, not authenticated startup timing. No signed-in Render startup comparison was made.

**Unverified:** nonempty initial URL filters, other service types/aliases, paging/error states, Chinese and reduced motion. Test those relevant siblings without treating them as observed failures.

Startup probes create nothing. The separately created Free web fixture has one observed Live REST deploy; no pre-delete public200 check is claimed. Service/deploy now404, all seven baseline ID sets match, App/serving/build children are gone, public URL404, exact-session logout confirmed, whoami401/cookies0/one blank page and own handles removed. Probe handlers are removed in finally.
