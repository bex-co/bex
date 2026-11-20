/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package egressquery owns the Prometheus metric vocabulary and PromQL used to
// compose tenant outbound bytes. Keeping it below both usage and metrics makes
// the durable rollup and the Render-shaped live views use identical sources.
package egressquery

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

type Source string

const (
	HTTP      Source = "http"
	Direct    Source = "direct"
	WebSocket Source = "websocket"
	Postgres  Source = "postgres"
	KeyValue  Source = "key_value"
)

type Spec struct {
	Source        Source
	Counter       string
	Matchers      string
	Job           string
	HealthMetric  string
	ProcessMetric string
	// NewSeriesFromZero marks a counter whose exporter creates each series at
	// zero, writes every series it holds on every scrape, and deletes a series
	// for good when it goes away, so a later series with the same labels
	// starts from zero again. Billed counts such a series from zero when it
	// first appears inside the window. Verified for Traefik's router counters
	// (w6/m455 t001) and for the process-local counters bex exports itself:
	// the WebSocket plugin and the Postgres and Key Value SNI proxies never
	// delete a series while their process lives (w6/133). The direct meter is
	// not: it restores a checkpointed counter after a restart.
	NewSeriesFromZero bool
	// DrainedMetric is a gauge each scrape target's process sets to 1 once it
	// counts nothing more: a drained SNI proxy sets it after its last stream
	// ends, and its last scrape reads it (w6/155). A source with one names an
	// exporter that is one process per target, with counters from zero, so
	// Health judges each target on its own (targetHealth) and Billed counts a
	// target's series from zero when its process started inside the window.
	DrainedMetric string
}

func App(appID string, routers []string, direct bool) []Spec {
	var out []Spec
	if matcher := routerMatcher(routers); matcher != "" {
		out = append(out, Spec{
			Source: HTTP, Counter: "traefik_router_responses_bytes_total", Matchers: fmt.Sprintf(`router=~%q`, matcher),
			Job: "traefik", ProcessMetric: `process_start_time_seconds{job="traefik"}`, NewSeriesFromZero: true,
		})
		// static_site has a router but no tenant connection to hijack and the
		// operator deliberately does not install the WebSocket middleware.
		if direct {
			out = append(out, Spec{
				Source: WebSocket, Counter: "bex_websocket_egress_bytes_total", Matchers: fmt.Sprintf(`app_id=%q`, appID),
				Job: "traefik-websocket-meter", HealthMetric: "bex_websocket_meter_healthy",
				ProcessMetric: "bex_websocket_meter_process_start_time_seconds", NewSeriesFromZero: true,
			})
		}
	}
	if direct {
		out = append(out, Spec{Source: Direct, Counter: "bex_app_direct_egress_bytes_total", Matchers: fmt.Sprintf(`app_id=%q`, appID), Job: "bex-egress-meter", HealthMetric: "bex_egress_meter_healthy"})
	}
	return out
}

func Datastore(resourceID, kind string, public bool) []Spec {
	if !public {
		return nil
	}
	switch kind {
	case "postgres":
		return []Spec{{
			Source: Postgres, Counter: "bex_pg_proxy_egress_bytes_total", Matchers: fmt.Sprintf(`resource_id=%q,resource_kind="postgres"`, resourceID),
			Job: "bex-pg-proxy", HealthMetric: `bex_pg_proxy_healthy{resource_kind="postgres"}`,
			ProcessMetric: `bex_pg_proxy_process_start_time_seconds{resource_kind="postgres"}`, NewSeriesFromZero: true,
			DrainedMetric: `bex_pg_proxy_drained{resource_kind="postgres"}`,
		}}
	case "key_value":
		return []Spec{{
			Source: KeyValue, Counter: "bex_kv_proxy_egress_bytes_total", Matchers: fmt.Sprintf(`resource_id=%q,resource_kind="key_value"`, resourceID),
			Job: "bex-kv-proxy", HealthMetric: `bex_kv_proxy_healthy{resource_kind="key_value"}`,
			ProcessMetric: `bex_kv_proxy_process_start_time_seconds{resource_kind="key_value"}`, NewSeriesFromZero: true,
			DrainedMetric: `bex_kv_proxy_drained{resource_kind="key_value"}`,
		}}
	default:
		return nil
	}
}

