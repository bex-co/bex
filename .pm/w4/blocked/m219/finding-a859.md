# a859 — Populated static-rule editors accept startup edits that the next render discards

**Severity:** major (silent draft loss). **Observed:** 2026-10-17 06:10–06:14 UTC. **Research:** `457b04078836b1767259d4cbd1ef709c855af089`, reviewed unchanged through `b3dbca6013a318be3e050ba080ec3fcaf1648aaf`. This adds two roots to open m219; it does not claim a regression in its ten already implemented roots.

## Reproduce and observe

Create the owned Free no-build static site in the [complete safe evidence](../../evidence/m219-a859-static-editors-startup.json). Save the four recorded routes and two headers through the dashboard. Only these setup saves write configuration; none of the startup probes writes.

1. Fresh-load `/static/<own-id>/headers` while holding the real fetched entry script, then release it unchanged using the recorded V3 script. Discover the current entry URL rather than assuming the old hash. At both 1440 and 390px, the first **Value** field is enabled before handlers, with saved value `routing-a859`. Native trusted typing changes it to `routing-a859-typed` while `reactProps:false`.
2. Hydration initially preserves that DOM text on the same node. Once handlers are attached, click **Add header**, an ordinary local draft update. The same Value input restores `routing-a859`, silently discarding the edit. Type `routing-a859-typed-ready` after readiness and click Add header again: that text stays.
3. Repeat `/static/<own-id>/redirects` at both widths using the first **Source** input. Its saved `/qa-jump/*` accepts `/qa-a859-typed` before handlers. The first ready **Add rule** restores `/qa-jump/*` on the same node. Ready typing `/qa-a859-typed-ready` survives another Add rule.

The four primary receipts contain the unchanged HTTP200 entry response metadata (`index-ByZLA702.js`, 1,957,482 bytes), trusted input events, 20 same-node samples, the controlled-render reset, the passing ready control and **zero mutations**. These are controlled startup-latency reproductions, not a claim that every ordinary load loses input. An earlier natural Add-header click produced no row, but its timing was uninstrumented and it is not a separately filed defect.

Screenshots were viewed: `.playwright-mcp/qa-a859-header-startup-before.png` shows the typed Value with Save disabled; `qa-a859-header-startup-after.png` shows its old value restored after Add. They are local aids; the tracked JSON carries the scripts and complete receipts needed for handoff.

## Root cause and target

- `dashboard/src/features/services/components/static-site-section.tsx:197–230` and `:344–376`: RoutesEditor and HeadersEditor seed controlled drafts from saved rows, but render editable native controls without a readiness gate. Their update/add handlers are `:219–226` and `:365–372`.
- The first Source flows through RuleInput at `:283–293`, whose Input at `:515–522` reads the controlled value and only records edits through onChange. The Header Value has its own controlled Input at `:426–433`. Before startup, native typing cannot invoke those handlers. An Add update therefore renders the original draft value.
- Opened actual cached official **react-dom 19.3.1**, matching `dashboard/yarn.lock:10564–10566`: `initInput` (`react-dom-client.production.js:1315`, hydration branch `:1343`) preserves existing DOM text during hydration; `updateInput:1269–1293` writes a differing controlled value during later updates. This explains the initially retained DOM and subsequent reset. This is an application readiness gap, not a React defect. The deployed entry's React version was not independently extracted; installed 19.2.3 is not evidence for the pinned mechanism.
- Reuse the existing `HydrationFieldset` at `dashboard/src/common/components/hydration-fieldset.tsx:13`, based on unchanged `useIsHydrated`. Gate each of the **two editor roots**, including its native draft actions, until handlers are ready. Preserve layout and immediate readiness on client navigation. Do not globally change Input/RuleInput, disable ordinary post-hydration in-flight editing, read arbitrary DOM values into state, or change routing/API semantics.

**Target behavior:** pre-handler controls visibly decline edits/actions they cannot retain. After readiness, exact native typing survives Add and normal controlled updates, with existing validation, Save/Cancel and server-accepted-row adoption intact. The API's saved rules remain authoritative; unavailable startup is not an empty saved list. Existing loading/error/capability and busy/save predicates retain their meaning after readiness; no new auth/error taxonomy is introduced.

## Shared consumers, aliases and preserved controls

