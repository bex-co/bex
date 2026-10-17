# Populated webhook search drops typing entered before handlers attach

Why: a usable-looking search field accepts text during startup and silently clears that intent once the controlled component updates.

**Severity:** minor. Functional QA a837, 2026-10-17 UTC. This adds a fifth local root to m219; its first four gates already landed in66b041df3. No product change is made by this filing. Research refreshed at bbe46c69a1042ac81d32734f668b3723719d881a.

## Exact reproduction and evidence

With one owned disabled endpoint listed, freshly open /webhooks at1440×844 or390×844. Fetch and hold the real startup entry response, then type into Search webhooks through the browser keyboard. Release the unchanged response and sample the input for about three seconds.

[Durable evidence](../../evidence/m219-a837-webhook-search-startup.json) contains exact scripts, three full receipt sets, native fixture lifecycle, source census and cleanup. Each fetched entry is HTTP200 and1,952,882 bytes at /assets/index-D60ymwlM.js; neither code nor API responses are rewritten.

| Fresh replay | Before handlers | After normal startup | Ready control |
| --- | --- | --- | --- |
| 1440px | enabled; no React props; trusted qa-a837-filter-1440 | empty on the same node | qa-a837-filter-1440-ready retained |
| 390px | enabled; no React props; trusted qa-a837-filter-390 | empty on the same node | qa-a837-filter-390-ready retained |
| repeated390px | enabled; no React props; trusted qa-a837-repeat | empty on the same node | qa-a837-repeat-ready retained |

In each replay the node initially retains text even after React props appear, then a later controlled update clears it; all20 samples retain node identity. The repeated mobile before/after screenshots were inspected: .playwright-mcp/qa-a837-webhook-search-{before,after}.png. They also show Add webhook appearing after permission loading. That separate layout change does not explain the unchanged input node or broaden this finding.

## Located cause and local target

At current main, features/webhooks/components/webhooks-panel.tsx:42 initializes search to empty. The Input at :118–124 binds that state and onChange without a startup gate. It is exposed for a populated list, including SSR; empty/error/loading branches are separate. There is one production WebhooksPanel mount, routes/webhooks.tsx:45, and one search Input site in this component.

Reuse useIsHydrated locally and disable just this native search field until handlers are ready, matching EnvGroupsPage's existing m219 gate. Keep labels, help, layout, ready filtering/toggles/links and list branches unchanged. Do not gate all of WebhooksPanel or the global Input component, or change the hook's snapshots. The unrelated webhook create page remains a805's passing control.

The actual lockfile pins react-dom19.3.1. The previously downloaded official npm archive was reopened: production initInput at1315/1343 preserves a DOM value during hydration; updateInput at1269–1293 writes a differing controlled value later. This agrees with the samples; no particular later query is blamed. Local node_modules is19.2.3 and was not substituted for the pinned source. React's deployed version was not independently extracted from the entry asset.

Current source has nine named production useIsHydrated call expressions in eight files, excluding its definition. Keep all unchanged; adding this one panel call makes ten in nine files. The native-field census from a815 remains a source audit, not proof that all other fields are affected. /webhooks is the observed list route; its create/detail/Settings routes are distinct. No service/datastore runtime or REST/GraphQL/MCP field changes are required.

## Ownership, checks and limits

Distinctive open/blocked/done searches, all84 actual open/blocked milestone titles, DO_NOT_DO, recent40 product commits and targeted WebhooksPanel history found no local gate.66b041df3 fixes the other four roots; bbe46c69a changes webhook loading geometry, without adding search readiness. This gap is not deployment lag for either fix. Extend m219 rather than create another issue; m220's pending geometry and333's project filter semantics remain separate.

[Render webhook documentation](https://render.com/docs/webhooks) describes configuration, not authenticated startup edit timing. No signed-in Render comparison was run. The existing m219 input-retention contract supplies the target.

**Unverified:** empty/error branches, workspace changes, Chinese, other roles and other SSR search inputs. Ready controls prove retained input text; no separate ready filtered-result DOM capture was taken. Verify those relevant sibling behaviors in implementation tests without presenting them as observed DoD claims.

The startup probes themselves send no create/deploy action. Supporting Free receiver srv-db9eid4fcf2c73c2mpkg and disabled endpoint whk-db9eiik50quc73dl5ou0 were created and deleted separately; the one-time secret is omitted from the safe create receipt. Their service/deploy/endpoint/history return404, all seven inventory ID sets match, the App across namespaces and serving/build children are gone, and the public URL returns404. Exact-session logout succeeded, whoami401/cookies0/one blank page; all handlers and own handles are removed.