// Health rejects an accounting window if a target was down, a health signal
// fell to zero, or either series lacks at least 80% of its expected 15-second
// samples. The sample-coverage term is what makes discovery gaps and process /
// pod replacement visible: Prometheus otherwise has no zero sample during a
// missing interval and increase() would silently bridge it.
func Health(spec Spec, seconds int64) string {
	if seconds < 15 {
		seconds = 15
	}
	expected := seconds * 8 / (15 * 10)
	if expected < 1 {
		expected = 1
	}
	if spec.Source == Direct {
		return directHealth(spec, seconds, expected)
	}
	if spec.DrainedMetric != "" {
		return targetHealth(spec, seconds, expected)
	}
	up := fmt.Sprintf(`up{job=%q}`, spec.Job)
	parts := []string{
		fmt.Sprintf(`min(min_over_time(%s[%ds]))`, up, seconds),
		fmt.Sprintf(`(min(count_over_time(%s[%ds])) >= bool %d)`, up, seconds, expected),
	}
	if spec.HealthMetric != "" {
		parts = append(parts,
			fmt.Sprintf(`min(min_over_time(%s[%ds]))`, spec.HealthMetric, seconds),
			fmt.Sprintf(`(min(count_over_time(%s[%ds])) >= bool %d)`, spec.HealthMetric, seconds, expected),
		)
	}
	// A reset can discard bytes written after the last scrape. Reject it for
	// every source rather than asking increase() to guess an unknowable delta.
	parts = append(parts,
		fmt.Sprintf(`((sum(resets(%s{%s}[%ds])) or vector(0)) == bool 0)`, spec.Counter, spec.Matchers, seconds),
	)
	// Process-local counters can restart before the first scrape in a window,
	// where resets() has no preceding in-range sample. The latest start time
	// closes that boundary case. The direct meter is node-persistent instead.
	if spec.ProcessMetric != "" {
		parts = append(parts,
			fmt.Sprintf(`(max(max_over_time(%s[%ds])) < bool (time() - %d))`, spec.ProcessMetric, seconds, seconds),
		)
	}
	return strings.Join(parts, " * ")
}

// directHealth judges the direct source on the meters that hold this app's
// counter (w4/442), not the whole fleet. Each node meter keeps a persistent,
// monotonic counter per app it has seen, so a missed scrape loses no bytes:
// increase() across the gap is exact, and only a counter loss or decrease can
// misstate it (resets and the loss time below). The fleet-wide up term failed
// every app's hour on one missed scrape anywhere, billing that hour's direct
// egress as zero. What still rejects the hour is the evidence the app's bytes
// depend on: a meter holding its counter reporting unhealthy hooks or
// attribution, or too few of that meter's health samples, or a loss on it. An
// app no meter has counted yet is judged on the whole fleet's health, so a
// broken meter can never pass as an app with no traffic.
func directHealth(spec Spec, seconds, expected int64) string {
	counter := fmt.Sprintf(`%s{%s}`, spec.Counter, spec.Matchers)
	holders := fmt.Sprintf(`group by (instance) (count_over_time(%s[%ds]))`, counter, seconds)
	scoped := func(agg, series string) string {
		return fmt.Sprintf(`(%s(%s and on(instance) %s) or %s(%s))`, agg, series, holders, agg, series)
	}
	healthMin := fmt.Sprintf(`min_over_time(%s[%ds])`, spec.HealthMetric, seconds)
	healthCount := fmt.Sprintf(`count_over_time(%s[%ds])`, spec.HealthMetric, seconds)
	const loss = "bex_egress_meter_last_counter_loss_time_seconds"
	return strings.Join([]string{
		scoped("min", healthMin),
		fmt.Sprintf(`((min(%s and on(instance) %s) >= bool %d) or (min(%s) >= bool %d))`, healthCount, holders, expected, healthCount, expected),
		fmt.Sprintf(`((sum(resets(%s[%ds])) or vector(0)) == bool 0)`, counter, seconds),
		fmt.Sprintf(`((max(%s and on(instance) %s) or max(%s) or vector(0)) < bool (time() - %d))`, loss, holders, loss, seconds),
	}, " * ")
}

