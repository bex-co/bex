# w6 · m46 — Private services publicly exposed via platform subdomain; first-time deploys wrongly marked Canceled with a stuck status pill

**Worker:** worker6 **Goal:** a Private Service is actually unreachable from the public internet, matching its own advertised guarantee; and a brand-new service's first deploy reports a status the dashboard, the deploy list, and the backend all agree on — never a false `Canceled` with a header pill stuck on a phase the deploy already left. **Status:** done (closed unshipped — the live `curl` / first-deploy probes are owed by whoever runs `/ship`; see Outcome)

## Tasks (in order)

| id   | title                                                                                                | est | depends_on          |
| ---- | ----------------------------------------------------------------------------------------------------- | --- | -------------------- |
| t001 | Root-cause + fix the write path that leaves a private_service's `Expose`/`Host` publicly routable — **DONE**      | 90m | —                     |
| t002 | Operator defense-in-depth: hard-gate Ingress/host creation for `private_service` regardless of `Expose` — **DONE**| 45m | —                     |
| t003 | Blast-radius audit: every create/update entrypoint (REST/GraphQL/MCP/blueprint sync) for the same gap — **DONE**  | 90m | [t001, t002]          |
| t004 | Root-cause + fix a first-time deploy wrongly marked `Canceled` (release-generation init race) — **DONE**          | 60m | —                     |
| t005 | Fix the Service header status pill staying stale after a server-driven (non-button) deploy closure — **DONE**     | 45m | [t004]                |
| t006 | Render parity across REST/GraphQL/MCP/UI — **DONE**                                                               | 30m | [t001, t002, t003, t004, t005] |
| t007 | Simplify the touched code — **DONE**                                                                              | 30m | t006                  |
| t008 | Test coverage for the fixed behaviors — **DONE**                                                                  | 45m | t006                  |
| t009 | Closeout — **DONE**                                                                                                | 10m | t008                  |

## Definition of done

- Creating a `private_service` through the dashboard's "New Service" form results in no live-serving `.onbex.co` URL: `curl -sSI https://<name>.onbex.co` returns a routing/TLS failure (no valid Ingress/cert), never the app's own 200 response — verified immediately after the service reaches `Live`.
- A controller-envtest that force-sets `spec.Expose=true` (and separately, a custom `spec.Host`) directly on a `private_service` App proves the operator never creates an Ingress object for it — proven red against the pre-fix operator, green after.
- A private_service created through each of REST, GraphQL, MCP, and a `render.yaml` blueprint sync never carries a public URL in its create/read response — one regression test per surface.
- A fresh service's first-ever deploy (nothing else triggered against it) reaches a terminal status that matches reality — `Live` or a genuine build/deploy failure — never `Canceled`; proven via an integration test that is red pre-fix.
- The dashboard's Service header status pill matches the deploy's actual terminal state within one poll interval after ANY deploy-closing event — a button click (already fixed by `w6/m45` t003) or a server-driven transition (this milestone) — without requiring a manual page reload.

## Outcome

**One write caused both bugs.** `lego/backend/internal/store/reconciler.go`'s `projectSpec` hardcoded `Expose: true` for every App it projected, because the `apps` table carried no service `type` — the projector had no way to tell a `private_service` from a `web_service`. `applyOwnedSpec` then re-stamped that onto the CR on every resync, overwriting the type-aware `Expose=false` the create path had computed (`apps/specFromCreate`, which was correct all along — which is why t001's own trace could not find the divergent write on the create path: it was not there).

- **Exposure (t001):** `spec.Expose=true` → `EffectiveHosts` yields `<slug>.<baseDomain>` → the operator builds a public Ingress. The hunt's `curl` was reading a route the control plane had itself created.
- **False `Canceled` (t004):** the same overwrite is a spec-mutating `Update`, bumping `metadata.generation` 1→2. `store.CreateApp` opens the first deploy row at generation 1; when the projector's write wins the race against the operator's first reconcile, `release_identity.go` initializes `status.releaseGeneration` from the already-bumped generation, and `supersededDeployStatus` closes the app's first-ever deploy `Canceled`. `Expose` is classified `identityOperational`, so only this *first-read initialization* is affected — which is why the bug was intermittent.

