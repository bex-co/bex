# A moved service denies its own environment for ~12s, and REST omits the field rather than returning null

Why: `setEnvironmentServices` returns success and the environment lists the service immediately, but the service's own record — GraphQL `server.environmentId` and REST `GET /v1/services/{id}` — reports no environment for about twelve seconds afterwards, so anything that moves a service and then reads it back (a script, the dashboard's own service page, `w4/m111`'s env-group scope filter) sees "no environment" and acts on it; and because REST omits the key entirely rather than returning `null`, a client cannot tell "not placed" from "not settled yet".

Found by live `/qa-find-bugs` 2026-09-21 pass 111 (w4-targeted, `muse.env` credentials), journey 1, on throwaway project `qa-20260921-proj` / environment `qa-prod`, since deleted.

## Measurement

Moving a web service out of the environment and back in, polling both surfaces from the moment the mutation returns:

```
t=0        setEnvironmentServices(id: env-…, serviceIds: [srv-…])  →  success
  1675ms   server.environmentId = null     REST environmentId = ABSENT
  2816ms   null   ABSENT
  3953ms   null   ABSENT
  5085ms   null   ABSENT
  6307ms   null   ABSENT
  7428ms   null   ABSENT
  8562ms   null   ABSENT
  9684ms   null   ABSENT
 10804ms   null   ABSENT
 11938ms   env-daoeumrs0ils73bgpdig        env-daoeumrs0ils73bgpdig     ← both flip together
```

Meanwhile `environment(id).serviceIds` lists the service **immediately**, and the dashboard's project page shows it under `qa-prod` straight away — so the two sides of the same fact disagree for the whole window.

## The control case rules out the obvious alternatives

- **Not a broken write.** After ~12s both surfaces are correct and stay correct; re-running the mutation changes nothing.
- **Not resource-type-specific.** First observed on a `static_site`, then reproduced on a `web_service`.
- **Not "the field never works".** A service created _directly into_ the environment (`createService(environmentId:)`) reads back correctly on both surfaces within 2s — `environmentId` and `projectId` are both present in its REST payload immediately.

The first reading in this pass looked like a permanent disagreement; a later read had converged, which is what prompted the timed re-measurement above. The finding is the window, not a lost write.

## Two separable defects

1. **The window itself.** `store.SetEnvironmentServices` (`lego/backend/internal/store/environments.go:183-209`) updates `apps.environment_id` / `apps.project_id` in one transaction, and `ListEnvironmentServices` reads that table directly — which is why the environment side is instant. The service's own record evidently resolves placement through a slower path (the App CR projection / reconcile). Decide the contract: either the service read is served from the same store row the mutation just wrote (read-your-writes), or the lag is documented and the mutation stops implying immediacy.
2. **REST omits the keys.** Throughout the window `GET /v1/services/{id}` has **no** `environmentId` and **no** `projectId` key at all — not `null`, absent. A service genuinely outside any environment is indistinguishable from one mid-move. `w4/m116` already fixed `null`-where-an-array-is-declared once at `core.WriteJSON`; this is the same family — a declared scalar that vanishes instead of being null — and the fix should say which of absent/null means what, for both fields, on every service type.

## Estimate

~45m for (2) plus the decision and a regression test; (1) is a bigger call — if it needs the read path to change, promote it to a milestone rather than absorbing it here.

## Also checked this pass, and found correct

Recorded so the next pass does not re-walk them — the static-site journey (journey 9) came back clean, and it closes several live re-probes that earlier milestones deferred:

- **`w4/m94` — an existing object wins over a redirect rule.** With a redirect `/index.html → /should-not-win` configured, `GET /index.html` returned **200** with the file's own bytes, not a 301.
- **`w4/m94` — headers match the visitor's request path.** An `/index.html`-scoped header appeared on `/index.html` and **not** on a wildcard-rewritten request served from the same object.
- **`w4/m101` — every response class carries custom headers.** `x-qa-marker` was present on 200, on a 301 (`/old-page → /`), and on real 404s (`/missing-qa.js`, `/nope/deep.css`), with `content-type` left honest on the error bodies.
- **Rewrites work.** `/app/* → /index.html` served the index at 200.
- **The SPA fallback is documented, not a bug.** An extension-less miss returns 200 + root `index.html`; that is the accepted Render divergence recorded in [ADR029 § Default SPA fallback](../../docs/ADR029-static-sites.md) and ADR018 row 84, and asset-like misses still 404 as it says.
- **`runtime: static` is refused by `createService` but accepted by Blueprint** — also correct, not a gap: `effectiveRuntime` (`apps/service.go:873-875`) records that bex's runtime enum deliberately has no `static`, and Render behaves the same way on both of its surfaces.
- **The webhook event catalog is complete.** The create form offers exactly the 53 types `webhookEventTypes` advertises (63 lines minus 10 group headings); five that a naive slug-vs-label comparison flagged as missing are all present under friendlier labels ("Service Went To Sleep" for `service_hibernated`, and so on).
