# w6 · m110 — App compute is never metered, and a service named ≥22 chars loses CPU/memory/instance metrics

**Worker:** worker6 **Goal:** every App's pods are selected by an identity that actually matches them — so App compute is metered and billed like Postgres/Key Value already is, and the Metrics page's Memory/CPU/Total Instances cards stop going blank for services with ordinary-length names. **Status:** code complete and deployed — t001–t007 done (t005's parity pass found and fixed a second, MCP-only empty-case divergence); `23c323f9` is an ancestor of the production image pinned by `71fe9660`; t008 closeout is blocked only on production QA credentials and cross-milestone fixture sequencing

## Background (found live, 2026-08-27, 21st `/qa-find-bugs` run)

Signed in as the QA user (workspace `bex`, `tea-d98210cbbpdc73dcrkvg`, Pro plan) and opened the Metrics tab of the one service in the workspace whose name is longer than 21 characters. Its **Application Metrics** cards (Memory, CPU, Total Instances) all read "No data in range" while the **Network Metrics** cards on the same page, for the same service, over the same window, had real data. Every other service in the workspace renders all six cards.

Chasing that from the API rather than the UI turned up **two independent defects in the same family** — the PromQL selectors that identify an App's pods in Prometheus. One of them is not a metrics bug at all; it is a billing bug that has been silently live the whole time.

### Defect A — App compute is never metered (billing)

`lego/backend/internal/usage/service.go:808` meters an App's `instance_seconds` with:

```go
quantity, ok = s.queryInstanceSeconds(ctx, app.Name, s.AppNamespace(app.TenantID), window, end)
```

`app` here is a `store.App`, whose `Name` is the **workspace-scoped public service name** (`lego/backend/internal/store/store.go:96-102` — "Slug is the globally-unique public subdomain … distinct from Name which is only workspace-unique"). But the App's Kubernetes objects are named `core.CRName(tenantID, name)` = `tenant + "-" + name` (`lego/backend/internal/core/base.go:1379`, applied at `lego/backend/internal/apps/service.go:1736`), and the Deployment takes that name verbatim (`lego/operator/internal/controller/app_controller.go:1751`). So `queryInstanceSeconds` builds

```
pod=~"block-eden-mono-[a-z0-9]+-[a-z0-9]{5}"
```

against pods that are actually named `tea-d98210cbbpdc73dcrkvg-block-eden-mono-5f557c45fd-ll7x9`. PromQL regexes are fully anchored, so this matches **zero pods for every App, at every name length** — the meter has never measured anything.

Live confirmation — in-page authenticated probe (`fetch('https://api.bex.co/graphql', {credentials:'include'})` from inside `dashboard.bex.co`, per this hunt's own Phase-3 trap about bare-UA clients getting Cloudflare `1010`):

```
query { usage { estimatedCost { resources { serviceId serviceName resourceKind
        charges { kind tier unit quantity costUsd } } } } }

charge kinds observed, by resourceKind:
  service    → ["egress_bytes"]                              ← every one of the 11 service rows
  postgres   → ["instance_seconds", "storage_gb_seconds"]
  key_value  → ["instance_seconds", "storage_gb_seconds"]
  sandbox    → ["sandbox_compute_seconds"]

beancount-forum (srv-d9nqg9dcavls73fp8m2g, tier "standard", running 24 days):
  charges = [{ kind: "egress_bytes", quantity: "19.30", unit: "GB", costUsd: "0.29" }]
  — no compute line at all, against a rate of $17.50/mo
    (lego/backend/internal/pricing/pricing.yaml:38-53, `standard: 0.000006659056316591` USD/s)

beancount-forum-db (dpg-d9nqg95cavls73fp8m20), the control:
  charges = [{ kind: "instance_seconds", quantity: "312.00", unit: "hr", tier: "basic-1gb", costUsd: "5.98" },
             { kind: "instance_seconds", quantity: "862.00", unit: "hr", tier: "basic-256mb", costUsd: "5.79" },
             { kind: "storage_gb_seconds", quantity: "0.614", unit: "GB-mo", costUsd: "0.13" }]
```

**The control case verifies the cause, not just the contrast.** Postgres and Key Value go through `queryInstanceSecondsStateful` (`usage/service.go:893`) and are called with `ds.Name` (`usage/service.go:573`) — a datastore CR name that _is_ the object's real Kubernetes name, so their `pod=~"<name>-[0-9]+"` matcher hits. The asymmetry is exactly "which name got passed", which is the claim.

**The consumer makes it silent.** `queryInstanceSecondsByMatcher` returns `(0, true)` for a successful query that matched nothing, and `processAppMeterWindowResult` (`usage/service.go:793-844`) documents that it "persists a successful measurement even when it is zero. Zero is real coverage evidence" — so every hour writes a `usage_hourly` row of `Quantity: 0` with `SourceHealth: healthy`. Nothing in the source-health machinery ever flags it, and the charge tree simply omits a line that was never non-zero.

### Defect B — Kubernetes truncates long pod names out of the two-segment shape (metrics)

The pod selector all three call sites share assumes a Deployment pod is named `<app>-<replicaset-hash>-<5 chars>`:

```go
// lego/backend/internal/metrics/source.go:255-257
matchers := fmt.Sprintf(`namespace=%q,pod=~%q,container!=""`,
    req.Namespace, fmt.Sprintf(`%s-[a-z0-9]+-[a-z0-9]{5}`, egressquery.RegexEscape(req.App)))
```

Kubernetes does not guarantee that shape. `k8s.io/apiserver@v0.35.0/pkg/storage/names/generate.go` (the version this repo pins) is:

```go
const (
	maxNameLength          = 63
	randomLength           = 5
	MaxGeneratedNameLength = maxNameLength - randomLength   // 58
)
func (simpleNameGenerator) GenerateName(base string) string {
	if len(base) > MaxGeneratedNameLength {
		base = base[:MaxGeneratedNameLength]
	}
	return fmt.Sprintf("%s%s", base, utilrand.String(randomLength))
}
```

The ReplicaSet controller passes `base = "<rsName>-"` where `rsName = "<deploymentName>-<podTemplateHash>"`. Once `len(app) + 1 + len(hash) + 1 > 58`, the truncation **eats the separating hyphen**, and the pod name collapses from two segments to one — which the anchored two-segment regex cannot match.

With the tenant prefix (`tea-` + 20-char id + `-` = 25 chars) and `podTemplateHash` typically 9–10 chars, the threshold lands at a **service name of 22 characters** — out of the 30 `ValidAppName` allows (`lego/backend/internal/store/api.go:588`, `^[a-z0-9]([a-z0-9-]{0,28}[a-z0-9])?$`). Because the hash length varies per pod template (6–10 observed live), a service near the boundary can flip between working and broken **across deploys** of the same service.

Live confirmation — pod names straight from Loki's `instance` label, which is the pod name:

```
query { logLabelValues(label: "instance", resource: "<srv>") }

srv-da7o6ovvqdcc73bpn9hg  (qa-20260826-webhook-svc — 23 chars, CR name 48 chars):
  tea-d98210cbbpdc73dcrkvg-qa-20260826-webhook-svc-55d855bcb7sxgd    ← 63 chars, ONE segment
  tea-d98210cbbpdc73dcrkvg-qa-20260826-webhook-svc-6d6cfb74ddh5dr
  tea-d98210cbbpdc73dcrkvg-qa-20260826-webhook-svc-7c964c469k4w25

srv-d9ndt8hmcglc739fkp50  (block-eden-mono — 15 chars, CR name 40 chars):
  tea-d98210cbbpdc73dcrkvg-block-eden-mono-5f557c45fd-ll7x9         ← 57 chars, TWO segments
  … 22 more, every one two-segment

srv-d9nqg9dcavls73fp8m2g  (beancount-forum — 15 chars):
  tea-d98210cbbpdc73dcrkvg-beancount-forum-65b9fd568c-wwtdw         ← TWO segments
```

and the metrics those selectors produce:

```
query { metrics(query: { filters: [{ field: "RESOURCE", values: ["<srv>"] }],
                         name: "<NAME>", start: <t-Nh>, end: <now>, resolution: 60 })
        { unit labels { field value } values { time value } } }

srv-da7o6ovvqdcc73bpn9hg  MEMORY 24h    → 0 series
                          CPU     1h    → 0 series
                          INSTANCES 12h → 0 series
                          HTTP_REQUESTS → 1 series, 19 points, newest 2026-08-27T01:56:37Z
srv-d9ndt8hmcglc739fkp50  MEMORY  1h    → 2 series, newest 2026-08-27T01:57:00Z
                          CPU     1h    → 2 series, newest 2026-08-27T01:57:00Z
                          INSTANCES 1h  → 1 series, newest 2026-08-27T01:57:00Z
srv-d9nqg9dcavls73fp8m2g  MEMORY  1h    → 1 series, newest 2026-08-27T01:57:00Z
srv-d9bkcspg9s7c73d0n8ug  MEMORY 24h    → 1 series, newest 2026-08-27T01:57:00Z
```

`HTTP_REQUESTS` returning data for the *same* service over the *same* window is the discriminator: request metrics go through `traefikServiceLabel(req.Namespace, req.App, req.Port)` (`source.go:271-278`), which reconstructs the k8s Service's real name from the same `req.App` and matches. So namespace and app identity are both correct on this path — only the pod-name *shape* assumption fails. The population check is complete for this workspace: of its six services, `qa-20260826-webhook-svc` (23 chars) is the only name ≥22, and it is the only one missing Application Metrics.

**Defect A is not a special case of Defect B, and fixing A alone would walk straight into B.** Passing `core.CRName(...)` at `usage/service.go:808` makes the usage matcher correct for short-named Apps and then leaves it broken for long-named ones for Defect B's reason. They must land together.

## Root cause

| # | file:line | what is wrong |
| --- | --- | --- |
| A | `lego/backend/internal/usage/service.go:808` | passes `store.App.Name` (workspace-scoped public name) where the matcher needs the Kubernetes object name `core.CRName(app.TenantID, app.Name)` |
| B1 | `lego/backend/internal/metrics/source.go:256` | `pod=~"%s-[a-z0-9]+-[a-z0-9]{5}"` cannot match a pod whose generated name Kubernetes truncated past the hash separator |
| B2 | `lego/backend/internal/usage/service.go:884` | identical matcher in `queryInstanceSeconds` (same defect, reached only once A is fixed) |
| B3 | `lego/operator/internal/controller/autoscale.go:101,104` | identical matcher in `NewPrometheusMetricsReader` |

## Blast radius (exhaustive, counted — not estimated)

`grep -rn '\[a-z0-9\]{5}' lego/ --include='*.go' | grep -v _test` returns **exactly 4 lines in 3 files** — `metrics/source.go:256`, `usage/service.go:884`, `autoscale.go:101` and `:104`. There are no other pod-name-shape selectors in the repo.

- **Metrics — one funnel, three API surfaces.** REST (`metrics/rest.go:92`), GraphQL (`metrics/graphql.go:298`) and MCP (`metrics/mcp.go:88`) all call `MetricsWithQuantiles` → `Metrics` → `resourceMetricRange`/`rangedResourceSeries` → `promResourceQueryFor`. One fix moves all three; there is no per-surface variant to miss.
- **Autoscaling — config-dependent.** `autoscale.go`'s Prometheus reader is explicitly the fallback "used when metrics-server is unavailable"; the primary `NewMetricsServerReader` (`autoscale.go:80-88`) selects pods with `labelSelector=<labelApp>=<app>` and is immune. Which reader production actually runs is **not verified this run** (see Unverified) — t001 must settle it before the milestone claims an autoscaling impact.
- **Resource-type family, enumerated.** web · private · worker · static are all Apps reconciled to a Deployment named `app.Name` (`app_controller.go:1751`) → all exposed to B. Cron is a CronJob named `app.Name` (`app_controller.go:3014`); its Job pods are generated through the same `SimpleNameGenerator` and are exposed to the same truncation — **reasoned from the k8s source, not probed live** (no cron service existed in the QA workspace). Postgres and Key Value use the ordinal matcher `pod=~"%s-[0-9]+"` (`usage/service.go:895`) with a real CR name and are unaffected by both defects — verified live above.
- **Callers that look correct today.** Every short-named service's *metrics* work today **because** the current shape assumption happens to hold for them. They are not evidence the selector is right, and they need regression tests, not just the broken case.

## Adjacent classes

Not an authorization or existence boundary — both defects are identity/selection bugs inside a metering and charting path, so there is no forbidden / unauthenticated / not-found distinction to place. The one class distinction that *does* matter is **"matched nothing" vs "could not query"**: `queryInstanceSecondsByMatcher` currently collapses them into `(0, true)` and persists a healthy zero. Any fix must keep a genuine zero (a scaled-to-zero App) distinguishable from a selector that matched nothing, or the same silence returns under a different cause.

## Look-alike symptoms traced separately (not folded into this root cause)

- **`build_seconds` is also absent from every service's charge tree.** `queryBuildSeconds` (`usage/service.go:1103-1114`) selects Jobs by the **label** `labelBuild: appName`, not by pod name — a different mechanism entirely. It shares the "which name gets passed" *shape* of Defect A but has its own `file:line` and its own unverified cause. Tracked as its own investigation inside t002, not asserted here.
- **Some services report `runtime: ""`** over GraphQL (`agentmarketcap-1`, `beancount-cms-v2`, `eden-cms-v2`), so the project resource table renders "Not available" and REST omits `serviceDetails.env`/`envSpecificDetails`/`runtime` entirely. Separate claim, separate surface, in the `w6/m109` Render-required-field family. Not filed this run; recorded here so a later hunt does not attach it to this cause.

## Unverified (carried forward, not asserted as fact)

- Which autoscale `MetricsReader` production runs (metrics-server → unaffected; `BEX_PROM_URL` fallback → affected). t001 settles it; until then the autoscaling impact is a hypothesis, not a finding.
- Whether cron-job pods truncate as reasoned — derived from `SimpleNameGenerator`, never observed live.
- Whether Stripe's `$30.01` "Total month to date" already collects App compute by some route outside `usage_hourly` — the estimate tree and the Stripe figure are independently sourced and cover different windows (see `w6/050`), so the absence of an `instance_seconds` line proves bex's own meter is empty, not that no money changed hands.
- How far back the historical `usage_hourly` zeros go, and whether re-metering is even possible within Prometheus' retention. t004 must establish this before choosing a repair.

## Tasks (in order)

| id   | title                                                                                                     | est | depends_on |
| ---- | --------------------------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Settle the pod-identity selector: name-shape tolerance vs a kube-state-metrics join, and which autoscale reader production runs — **DONE** | 40m | —          |
| t002 | Usage meter: select App pods by the Kubernetes object name, not the workspace-scoped store name — **DONE**             | 45m | t001       |
| t003 | Make all three cAdvisor pod selectors survive Kubernetes' 58-char generateName truncation — **DONE**                    | 45m | t001       |
| t004 | Decide and implement the repair for `usage_hourly` instance-seconds rows already persisted as healthy zeros — **DONE**  | 40m | t002       |
| t005 | Render parity — metrics across REST/GraphQL/MCP + the dashboard charge tree — **DONE** (found + fixed an MCP empty-case divergence: `{"series":null}` vs REST's `[]`; the `/billing` charge-tree bullet carries to t008's live sweep) | 30m | t003, t004 |
| t006 | Simplify — `/simplify` over the code this milestone changed — **DONE**                                       | 20m | t005       |
| t007 | Test coverage — **DONE**                                                                                                | 45m | t005       |
| t008 | Closeout                                                                                                     | 15m | t007       |

## Resolution (landed 2026-08-27)

Both filed defects reproduced from the source before anything was changed, and both are real. A **third instance of Defect A** turned up in the same pass and is fixed with it.

### What was wrong, confirmed in-repo

| #   | file:line                  | verified                                                                                                                                                                                                                                                                                                                                                                                                                                                       |
| --- | -------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| A   | `usage/service.go:808`     | `store.App.Name` is written from `req.Name` (`apps/service.go:1917`) — the bare workspace-scoped name — while the CR, and therefore the Deployment, and therefore the pods, are named `core.CRName(tenantID, name)` (`apps/service.go:1735`). The metrics path was already correct for the mirror reason: `metrics/service.go:615` passes `app.Name` off an `*appv1alpha1.App`, which IS the object name. The asymmetry is exactly "which name got passed".        |
| A′  | `usage/service.go:1103`    | `queryBuildSeconds` had the **same** wrong name **and** the wrong namespace: the operator stamps `app.bex.co/build` with the App CR's name (`app_controller.go:770` → `build/build.go:980`) and runs the Job in `BEX_BUILD_NAMESPACE` (`bex-build`, `config/api/deployment.yaml:329`), while the meter listed by the store name in `s.Namespace`. So `build_seconds` was never metered either — the milestone parked this as a look-alike; it is the same cause.  |
| B   | 4 lines / 3 files          | `k8s.io/apiserver@v0.35.0/pkg/storage/names/generate.go` reads exactly as filed (`maxNameLength = 63`, `randomLength = 5`, truncate-then-append).                                                                                                                                                                                                                                                                                                               |

### Decisions

**t001 — selector.** Chose **(a) name-shape tolerance**, refined: rather than the filed `(-[a-z0-9]{5})?`, the truncated alternative is pinned to its exact length, `<obj>-[a-z0-9]{62-len(obj)}`. A truncated pod name is always exactly 63 chars (58 kept + 5 appended), so the length is known, and exact-length + a hyphen-free class is what keeps a sibling App out: any other workload whose name starts with `<obj>-` carries a hyphen after its own extra segment and can never fill N alphanumerics. The open `+` form gives that guarantee up in one narrow case (a sibling whose truncated remainder happens to be exactly 5 chars). Rejected **(b) the kube-state-metrics join**: it buys immunity to Kubernetes' generator at the cost of a join on every metrics read and a dependency on kube-state-metrics staying scraped, and it cannot be tested without a live cluster — where (a) is exhaustively testable against the generator itself.

**t001 — shared helper: yes.** `egressquery.PodNameRegex` / `PodNameMatcher` (`lego/backend/internal/egressquery/podname.go`) is the single definition for the backend's two call sites — three copies of one PromQL fragment is what let this drift. The operator imports no backend package, so `controller.podNameRegex` is a deliberate second copy with a comment binding it to the first; a test pins both to the same live pod names.

**t001 — which autoscale reader production runs: metrics-server.** `cmd/manager/main.go:424` constructs `NewMetricsServerReader(cs)` and nothing else; `NewPrometheusMetricsReader` is not wired anywhere in the manager (`BEX_PROM_URL` at `main.go:498` feeds the _database_ disk-usage reader, not autoscaling). So the autoscaling impact was **not** live — the reader is a latent fallback. Fixed anyway, since a cluster without metrics-server would hit it silently.

**t004 — historical `usage_hourly` zeros: (a) leave history as-is.** Prometheus retention is **`3d`** (`deploy/gitops/base/prometheus.yaml:128`), so every window older than 72 hours is unrecoverable at any price — that is essentially the entire affected range. Of the 72 h that survive, `BEX_STRIPE_SEAL_HOURS` defaults to 48 (ADR040 §3), so two thirds of it is already sealed and exported; re-metering it would either double-count against Stripe or need suppression plumbing built for a single day of recovery. The remaining ≤24 h would additionally need a cursor-rewind mechanism that does not exist. The error direction is under-billing, never over-billing, so no customer correction is owed. Recorded here rather than implied, per the task's own instruction.

**DoD bullet 6 — zero vs unmatched.** An empty Prometheus vector cannot, by itself, distinguish "the selector matched nothing" from "the App is genuinely at zero replicas", so the discriminator is the App's own desired state. `instance_seconds` now writes the zero row (a genuinely-down App must not stall the cursor forever) but records the source observation as **`degraded`** instead of `healthy` when the App is not suspended, not a cron job, and has `spec.replicas > 0`. `degraded` already flows into `UsageCoverage.DegradedSources` and clears `Complete` (`store/usage.go`), which is the signal that would have surfaced Defect A on day one. An App that cannot be identified at all is `unavailable` with no row, so the cursor retries the window.

### What changed

- `lego/backend/internal/egressquery/podname.go` (new) — `PodNameRegex` / `PodNameMatcher`, with the generator's constants and the truncation reasoning inline.
- `lego/backend/internal/metrics/source.go` — `promResourceQueryFor` uses the shared matcher (moves REST, GraphQL and MCP together; they share one funnel).
- `lego/backend/internal/usage/service.go` — `resolveAppCR` extracted from the egress path and reused; `appPodIdentity` returns the CR's real name + namespace with a `CRName`/`AppNamespace` fallback; `queryInstanceSeconds` and `queryBuildSeconds` take the object name; `expectsRunningPods` + `degradeSourceHealth` implement the suspicious-zero signal; `BuildNamespace` field added.
- `lego/backend/cmd/api/main.go` — wires `usageSvc.BuildNamespace` from `BEX_BUILD_NAMESPACE`.
- `lego/operator/internal/controller/autoscale.go` — `podNameRegex` helper, both queries switched to it.
- `docs/ADR010-observability.md` — the heuristic paragraph and the selector line now describe both shapes and say `<obj>` is the Kubernetes object name.
- Tests: `egressquery/podname_test.go` (new; includes a sweep that reproduces `SimpleNameGenerator` for every service-name length 1–30 × hash length 6–10 and asserts the selector matches every name it generates), `metrics/range_test.go`, `usage/service_test.go`, `operator/internal/controller/autoscale_podname_test.go`.

`make test`, `cd lego/backend && go test ./...`, and `make lint` (all four modules) pass.

### Still open

- **t005 / DoD bullets 2–5** need the fixed bex-api deployed: they are live assertions against `dashboard.bex.co`. The code-side half of t005 is structural — REST/GraphQL/MCP reach one funnel, so they cannot disagree — but that has not been observed on the live product.
- **t006** (`/simplify`) and **t008** (closeout) not run.

## Definition of done

Each bullet is a command or a click the next person can repeat and watch pass or fail.

1. **App compute is metered.** For a running paid-tier service, the in-page probe `{ usage { estimatedCost { resources { serviceName resourceKind charges { kind quantity unit costUsd } } } } }` returns a charge whose `kind` is `instance_seconds` on a `resourceKind: "service"` row, with `quantity` matching the hours it has actually been running (± one rollup window). Today every service row contains only `egress_bytes`.
2. **The selector matches truncated pod names.** For `srv-da7o6ovvqdcc73bpn9hg` (`qa-20260826-webhook-svc`, 23 chars, pods `tea-…-svc-55d855bcb7sxgd`), `{ metrics(query: { filters: [{field:"RESOURCE", values:["srv-da7o6ovvqdcc73bpn9hg"]}], name:"MEMORY", … }) { values { time value } } }` returns ≥1 series with points inside the requested window. Today it returns 0 series.
3. **The Metrics page agrees.** `https://dashboard.bex.co/services/srv-da7o6ovvqdcc73bpn9hg/metrics` renders numbers in Memory, CPU and Total Instances instead of "No data in range", at the same time as the Network cards.
4. **A newly-created long name works too.** A fresh service named with ≥22 characters shows Application Metrics within one scrape window of going Live, and shows them again after a redeploy (a new pod-template hash must not flip the behavior).
5. **Short names did not regress.** `block-eden-mono` (`srv-d9ndt8hmcglc739fkp50`) and `beancount-forum` (`srv-d9nqg9dcavls73fp8m2g`) still return MEMORY/CPU/INSTANCES series and still render all six Metrics cards, and no series belonging to a *different* App appears in either one's `instance` labels.
6. **Zero stays distinguishable from unmatched.** A unit test asserts that a selector matching no pods is not recorded as a healthy zero `usage_hourly` row; an App genuinely at zero replicas still is.
7. **All three metrics surfaces move together.** REST `/v1/metrics/memory`, GraphQL `metrics(name:"MEMORY")` and MCP's metrics tool each return the same non-empty series for the service in bullet 2.
8. The historical-zero repair decision from t004 is written down in the milestone with what was chosen and why — including "do nothing" if that is the answer.

## Source + Goal linkage

- **Source:** live `/qa-find-bugs` hunt of `https://dashboard.bex.co`, 21st run, 2026-08-27. Probes and captures are pasted inline above (the durable artifact for an API contract is the request and the full response, not a screenshot); working screenshots for this run are under `.playwright-mcp/` and are gitignored, session-local, and deliberately not load-bearing for any bullet here.
- **Goal linkage:** ADR008 pillar "the open-source Render alternative" — metering and observability are hosting primitives, not extras. `docs/ADR030-pricing.md` and `docs/ADR040-billing-metronome.md` define compute as a per-second metered kind; `docs/ADR010-observability.md` owns the cAdvisor selector heuristic this milestone corrects, and its documented rationale ("anchored and two-segment, so app `web` never matches a `web-api-…-…` pod") needs updating to whatever selector wins t001.
- **Expected outcome:** App compute appears in the charge tree and on invoices for the first time; the Metrics page stops silently blanking for a third of the allowed service-name space; the autoscaler's Prometheus fallback stops reading an empty series for those same services.
- **Why now:** the billing half is revenue-affecting and has been silently wrong for the entire life of the meter, with a `SourceHealth: healthy` signal actively asserting that it is fine. The metrics half is a correctness bug whose trigger is an ordinary user choice (a 22-character service name) and whose failure mode is an honest-looking empty state, so nobody reports it. Both live in the same four lines of PromQL, so fixing them separately would mean touching the same selectors twice.
- **Render parity task included:** yes — the change moves `metrics` across REST, GraphQL and MCP, alters the `/billing` charge tree the dashboard renders, and touches a field family (`instance_seconds`) that Render exposes on its own metrics and billing surfaces.