This also explains the affected set exactly: `web_service`/`static_site` agree with the projector on `Expose`, so they never diverge. `private_service`/`background_worker`/`cron_job` diverge on the projector's very first pass — matching the hunt's exposed `private_service` and falsely-canceled `background_worker`.

**What shipped**

- Migration `0095` adds `apps.type`; the create path records it, and `projectSpec` derives exposure from it. `Expose` and `Type` left `applyOwnedSpec`'s owned set — nothing in the row or any lifecycle verb ever changes them, so a converged resync now writes nothing at all (which is what removes the generation bump). Pre-0095 rows self-heal from their own live CR, once.
- The exposure rule now lives in exactly one place: `AppSpec.PubliclyRoutable` / `TypePubliclyRoutable` on the CRD contract, an **allowlist** so an unrecognized type fails closed. `EffectiveHosts` gates on it (covering the operator's Ingress/status derivation, the static-server resolver, and bex-api's pending-URL projection at once), and `reconcileKubernetes` repeats the check where the Ingress is actually created. `specFromCreate`, `pendingPublicURL`, `setPublicRoutingCondition`, and the Blueprint validator all now call it instead of keeping their own copies.
- Custom domains follow the same rule everywhere. The Blueprint path already refused them for a non-routable type; create named only `background_worker`/`cron_job` (letting `private_service` through) and the `AddDomain` verb checked nothing at all. Both were aligned onto the shared predicate, and the dashboard stops offering the Custom Domains card and the platform-subdomain toggle to a type that can never have either.
- `Service.Create` now stamps `app.bex.co/release-generation: 1` — every other deploy-opening path already did; create was the one leaving the operator to infer a release from whatever generation it read first. `store.FirstDeployGeneration` makes the row/annotation agreement a compiler-checked fact instead of two literals and a comment.
- The header polls honestly (t005). Both header badges were on the flat 30s baseline with nothing to refetch them: a server-driven deploy closure fires no local mutation, so `w6/m45` t003's `refetchQueries` fix (Cancel/Rollback button only) could not reach it. `useServer` and `useLatestDeploy` now use a shared `useConvergingPoll` — 3s while the phase/deploy is still moving, baseline once terminal.

**Verified**

Every regression test was proven red against pre-fix code and green after (ledger in the commit): 3 projector-exposure subtests, the resync no-write assertion, the store-row type subtests, the real-Postgres migration round-trip, 5 operator envtest specs (which write bad `Expose`/`Host`/`Hosts` values directly onto the CR, bypassing bex-api entirely), the 4 domain-gate subtests that were genuinely wrong, the two first-deploy pins, and 7 dashboard poll/gating specs. All four Go suites, the operator envtest suite (108 specs), `make lint` (4 modules), and `dashboard/yarn test` (2542 tests) pass. The migration was applied, rolled back, and re-applied against a real Postgres 17.

**Owed at `/ship`** — these DoD bullets are inherently production checks and cannot be run from a working tree:

- `curl -sSI https://<name>.onbex.co` against a freshly created `private_service`, immediately after it reaches `Live` — must be a routing/TLS failure, never the app's 200.
- A fresh service's first-ever deploy reaching `Live` (or a genuine build failure), never `Canceled`.
- The header pill tracking a server-driven deploy closure without a reload.

Also worth doing once deployed: any `private_service` created before this fix keeps its Ingress until the operator reconciles it; the fix removes the route rather than merely stopping new ones (covered by the "removes an Ingress a previously-exposed private service already had" envtest), so confirm the hunt's finding cannot be reproduced against an existing service. A previously-exposed private service's orphaned TLS Secret is left behind inert — no Ingress references it, and the App-deletion prefix sweep collects it — deliberately out of scope here.