// targetHealth judges an SNI proxy source one scrape target, one pod, at a
// time (w6/155). Judged as one fleet, every public store's hour failed when a
// pod started or exited in it: a roll starts a new pod on each node, which
// failed the process-start term, and an old pod that exited before minute 48
// had too few samples. What a process-local counter can lose is what it
// counted after its process's last scrape, so a pod passes when nothing it
// counted in the window escaped a scrape:
//
//   - A pod that ran as one process from before the window and is still up
//     at its end is judged as every pod was before: up at every scrape, with
//     80% of the window's samples for its up and health series.
//   - A pod that ran from before the window and drained in it passes when its
//     last successful scrape read the drained gauge at 1, so its counters did
//     not move after that scrape, and it has 80% of the samples from the
//     window's start to its last scrape. A failed scrape after the drained
//     one, of a target that outlived its process, loses nothing.
//   - A new pod counts from zero (Billed): one process that started inside
//     the window, in a pod that had no scrape in the 30 s before the window
//     while its job had one. A restarted container is never new: its process
//     start changed in the window or its pod was scraped just before. A new
//     pod passes when it is up at its last scrape at the window's end, or
//     drained.
//
// Every pod's health gauge reads 1 at every scrape, and a reset in the
// store's counter still fails the hour. A pod that vanished without a drained
// scrape, or whose scrapes stopped before the window's end, fails it. A
// window with no target has no answer, as before.
func targetHealth(spec Spec, seconds, expected int64) string {
	t := newTargetTerms(spec, seconds, expected)
	return strings.Join([]string{
		fmt.Sprintf(`(count(%s) > bool 0)`, t.targets),
		fmt.Sprintf(`((count(%s unless on(instance, pod) %s) or vector(0)) == bool 0)`, t.targets, t.good()),
		fmt.Sprintf(`((sum(resets(%s{%s}[%ds])) or vector(0)) == bool 0)`, spec.Counter, spec.Matchers, seconds),
	}, " * ")
}

// targetTerms are targetHealth's conditions, each a filter that keeps the
// targets, by instance and pod, that meet it.
type targetTerms struct {
	targets, healthy, old, born, full, covered, alive, drained string
}

