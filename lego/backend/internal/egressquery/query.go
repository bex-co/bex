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
}

func App(appID string, routers []string, direct bool) []Spec {
	var out []Spec
	if matcher := routerMatcher(routers); matcher != "" {
		out = append(out, Spec{
			Source: HTTP, Counter: "traefik_router_responses_bytes_total", Matchers: fmt.Sprintf(`router=~%q`, matcher),
			Job: "traefik", ProcessMetric: `process_start_time_seconds{job="traefik"}`,
		})
		// static_site has a router but no tenant connection to hijack and the
		// operator deliberately does not install the WebSocket middleware.
		if direct {
			out = append(out, Spec{
				Source: WebSocket, Counter: "bex_websocket_egress_bytes_total", Matchers: fmt.Sprintf(`app_id=%q`, appID),
				Job: "traefik-websocket-meter", HealthMetric: "bex_websocket_meter_healthy",
				ProcessMetric: "bex_websocket_meter_process_start_time_seconds",
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
			ProcessMetric: `bex_pg_proxy_process_start_time_seconds{resource_kind="postgres"}`,
		}}
	case "key_value":
		return []Spec{{
			Source: KeyValue, Counter: "bex_kv_proxy_egress_bytes_total", Matchers: fmt.Sprintf(`resource_id=%q,resource_kind="key_value"`, resourceID),
			Job: "bex-kv-proxy", HealthMetric: `bex_kv_proxy_healthy{resource_kind="key_value"}`,
			ProcessMetric: `bex_kv_proxy_process_start_time_seconds{resource_kind="key_value"}`,
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
	// A direct-meter process restart reopens its pinned monotonic counter. A
	// lost/recreated map can only restore the last node checkpoint, however, so
	// a decrease means the exact delta is unknowable. Reject that entire window
	// instead of letting increase() interpret the restored absolute value as new
	// traffic.
	if spec.Source == Direct {
		parts = append(parts,
			fmt.Sprintf(`((max(bex_egress_meter_last_counter_loss_time_seconds) or vector(0)) < bool (time() - %d))`, seconds),
		)
	}
	return strings.Join(parts, " * ")
}

func Increase(spec Spec, seconds int64) string {
	return fmt.Sprintf(`sum(increase(%s{%s}[%ds]))`, spec.Counter, spec.Matchers, seconds)
}

func Rate(spec Spec, seconds int64) string {
	return fmt.Sprintf(`(sum(rate(%s{%s}[%ds])) or vector(0))`, spec.Counter, spec.Matchers, seconds)
}

func SumRates(specs []Spec, seconds int64) string {
	if len(specs) == 0 {
		return ""
	}
	parts := make([]string, 0, len(specs))
	for _, spec := range specs {
		parts = append(parts, Rate(spec, seconds))
	}
	return strings.Join(parts, " + ")
}

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
