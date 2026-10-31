# Metrics loading changes chart geometry, and byte ticks are clipped

Why: opening an owned datastore's Metrics tab rearranges the card at both widths; a correct returned memory measurement also loses part of its axis label.

Source: functional $qa-find-bugs w4, 2026-10-14 pass a470. **Severity: minor** for each of two independently traced findings. Filing only. [Complete requests, responses, DOM measurements and cleanup](../evidence/m209-keyvalue-metrics-geometry.json).

## Reproductions

Owned private Free Valkey 8: `red-db7qh19llfps73emudq0`, `qa-20261014-a470-metrics`. From a fresh dashboard home, click its existing resource link and the **Metrics** tab. Hold the actual successful response unchanged for seven seconds, then release it. Repeat at 1440×1000 and 390×844.

| State held | Desktop pending → populated ready | Mobile pending → populated ready |
| --- | --- | --- |
| Resource query | Card 768 → 998.5625; header 92 → 40 | Card 812 → 514.390625; header 136 → 40 |
| Four metric queries | Card 770 → 998.5625; each plot 180 → 256.1875 | Card 770 → 514.390625; each plot 180 → 94.796875 |

The first state is a complete one-operation **KeyValue** HTTP batch, not a selected projection. The second is a complete four-operation **DatastoreMetrics** batch: DISK, DISK_CAPACITY, MEMORY, CONNECTIONS. All return 200 without GraphQL errors. Bodies are recorded completely; no headers or credentials. The series identify the owned resource and contain nonempty positive memory/disk samples and connection count 1. No mutation, generated sample or response substitution creates these failures.

A fresh 1440px reload independently reproduces the second finding: the Memory axis prints **536.7 KiB**, but its native SVG `getBBox().x` is **-4.1895566** viewBox units. Its right anchor is x38 and rendered width42.1895561. The narrow capture separately measures x-4.1592245. **26.0 KiB** is the control at x1.2446954 after the desktop reload. Both receive the same 9px font. The actual numeric samples and DOM text are correct; the SVG clips the beginning of the wider label.

The shorter viewport clips some element screenshots through the scroll container. Those are excluded as complete visual pairs. Replays at the **same widths**, with viewport height1400, reproduce identical card/plot geometry and provide eight complete crops, all inspected:

- `.playwright-mcp/qa-a470-metrics-{pending,ready}-{desktop,mobile}-complete.png`
- `.playwright-mcp/qa-a470-series-{pending,ready}-{desktop,mobile}-complete.png`

Early initial Metrics settled to an honest “No data in range” card at height770 before telemetry arrived; only DOM was retained for that early state. Error, throttle, unavailable, HA and multiseries transitions were **not induced live**.

## Finding 1: two distinct loading producers

**Resource preview:** `common/components/route-skeletons.tsx:2186–2197` mounts `MetricsCardSkeleton(sections=3, compact)`. The producer at1442–1480 unconditionally reserves three h9 filter controls that `DatastoreMetricsPanel:126–134` does not render. Each section adds a 28px trailing placeholder and a **154px** plot. Actual datastore headings are native text rows; populated plots scale with available width. The one production DatastoreMetricsSkeleton mount is `routes/keyvalue.$keyValueId.tsx:275`. Its type accepts database, but there is **no production database mount**: Postgres instead mounts its panel through DeferredMount/Suspense at `routes/databases.$databaseId.tsx:287–305`. Do not conflate or claim that untested producer fixed.

**Mounted chart loading:** `features/metrics/components/metric-section.tsx:43–57` uses fixed `CHART_HEIGHT=180` for initial-loading Skeletons. `svg-line-chart.tsx:100–106` renders a width100% SVG, viewBox **600×180**, with no fixed CSS height. Its intrinsic aspect ratio scales the actual height: 854×180/600≈256.2 desktop, 316×180/600≈94.8 mobile. Browser CSS inspection returns `.w-full { width: 100%; }`; computed SVG aspectRatio is auto. `chart-layout.ts:8–10` supplies the shared canvas, while `ui/card.tsx:5–68` supplies the measured card/header/content padding and gaps. This is native React/SVG/CSS, not an external chart library.

