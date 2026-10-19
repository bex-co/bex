# Live QA findings — loop run 1, 2026-10-19 (`/qa-find-bugs w4`, `muse.env`, workspace `bex-canary`)

Production `https://dashboard.bex.co` + `https://api.bex.co`. Owned fixtures (all created and deleted in-run): web service `qa-20261019-loop1-web` (`srv-dbaupvqot0kc73chuspg`), Key Value `qa-20261019-loop1-kv` (`red-dbavm2sso40s73cfs9ig`), broken-build service `qa-20261019-loop1-broken` (`srv-dbavpllk8aos739580bg`), env group `qa-20261019-loop1-group`, project `qa-20261019-loop1-proj`. Read-only control: pre-existing `hello-go` (`srv-daif6dsmg29s73d1umvg`).

Journeys exercised and passing: create web service from public Git (build → Live → `.onbex.co` serves the `MESSAGE` env value), env-group link → redeploy, env edit → redeploy → new value served, manual deploy → cancel (prior deploy stays live), rollback (restores retained config, `autoDeploy` flips to `no`), logs search narrowing, free-tier sleep → wake serves in < ~35 s, scaling/shell paid-gate honesty, custom-domain add (TXT + CNAME instructions) → remove same visit, Postgres create refused at the 1-per-Free-workspace limit with the reason in the dialog, Key Value create → `available`, project + environment create and resource assignment, broken build (`rootDir` that does not exist) → `build_failed` with the precise reason on the deploy page.

### 1. The Metrics page renders two Application Metrics cards; the first loads forever

- Severity: major
- Repro: 1. Sign in. 2. Open any non-static service → Metrics (fresh document load). 3. Wait ≥ 9 s. 4. Run the DOM probe below. (On `https://dashboard.bex.co/services/srv-daif6dsmg29s73d1umvg/metrics` and on the owned fixture alike.)
- Expected / Actual: one Application Metrics card with data. Actual: two. Card 0 never resolves — header tabs render, Memory/CPU/Total Instances show `Limit` + `…`, no Instances selector ever appears. Card 1 is fully working (Instances selector populated, charts + latest values).
- Durable probe (run in the page console / `browser_evaluate`):

  ```js
  [...document.querySelectorAll("main *")].filter(
    (e) => e.children.length === 0 && e.textContent.trim() === "Application Metrics",
  ).length; // → 2 today, must be 1
  document.querySelectorAll("main [data-slot=card]").length; // → 3 today, must be 2
  [...document.querySelectorAll("main [data-slot=card]")].map((c) => c.innerText.slice(0, 120));
  // → card 0: "Application Metrics\nRaw\nMin\nMax\nAvg\nPercentage\nTotal\nMemory\nLimit\n…\nCPU\nLimit\n…\nTotal Instances\nManage scaling\n…"
  // → card 1: "Application Metrics\nInstances\nsrv-…-9ddf2\n…" (working)
  ```

- Evidence: `.playwright-mcp/qa-metrics-1.png` (session-local; the probe above is the durable artifact). No `data-skeleton-frame` nodes remain in the settled DOM, and neither heading sits under an `aria-hidden` ancestor — both are live, visible cards in the same `max-w-4xl` content column, with a single MetricsFilters above them.
- Root cause (partially pinned): `dashboard/src/routes/services.$serviceId.metrics.tsx:128-136` renders `ApplicationMetricsCard` exactly once, and `metrics.applicationTitle` has one render site (`dashboard/src/features/metrics/components/application-metrics-card.tsx:169`) — so the component is mounted twice at runtime, not written twice. The dead twin behaves like a mount whose queries never settle (its instance-discovery read yields nothing, so no Instances selector, and every `LatestValue` stays `…`). Prime suspects, in order: (a) the parent route `dashboard/src/routes/services.$serviceId.tsx:23-24` sets `pendingComponent: RouteComponent` with `pendingMs: 0`, so during the title-loader wait the router mounts the layout — including its `<Outlet/>` (`dashboard/src/features/services/components/service-detail-layout.tsx:156-160`) — and the settled tree mounts the page a second time without the pending copy's DOM being retired (SSR streaming / hydration boundary); (b) the layout's `service ? <Outlet/> : <PendingTabContent/>` swap racing the route's own `pendingComponent: ServiceMetricsSkeleton`. Which of these is load-bearing is **unverified** — t001 settles it before fixing. What is verified: at HEAD there is one textual render site, and production mounts two.
- Fix (target behavior): exactly one `ApplicationMetricsCard` mount per Metrics page, and it is the data-carrying one. The pending/skeleton path must never leave a second live copy of the page behind — if the pending mount is the cause, the parent pending frame must not render the child `<Outlet/>` (render the tab skeleton instead, as `PendingTabContent` already does once `service` is null), or the pending copy must unmount cleanly on settle. Do not paper over it by hiding the first card with CSS.
- Blast radius: the pending mechanism is shared by every service tab (`ServiceRouteContentSkeleton`, `route-skeletons.tsx:2592`) and by `/static/$serviceId` (`pendingComponent` there too). Any fix in the parent route or layout must be checked against Deploys, Logs, Environment, Settings and the static tree for the same double-mount (a cheap DOM probe per tab: no duplicated primary heading). Callers that behave correctly today may be correct because their queries dedupe in Apollo — Metrics' fan-out is what made the twin visible.
- Adjacent classes: the static-site Metrics page (`static.$serviceId.metrics.tsx`) renders its own skeleton + the shared `ServiceMetricsPage`; verify it shows one Network card and no Application card, before and after.
- Unverified: the exact remount mechanism (above); whether other tabs double-mount invisibly; whether this also fires on client-side tab navigation (observed on fresh document loads only). Separately observed, not filed as its own claim: on the owned fixture the working card's Total Instances header read `2` while the Scaling page's Total Instances chart and the service header both said `1` — plausibly a latest-point vs window reading during the rolling deploy; re-check while fixing, file separately only with its own probe.
- Render: render.com's Metrics page has one Application Metrics card (the card is modeled on it, `application-metrics-card.tsx:46`).
- Estimate: 45m including mechanism confirmation.

