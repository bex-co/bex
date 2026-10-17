# a797 — Startup discards input accepted by three SSR create forms

**Severity:** major (silent draft loss). Reproduced under browser-local startup latency; this does not claim every ordinary load loses input. **Observed:** 2026-10-16 23:14–23:18 UTC. **Research HEAD:** `00f2afcfb44a3626ef4c2117931d82affaf2f186`.

## Reproduction and evidence

Use an authorized QA session, freshly navigate to a URL below, and hold only `https://dashboard.bex.co/assets/index-D60ymwlM.js` after fetching the real response. The response is HTTP 200, 1,952,882 bytes, SHA256 `468297ce6781d04add5c491167d1ee56b28f9f1888a756f445e68056968afba8`. Do not rewrite the JavaScript or API responses. Wait until the response is captured and the native input is visible, enabled and has no React props. Focus it, select any existing text, and type through the browser keyboard. Release the original script response unchanged and sample the same input every 150ms for about three seconds.

| Receipt | Width | Field | Accepted text | Later value |
| --- | --- | --- | --- | --- |
| a797KeyValueHeldDesktop | 1440 | Name | qa-a797-kv-hold | empty |
| a797KeyValueHeldMobile | 390 | Name | qa-a797-kv-second | empty |
| a797ServiceHeldDesktop | 1440 | Name | qa-a797-service | empty |
| a797ServiceHeldMobile | 390 | Name | qa-a797-svc-mobile | empty |
| a797BlueprintHeldDesktop | 1440 | Blueprint Name | qa-a797-blueprint | empty |
| a797BlueprintHeldMobile | 390 | Blueprint Name | qa-a797-bp-mobile | empty |
| a797BlueprintPathHeld | 1440 | Blueprint path | examples/hello-go/render.yaml | render.yaml |
| a797ServicePortHeld | 1440 | Port | 8197 | 3000 |

URLs are `/keyvalue/new`, `/services/new?type=web_service` and `/blueprints/new` on `https://dashboard.bex.co`. Every primary receipt has `delayApplied:true`, a trusted native input event with `reactProps:false`, and `sameNode:true` in every post-release sample. The typed value survives initial hydration briefly, then the controlled field returns to its initial state. A later readiness control on each same field retains its entered text. Port's readiness control stays numeric `8197`; the path control's `-ready` suffix tests typing retention only, not manifest validity.

Two ordinary-load Key Value controls at 23:10:08/23:10:19 UTC already had React handlers before input and retained the exact name on desktop/mobile. These controls support the readiness distinction; they do not prove every unthrottled cold load passes.

[Durable JSON](../../evidence/m219-a797-input-startup.json) preserves the exact V2/V3 replay factories, modified executed snippets, ten complete DOM/control receipts, real script metadata and cleanup. The handler always releases and unroutes in `finally`. A new deployment may change the script hash; discover the corresponding current entry URL before replay. The broad-pattern exploratory run and a run with `delayApplied:false` are excluded from primary causal evidence.

Local supporting screenshots, inspected before filing: `.playwright-mcp/qa-a797-blueprint-mobile-typed.png` and `.playwright-mcp/qa-a797-blueprint-mobile-cleared.png` show the entered Name and its subsequent placeholder. They are ignored and are not the durable evidence dependency.

## Root cause and target fix

- `dashboard/src/routes/keyvalue.new.tsx:92` initializes Name to empty; lines 149–154 render an enabled controlled Input whose `onChange` updates that state.
- `dashboard/src/features/services/hooks/use-service-name-draft.ts:33` initializes Name to empty; `routes/services.new.tsx:180–199` binds it to the controlled input. `features/services/hooks/use-new-service-form.ts:57–83` initializes Port from the default; `routes/services.new.tsx:449` binds `form.port` through TextField. No source is selected in these probes, so source-derived name behavior is not the explanation.
- `dashboard/src/routes/blueprints.new.tsx:58–68` initializes Name empty and path `render.yaml`; lines 269–282 and 302–313 expose their controlled native inputs before handlers exist. No repo or branch was chosen in this probe.
- The server shell renders content before `<Scripts/>`. Browser-native editing can therefore happen before React's handlers attach. The unchanged node in the receipts rules out an observed form remount as the cause.
- Dependency code was opened at the lockfile's actual `react-dom@19.3.1`, from the [official npm tarball](https://registry.npmjs.org/react-dom/-/react-dom-19.3.1.tgz). In `cjs/react-dom-client.production.js`, `initInput` (1315; hydration branch 1343) preserves the current DOM value while hydrating. `updateInput` (1269–1293) later writes a differing controlled value. An event fired before handlers cannot update component state. This explains preservation during initial hydration followed by restoration on a normal controlled update. No claim pins the later update exclusively to catalog arrival or independently extracts React's deployed version from the entry script.