The [SVG 2 intrinsic-sizing algorithm](https://www.w3.org/TR/SVG2/coords.html#IntrinsicSizing) derives the ratio from viewBox when fixed sizing does not establish it. The captured rectangles confirm that mechanism. A canvas coordinate of180 does not mean180 CSS pixels. The bar sibling independently uses the same viewBox/width-only markup at `svg-bar-chart.tsx:100–104`.

**Target:** preserve the existing populated **600:180 responsive plot viewport**. Reuse that ratio for loading, resolved-empty, unavailable and error frames; remove internal fixed-height empty overrides in line/bar renderers while retaining intentionally supplied public height overrides if needed. Use the existing constants, not per-breakpoint approximations. Keep chart content, units, queries, domains, tooltips and marker transforms. Keep error/unavailable/throttle meanings and cached populated charts during polling. Reserve known heading/extras/legend/marker-strip slots in their actual caller layout; do not add fake controls, fake series or success states. For the exercised single-series KV panel, the corrected preview and initial-metric-loading state should retain the populated card geometry: **998.5625 desktop / 514.390625 mobile**, header40, plot256.1875/94.796875 (allow normal pixel rounding). Capacity arrives later but must not change the heading-row height.

## Finding 2: tick gutter assumes shorter labels

`chart-layout.ts:10` fixes left padding44. `svg-line-chart.tsx:132–139` places end-anchored formatted ticks at left-6 = **38**. The byte formatter `lib/format.ts:25–28,45–53` legitimately returns536.7 KiB. That 42px label exceeds the available38px; CSS scales the already-clipped SVG equally at both widths. The bar sibling separately uses left-6 and the same formatter at `svg-bar-chart.tsx:119–126`.

**Target:** keep the complete formatted tick inside the canvas with a small readable left inset. Size the plot gutter against the actual formatted tick widths at the rendered axis font, with a bounded sensible fallback before measurement; do not truncate the label, alter samples, reduce the existing measurement precision, expose content outside the card, or shrink the font. For this recorded label, left≥52 viewBox units leaves a 4px inset; that is a measured minimum for this case, **not a universal constant guarantee**. Include long bytes, counts, latency and adaptive CPU/percentage labels in browser verification. Feed any changed left bound consistently into grid starts, line/bar geometry, time mapping, reference lines, tooltip hit testing and marker clustering. Existing geometry functions use numeric bounds and can accept a computed left pad; derive it once rather than changing only text anchors.

## Shared consumer census and limits

Exhaustive production JSX/import search, excluding tests:

| Producer | Production JSX sites |
| --- | --: |
| MetricSection | 11: application2, network3, datastore4, scaling2 |
| SvgLineChart | 11: application2, network2, datastore5, scaling2 |
| SvgBarChart | 1: network request-volume |
| EmptyChart | 9: scaling4, line1, bar1, datastore1, application2 |
| MetricError / MetricUnavailable | 3 each: MetricSection2 + direct datastore replication1 |
| DatastoreMetricsPanel | 3: Key Value route, Postgres route, service DiskSection |
| DatastoreMetricsSkeleton | 1: Key Value route |
| MetricsCardSkeleton | 4: ServiceMetricsSkeleton2, service Scaling preview1, datastore1 |

CHART_PAD has **three** production imports: line renderer, bar renderer, chart-event-markers. The last uses top/bottom for event lines; x clusters are produced by the renderers and the HTML strip positions them against CHART_WIDTH. Preserve that alignment. Fix the KV-specific route structure at its datastore producer; do not remove the filters used by the other three MetricsCardSkeleton sites. Their geometry, Postgres deferred/lazy states, service disk extras, multiseries legends, service event strips, scaling percentage/unavailable controls and bar interactions are **source-traced verification tasks**, not claimed live failures.

Routes: canonical `/keyvalue/$id?tab=metrics`; legacy `/r/$id` redirects to the same resource route. Postgres canonical/legacy d routes and service/static/cron/worker/private families keep their own placement/gating. No backend mutation or new metrics capability is proposed. Private/no samples means an honest empty frame; unavailable/error/429 remain distinguishable; read-only permissions and missing resources retain existing routing.

## Dedupe, history and prior guarantees

Searched open/blocked/done PM text for chart height/aspect/geometry, MetricsCardSkeleton, clipping/tick/gutter/CHART_PAD; scanned **66 actual open/blocked milestone READMEs**, anti-goals, recent code history and targeted producer/padding history. No matching open owner or landed fix found. Original width-only renderer predates the skeletons; this is an uncovered original geometry guarantee, not a claimed later regression.

- **w9/m63 t002:** first-fetch shimmer passes, genuine early empty UI passes, datastore shared path passes, cached-poll retention was not deliberately delayed this run. Its claimed fixed-height/no-shift premise fails against real SVG geometry. Full milestone DoD disposition: log virtualization was explicitly deferred; ANSI parsing/follow/selection/performance, events, Logs/Scaling pending and the entire dashboard suite were not re-certified.
- **w5/m79 t009:** both canonical resource and chart-pending KV Metrics states fail the shared geometry gate. Outer card widths remain896/358; global navigation/chrome was not used to calculate card deltas. Metadata/Connections, policy, Networking and plan loading stay with w4/298,299,303,m208. Create URL, ID transitions, all lazy panels and all three original owned URLs are unverified here. t016 service Logs/Metrics is source audit only; no claim to recertify its DoD.
- **w4/m196:** service plan Free/Paid grouping, separate from charts.
- **w5/m90:** percentage/replica/limit contract preserved; no incorrect samples or denominator claim. **w4/296:** adaptive numerical precision remains separate; do not “fix” clipping by reverting it. **w4/297:** translated chart labels remain separate.
- **w4/m175/m203:** request-window clipping/selection time, not glyph clipping or chart layout. No anti-goal match.

[Render Service Metrics](https://render.com/docs/service-metrics) documents datastore graphs and graph controls, including KV connections; it does not establish a particular loading animation, dimensions or this bug. Preserve the existing Bex datastore metric-set divergences and three-surface data contracts. Authenticated Render behavior was not tested. Parity closing task is included for this UI change; no new datastore telemetry/API feature is justified by it.

## Cleanup

Native DeleteKeyValue returned true at **16:16:56.519 UTC**. Detail REST404 and all seven exact baseline ID sets restored at **16:17:22.416 UTC**. Own logout completed, whoami401 / cookies0 / one blank page at **16:18:21.531 UTC**. Own observers and handlers removed, only stat-matched session files removed. A transient ERR_CONNECTION_CLOSED disappeared on fresh navigation and was not filed as a network failure. No baseline resource, policy, stored key or paid plan changed.