func newTargetTerms(spec Spec, seconds, expected int64) targetTerms {
	up := fmt.Sprintf(`up{job=%q}`, spec.Job)
	// per reads one value per target from a range function over series.
	per := func(agg, fn, series string, window int64) string {
		return fmt.Sprintf(`%s by (instance, pod) (%s(%s[%ds]))`, agg, fn, series, window)
	}
	start := fmt.Sprintf(`(time() - %d)`, seconds)
	before := fmt.Sprintf(`count_over_time(%s[%ds] offset %ds)`, up, newSeriesGuard, seconds)
	samples := per("min", "count_over_time", up, seconds)
	// The timestamp of each target's last scrape, a failed one included.
	last := fmt.Sprintf(`max by (instance, pod) (max_over_time(timestamp(%s)[%ds:15s]))`, up, seconds)
	return targetTerms{
		targets: fmt.Sprintf(`group by (instance, pod) (count_over_time(%s[%ds]))`, up, seconds),
		healthy: fmt.Sprintf(`(%s == 1)`, per("min", "min_over_time", spec.HealthMetric, seconds)),
		old:     fmt.Sprintf(`(%s < %s)`, per("max", "max_over_time", spec.ProcessMetric, seconds), start),
		born: fmt.Sprintf(`((%s >= %s) and on(instance, pod) (%s == 0) unless on(pod) %s and on() %s)`,
			per("min", "min_over_time", spec.ProcessMetric, seconds), start, per("max", "changes", spec.ProcessMetric, seconds), before, before),
		full: fmt.Sprintf(`((%s == 1) and on(instance, pod) (%s >= %d) and on(instance, pod) (%s >= %d))`,
			per("min", "min_over_time", up, seconds), samples, expected, per("min", "count_over_time", spec.HealthMetric, seconds), expected),
		covered: fmt.Sprintf(`(%s >= on(instance, pod) ((%s - %s) * 8 / 150))`, samples, last, start),
		alive:   fmt.Sprintf(`(%s == 1)`, per("max", "last_over_time", up, newSeriesGuard)),
		drained: fmt.Sprintf(`(%s == 1)`, per("max", "last_over_time", spec.DrainedMetric, seconds)),
	}
}

// good keeps every target that passes.
func (t targetTerms) good() string {
	and := func(terms ...string) string { return "(" + strings.Join(terms, " and on(instance, pod) ") + ")" }
	return and(t.healthy, fmt.Sprintf(`(%s or %s or %s)`,
		and(t.old, t.alive, t.full), and(t.old, t.drained, t.covered), and(t.born, "("+t.alive+" or "+t.drained+")")))
}

func Increase(spec Spec, seconds int64) string {
	return fmt.Sprintf(`sum(increase(%s{%s}[%ds]))`, spec.Counter, spec.Matchers, seconds)
}

// newSeriesGuard is how far before a window Billed looks for a successful
// scrape of a series' target: two of the 15 s scrape intervals.
const newSeriesGuard = 30

// Billed is the bytes a source bills over the `seconds` ending at the instant
// it is evaluated at (ADR023 § New series count from zero, w6/m455). For a
// source without NewSeriesFromZero it is Increase.
//
// For one with it, a series that first appears inside the window counts from
// zero: it bills its last value in the window. increase() reads only the growth
// after a series' first sample, so it drops what a new service's router served
// before its first scrape (w6/123: 5.4 MB of a 10 MB probe). A series is new
// only when its target was scraped successfully in the 30 s before the window
// and that scrape did not carry it: the exporter writes every series it holds
// on every scrape, so the series did not exist then and started from zero
// after it. A series missing at the start for any other reason (its target
// not scraped, a failed scrape) keeps increase(), so an old cumulative value
// is never billed as new bytes, and every series that is not new bills exactly
// what Increase bills. A series gone stale before the window ends (a deleted
// service) still counts through its last sample. The health gate is
// unchanged: a reset anywhere in the window still skips the whole source.
//
// For a source with a DrainedMetric, a series is also new when every sample
// its target gave in the window came from a process that started inside the
// window (w6/155): the SNI proxies' counters live in process memory, so every
// series such a process holds started from zero after the window began. That
// is how a pod a roll starts inside the hour bills what it served before its
// first scrape.
func Billed(spec Spec, seconds int64) string {
	if !spec.NewSeriesFromZero {
		return Increase(spec, seconds)
	}
	// `or` keeps every new series and adds increase() for the label sets it
	// does not already hold.
	return fmt.Sprintf(`(sum(%s or increase(%s{%s}[%ds])) or vector(0))`, newSeries(spec, seconds), spec.Counter, spec.Matchers, seconds)
}