**Deliberately not changed:** the recorded `w9/m58` split where REST/MCP omit `serviceDetails.url` for a private service while GraphQL retains `url` as a bex-shaped surface (ADR018 § Internal address). It was re-verified rather than reversed; what all four surfaces now agree on is that none reports a *public* host for a private service. The deploy-status vocabulary is unchanged.

## Source + Goal linkage

- **Source:** live QA hunt of `dashboard.bex.co` hosting features, 2026-08-22 (via `/qa-find-bugs`, run from a `/loop 30m` session, iteration 4). Workspace "bex", project `qa-20260822-hunt4` (deleted, cleanup verified against the pre-hunt Overview baseline).

  **Private-service exposure — durable evidence (the probe itself, per this hunt's own evidence rule):**

  ```
  $ curl -sSI --max-time 10 https://qa-20260822-private.onbex.co
  HTTP/2 200
  content-type: text/plain
  date: Sun, 23 Aug 2026 03:52:33 GMT

  $ curl -sS --max-time 10 https://qa-20260822-private.onbex.co
  hello from bex
  ```

  Raw GraphQL capture (`fetch` from inside the authenticated dashboard page, `credentials: 'include'`, against `https://api.bex.co/graphql`):

  ```
  query { server(id: "srv-da56phoaj4rs73a8bnrg") { id name type url renderSubdomainPolicy phase } }

  {"data":{"server":{"id":"srv-da56phoaj4rs73a8bnrg","name":"qa-20260822-private","phase":"Running","renderSubdomainPolicy":"enabled","type":"private_service","url":"https://qa-20260822-private.onbex.co"}}}
  ```

  `url` here is `a.Status.URL` (`lego/backend/internal/apps/service.go:796`) — the **operator's own** status field, not a client-computed convenience string, so the operator itself independently confirms this App is live-routed publicly. The Settings page's "Platform Subdomain: Enabled" toggle and live URL link matched. After deletion, the same host's TLS handshake fails (no cert), confirming the route existed only while the service did and that cleanup fully removed it. No other `private_service` exists in this workspace today (`services { type }` swept clean), so no other live customer resource in this workspace was found similarly exposed by this hunt — the underlying code path is not workspace-scoped, though, so this does not bound the defect to this workspace.

  **Code trace (why this needs research, not a one-line patch):** `lego/types/v1alpha1/app_types.go:700-717` (`EffectiveHosts`) only adds the platform subdomain when `spec.Expose && baseDomain != "" && spec.SubdomainPolicy != Disabled` — it does not gate on `spec.Type` at all, and its `add(spec.Host)` line fires unconditionally regardless of `Expose`. The operator's `reconcileKubernetes` (`lego/operator/internal/controller/app_controller.go:1606-1608`) only special-cases `background_worker` to skip Service/Ingress/URL entirely (`worker := app.Spec.Type == appv1alpha1.TypeBackgroundWorker`); `private_service` falls through the exact same path as `web_service`, relying solely on `Expose` staying `false`. Yet the ONE place in the whole backend that decides `Expose` (`lego/backend/internal/apps/service.go:2098`, `specFromCreate`) is explicitly type-aware and already correct: `Expose: svcType == appv1alpha1.TypeWebService || svcType == appv1alpha1.TypeStaticSite` with the comment "private has no platform host." An exhaustive `grep -rn "\.Expose\s*=\|Expose:"` across `lego/backend/internal/apps/*.go` and `lego/operator/internal/controller/*.go` found exactly **two** assignment sites — this one, and `service.go:2458`'s straight pass-through (`dst.Expose = want.Expose`) inside `applyCreateToSpec`, which is fed by the same correctly-computed `want`. No legacy `req.Private`-style field remains in `rest.go`/`mcp.go`. The `Expose` line itself is over a month old (`84e2216`, 2026-07-12) and long merged to `main` — not an undeployed fix. **This is the "probe contradicts the code" case the hunt's own rigor checklist calls out**: the observed live behavior (Expose effectively true) contradicts every create-path read this session traced. t001 is scoped to find the actual divergent write (a second post-create PATCH? the blueprint/stack `applyCreate` path in `deploy.go`, untraced this run? something inside `materializeNewApp`/`provisionAppIdentity` not yet read?) — not to re-guess it. t002 adds the type-level backstop `EffectiveHosts`/`reconcileKubernetes` should have had regardless: relying on a single boolean staying correct forever, with no second gate, is exactly the shape of bug that produced this.

  **First-time-Canceled deploy — evidence:** created `qa-20260822-worker` (background_worker), whose only deploy `dep-da56pr0aj4rs73a8bnu0` went from `Build queued` (08:45:48 PM) straight to `Deploy canceled` (08:46:31 PM) with no build ever starting — no compile output in the deploy log, just those two lines. The Events tab shows `Build started` / `Build ended: Canceled` / `Deploy ended: Canceled`, but nothing in the app names a cause; I never clicked Cancel. A GraphQL `server(id).phase` probe, repeated after a genuine full-page reload (fresh Apollo cache), kept returning `"phase":"Building"` — persisted well past the deploy's own `finishedAt`, confirming this is a backend-persisted inconsistency, not client staleness. Code: `lego/backend/internal/store/reconciler.go:917-929` (`supersededDeployStatus`) closes an open deploy row `DeployCanceled` whenever `app.Status.ReleaseGeneration > open.Generation`; `lego/operator/internal/controller/release_identity.go:301-302` initializes `ReleaseGeneration = app.Generation` (the CURRENT, possibly already-bumped k8s object generation) the first time it is read as zero. Any spec-mutating write to the App between the deploy row's creation and this initialization would manufacture a "supersede" for a release nothing ever actually replaced. **Unverified this run:** which second spec write actually bumps `app.Generation` for a bare first-time create with no env vars/secrets set — t004 is scoped to reproduce this in an integration test with generation assertions at each step, not to re-guess the trigger.

- **Goal linkage:** tenant network-isolation guarantee (`docs/ADR043-tenant-namespace-isolation.md`) for the exposure bug; deploy-status trustworthiness (`docs/ADR004-app-deployment.md`, `docs/ADR060-build-worker-reliability-and-performance.md`) for the false-cancel/stale-pill bugs.
- **Expected outcome:** a customer who deploys a Private Service can trust the product's own explicit promise ("Accessible only within the platform network") instead of unknowingly exposing internal-only infrastructure to the public internet with no authentication; a customer's first deploy is never falsely reported as canceled, and the dashboard's status indicators stay truthful without a manual reload.
- **Why now:** the exposure bug is a live tenant-isolation violation on production right now, reproduced by curling a real public URL that served a real app response with zero authentication — security-relevant, not cosmetic, and root-caused down to a two-candidate-site search with a clear defense-in-depth backstop already identified. The false-cancel/stale-pill bugs were caught in the same repro session, share the same "first deploy of a brand-new App CR" lifecycle this hunt was already exercising, and have clean file:line citations. Render parity closing task **included** — `Expose`/host semantics are read identically across REST/GraphQL/MCP/UI, and the deploy-status vocabulary + Service header are UI+API surfaces shared with `w6/m45`'s already-shipped t003.

## Investigated and rejected (not filed)

- **Shell access unavailable for a Free-tier Private Service.** `/services/<id>/shell` correctly reports "Shell access requires a running paid web, private, or background service and an active SSH gateway" — a documented plan gate, not a bug. No task filed.
- **New Web Service form's Runtime not re-detecting on Root Directory change.** Reproduced again this run (root dir set to `examples/hello-node`, Runtime combobox stayed on the previously-selected `Go`/kept whatever was last chosen rather than re-inferring from the new path) — already tracked as the needs-more-research bundle in `w6/m45` t004. Not re-filed.
- **Danger Zone delete copy claims "its URL" for a service type that never had one.** Reproduced again live on both the Private Service and a Cron Job created this run. Already filed and shipped as `w6/029.md` (`77b803c6`). Not re-filed.