**Target:** reuse `dashboard/src/common/hooks/use-is-hydrated.ts:22` at the three form roots. During SSR and hydration, native editable controls must be disabled/inert; once handlers are ready they become editable with the same layout and defaults. A local disabled fieldset or explicit local props can express this without changing the global Input or introducing a new readiness framework. If using fieldset, reset its default geometry. Preserve immediate usability on client-only navigation. Do not repair after the loss, silently import arbitrary DOM text, or make these validated forms uncontrolled.

## Blast radius, aliases and neighboring states

The existing hydration helper has seven production calls in six files: `session-row.tsx:22`, `deploy-header.tsx:36`, `charges-card.tsx:282`, `use-local-date.ts:17` and `:23`, `local-time.tsx:45`, `ui/sidebar.tsx:236`. Grep excludes tests and the definition. Its implementation stays unchanged; t004 checks its existing consumers and the three new local uses.

The service form shares five types: web, private, worker, cron and static. `common/lib/render-alias.ts:35–42` maps `/web/new`, `/pserv/new`, `/worker/new`, `/cron/new`, `/static/new` onto their `services/new?type=` variants. Key Value aliases `/r/new` and `/new/redis` reach `/keyvalue/new`. Blueprint's canonical route is `/blueprints/new`; route/source search found no Blueprint creation alias. Verify redirects and query preservation; no alias was live-probed here.

The startup gate is distinct from data loading, catalog errors, validation, forbidden/unauthenticated responses and request timeouts: once hydrated, those states retain their present handling, and m218 owns catalog retry work. Keep existing submission gates and zero paid actions. At the original a797 capture, dialog-based creates and non-web service variants remained unverified. The [a800 follow-up](finding-a800.md) subsequently observes all four non-web variants and a passing Postgres dialog control at the listed widths. The [a805 control](control-a805.md) additionally verifies the webhook create **page** avoids pre-handler editing and retains drafts during catalog retry at both widths. Project/environment-group dialogs, other editable fields, untested widths/source tabs, client-side navigation and nondefault languages remain **unverified** and belong to the bounded audit rather than observed DoD claims. No API shape change is needed.

## Render and dedupe

Primary docs checked 2026-10-16: [web service creation](https://render.com/docs/deploy-node-express-app), [Key Value creation](https://render.com/docs/key-value), [Blueprint specification](https://render.com/docs/blueprint-spec). They document creation/configuration, not authenticated pre-hydration timing. No comparative Render startup probe is claimed. ADR004, ADR021, ADR049 and ADR018 govern the same bex surfaces; no anti-goal covers discarding accepted draft input.

Reviewed all 85 open/blocked milestone titles across workstreams, DO_NOT_DO, open/done searches for hydration, early input, draft resets and these form paths, the latest 40 product commits and targeted hydration/name history. `w6/done/m102`, `m107`, `w5/done/m3` and `w4/done/m152` concern timestamp/locale hydration. `w9/done/m55` concerns dead-ID redirects; `w9/done/062` CLI device auth; `w4/done/265` Search query lifetime; `w3/done/m80/t005` re-auth draft persistence. These have different triggers/contracts. `w4/331` fixes branch state on source-tab changes; `w4/329–330` plan skeletons; m218 catalog-error recovery. No matching owner or already-landed startup input fix found.

## Cleanup

Read-only pass: no resources created or create/deploy action submitted. At 23:23:36 UTC all seven inventory families exactly matched baseline; scoped cleanup reports no console errors or HTTP failures. At 23:23:42 the pass's own session was revoked (whoami 401), cookies cleared to zero, sole tab `about:blank`, and exact session handles removed. This does not claim an exhaustive historical browser-request census.