// newSeries selects, at its last value, every series Billed counts from zero.
func newSeries(spec Spec, seconds int64) string {
	series := fmt.Sprintf(`%s{%s}`, spec.Counter, spec.Matchers)
	guarded := fmt.Sprintf(`((last_over_time(%[1]s[%[2]ds]) unless last_over_time(%[1]s[%[3]ds] offset %[2]ds)) and on(instance) (max_over_time(up{job=%[4]q}[%[3]ds] offset %[2]ds) == 1))`,
		series, seconds, newSeriesGuard, spec.Job)
	if spec.DrainedMetric == "" {
		return guarded
	}
	return fmt.Sprintf(`(%s or (last_over_time(%s[%ds]) and on(instance, pod) (min_over_time(%s[%ds]) >= (time() - %d))))`,
		guarded, series, seconds, spec.ProcessMetric, seconds, seconds)
}

// CompletedHoursIncrease is the sum of Billed over each whole UTC hour in the
// window ending at the hour-aligned instant it is evaluated at, so it reads
// what the hourly meter bills: the subquery steps on absolute hour boundaries,
// so every hour's term is a fixed window whose value no longer changes once
// the hour has ended. Reading a long window as one increase() instead
// extrapolates over a span that grows with each read, so a flat counter's
// total shrinks between reads (w4/468). A source with no samples yet
// contributes 0.
func CompletedHoursIncrease(spec Spec, hours int64) string {
	return fmt.Sprintf(`(sum_over_time((%s)[%ds:3600s]) or vector(0))`, Billed(spec, 3600), hours*3600)
}

// ExactDelta is the bytes counted over the last `seconds` as the counter's value
// now minus its value `seconds` ago, per series, with a series that did not
// exist yet starting from zero. Unlike increase() it never extrapolates to the
// window edges, so a flat counter reads the same on every later instant and a
// month-to-date figure built from it cannot shrink between reads (w4/477). A
// counter reset clamps that series to zero rather than going negative. Use it
// only for the live current-hour term: billing windows are fixed and keep
// Increase.
func ExactDelta(spec Spec, seconds int64) string {
	series := fmt.Sprintf(`%s{%s}`, spec.Counter, spec.Matchers)
	return fmt.Sprintf(`(sum(clamp_min(%[1]s - (%[1]s offset %[2]ds or %[1]s * 0), 0)) or vector(0))`, series, seconds)
}

// IncreaseOrZero is the chart-grade sibling of Increase: a missing source
// contributes 0 instead of nulling the composed sum.
func IncreaseOrZero(spec Spec, seconds int64) string {
	return fmt.Sprintf(`(%s or vector(0))`, Increase(spec, seconds))
}

// SumIncreases composes per-bucket byte counts (increase over the step) across
// App egress sources. Used by the Outbound Bandwidth chart so summing series
// points over a day reconciles with the month-to-date Increase read. Billing
// metering still calls Increase directly (usage/service.go) — do not route
// money through this helper.
func SumIncreases(specs []Spec, seconds int64) string {
	if len(specs) == 0 {
		return ""
	}
	parts := make([]string, 0, len(specs))
	for _, spec := range specs {
		parts = append(parts, IncreaseOrZero(spec, seconds))
	}
	return strings.Join(parts, " + ")
}

// RouterMatcher is routerMatcher exported for the request-metric builders
// (w4/m113): a static site has no per-App Kubernetes Service, so its request
// count, latency and status-code discovery must select Traefik's per-router
// series by the same anchored, regex-escaped router identities the bandwidth
// read already uses. Empty when there are no routers — callers MUST treat that
// as "no query", never as an unbounded match.
func RouterMatcher(routers []string) string { return routerMatcher(routers) }

func routerMatcher(routers []string) string {
	if len(routers) == 0 {
		return ""
	}
	unique := make(map[string]struct{}, len(routers))
	for _, router := range routers {
		if router != "" {
			unique[router] = struct{}{}
		}
	}
	values := make([]string, 0, len(unique))
	for router := range unique {
		values = append(values, regexp.QuoteMeta(router))
	}
	sort.Strings(values)
	if len(values) == 0 {
		return ""
	}
	return "^(" + strings.Join(values, "|") + ")$"
}