### 2. Tenant deploy/event surfaces print the cluster-internal build registry host

- Severity: minor
- Repro: 1. Create any git-built service. 2. Open its Events tab, or `GET /v1/services/<id>/events` / `GET /v1/services/<id>/deploys/<deployId>` from the signed-in page. 3. Deploy rows carry `zot.bex-registry.svc:5000/<workspace>/<workspace>-<service>:gen-2@sha256:…`.
- Expected / Actual: tenant-facing deploy data names the commit (and, for image-backed services, the user's own image). Actual: git builds expose the cluster-internal registry DNS name and the tenant-prefixed internal repository path on the Events tab and in the REST `image.ref` / event `details.image`.
- Root cause: deploy rows store the built image ref verbatim and every read passes it through — `lego/backend/internal/deploys/rest.go:134-135` (`renderDeployImage{Ref: d.Image}`), `lego/backend/internal/events/service.go:901,1003` (`ev.Details.Image = r.Image`, the w1/m47 dashboard enrichment), surfaced by `events/rest.go:95,191` and `events/graphql.go:81`. The internal host is a known constant (`lego/backend/internal/store/image.go:93` trusts `zot.bex-registry.svc` / `*.cluster.local`).
- Fix (target behavior): no tenant-facing surface (REST deploy `image.ref`, REST/GraphQL/MCP event `details.image`, dashboard Events/Deploys) contains a `*.svc` / `*.cluster.local` registry host. For git builds show the digest-bearing short form without the internal host (or omit `image` and keep commit identity, which is what the row already carries); user-supplied images pass through byte-identical. One shared projection helper at the read boundary, not per-surface string surgery.
- Blast radius: the deploy `image` is also consumed by rollback ("run this deploy's image") and by operator/status paths — the stored value must not change; redact at projection only. Grep every reader of `Deploy.Image` / `Details.Image` and count them in the task before choosing the seam.
- Adjacent classes: forbidden/foreign reads unchanged (projection happens after authorization); image-backed services are the control case and must not be redacted.
- Unverified: what render.com puts in `deploy.image` for git builds (no account); the fix does not depend on it — the claim is only that cluster-internal DNS is not tenant data.
- Render: n/a (see Unverified).
- Estimate: 45m.

## Dropped after triage (not bugs)

- `GET /v1/services` without `ownerId` returns only the default workspace's services (5 of 17 across the QA user's 3 workspaces) — documented deterministic-default contract (`lego/backend/internal/apps/service.go:1353-1365`, ADR018 create/list rows; multi-value `ownerId` works per w1/m168).
- First deploy's REST `trigger: "other"` — deliberate mapping onto Render's closed enum (`lego/backend/internal/deploys/rest.go:302-320`, w8/058).
- Custom-domain REST `id` equals the hostname, not `cdm-…` — deliberate per `docs/ADR020-identifiers.md:87`.
- Two isolated `Failed to load resource: net::ERR_FAILED` + CORS console errors on `api.bex.co/graphql` during ~4 h of polling — `curl` probes with an `Origin` header show correct `Access-Control-Allow-Origin` on preflight and 401s; consistent with a transient edge response, not reproduced. Watch-list only.
- Deploy-failure summary joins build-log lines with spaces (`…FETCH_HEAD the root directory…`) — cosmetic; the reason itself is precise. Not filed.