Exhaustive production grep finds two editor definitions and exactly one mount of each: `routes/services.$serviceId.redirects.tsx:23` and `routes/services.$serviceId.headers.tsx:23`. The `/static/$serviceId/redirects` and `/static/$serviceId/headers` route modules reuse those page exports. These are **four route entries for two roots**; only the static-base entries were live-probed. StaticEdgeRulePage has the two page mounts and keeps the non-static redirect-to-Settings guard. There are four RuleInput sites (two per editor), one direct Header Value Input and one route Type Select site. The existing hydration fieldset has three production mounts in creation forms. Keep its implementation and those callers unchanged.

The future shared-consumer task must verify Header Path/Name, route Destination/Type and native add/remove/cancel controls, empty/populated lists, services-base entries and client navigation. Those are source-enumerated audit work, not additional observed failures.

The owned ready-state journey passed:

- Native saves persisted four routes and two headers; complete independent GraphQL and REST reads retained them after all startup probes. There was still one Live create deploy, `dep-db9h2fg7bp4s73el3m1g`, at `rev-1`; edge-rule saves did not rebuild.
- Sixteen final GET/HEAD cases served the 1548-byte root and 795-byte YAML, made `/friendly.yaml` return the same YAML hash, and retained captured `%23`, `%3F`, `%20` and `%25` path data in 301 Locations with the configured `?from=rule#section`. `/render.yaml` remained 200 despite its redirect rule because the existing asset wins.
- `X-QA-A859: routing-a859` applied to successful and redirect responses. `/friendly.yaml` alone used `Cache-Control: no-store`; direct YAML retained its default policy. The plain redirect's configured `/index.html` was observed; no new guarantee about forwarding an incoming query string is inferred.
- Preserve w4/136's accepted-row normalization and later-draft guard, and w4/268's newer-saved-row adoption. This pass checks native saves/fresh reads and ready typing, not a complete padded-save, mid-save or two-tab replay of those earlier DoDs; t018 and the standing tests own those regression controls.

## Dedupe, parity and limits

Reviewed **89 canonical open/blocked milestone READMEs**, whole-board terms including done items, DO_NOT_DO, recent 40 product history and targeted editor history. Two nested evidence READMEs (`w8/evidence/m61`, `m62`) were excluded from the milestone count. The editor's latest source commit remains `3ec9f8d15` (w4/268); no readiness fix is pending on fetched main. m219 already owns the mechanism: add implementation/audit tasks and reopen its standing checks, preserving all prior work. m220 concerns loading geometry, m221 placement-read errors, w4/145 row validation, w4/136 canonical saves and w4/268 newer-data refresh; none owns this pre-handler loss.

[ADR008](../../../../docs/ADR008-vision.md), [ADR029](../../../../docs/ADR029-static-sites.md) and [ADR018](../../../../docs/ADR018-render-parity.md) establish the existing hosting/rule surface. [Render's primary routing docs](https://render.com/docs/redirects-rewrites), read this sweep, describe dashboard rules, first-match behavior and existing-file precedence; they do not establish authenticated startup handling. That comparison remains unmeasured. No feature expansion, paid action, existing-resource mutation or cybersecurity work is part of the finding.

Unverified: other field/action sites listed above, empty-editor startup, CSR and services aliases, nondefault locales, real network outages and full old-m219 live acceptance. The first Header V1 retained text through the sampling window; only the ordinary Add render exposes the loss. An initial `#route-0-source` probe matched nothing because RuleInput uses that identifier for its error element, not the input: it is excluded, not an absence control. Corrected aria-label selectors produced both route failures.

## Cleanup and closeout condition

The owned site `srv-db9h2fg7bp4s73el3m10` was natively deleted at 06:15:49 UTC. Its service/deploy reads return 404 and all seven inventory families equal their starting sets (06:16:03). Logout is confirmed: whoami 401, zero cookies, one blank tab and own handles removed (06:16:09). Exact App, serving/build/publish/purge/predeploy children are all absent across namespaces (06:16:41); the public URL returns404 with a complete final receipt. No fixture or session remains. No remote object-store erasure claim is made.

Close m219 only after its original ten-root matrix **and** these four populated static-editor cases pass on the deployed fix, with preserved ready controls, API/routing behavior and cleanup. This hunt ships the filing only.
