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

package egressquery

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/testenv"
)

// Only a source whose exporter is verified to recreate a series from zero
// gets the new-series rule; every other source bills Increase unchanged.
func TestBilledCountsNewSeriesOnlyForTraefikRouters(t *testing.T) {
	specs := App("srv-one", []string{"r@kubernetes"}, true)
	specs = append(specs, Datastore("dpg-one", "postgres", true)[0], Datastore("kv-one", "key_value", true)[0])
	for _, spec := range specs {
		got := Billed(spec, 3600)
		if spec.Source != HTTP {
			if spec.NewSeriesFromZero || got != Increase(spec, 3600) {
				t.Errorf("%s bills %q, want Increase", spec.Source, got)
			}
			continue
		}
		for _, want := range []string{
			`last_over_time(traefik_router_responses_bytes_total{router=~"^(r@kubernetes)$"}[3600s])`,
			`unless last_over_time(traefik_router_responses_bytes_total{router=~"^(r@kubernetes)$"}[30s] offset 3600s)`,
			`and on(instance) (max_over_time(up{job="traefik"}[30s] offset 3600s) == 1)`,
		} {
			if !strings.Contains(got, want) {
				t.Errorf("HTTP billed query missing %q: %s", want, got)
			}
		}
	}
}

// TestBilledOnPrometheus evaluates the billing expression in Prometheus's own
// engine via `promtool test rules`, like TestInstanceSecondsQueryOnPrometheus.
// Each group is one 15 s-scraped Traefik target over two hours; the billed
// hour is (3600 s, 7200 s], evaluated at its end.
func TestBilledOnPrometheus(t *testing.T) {
	promtool, err := exec.LookPath("promtool")
	if err != nil {
		testenv.Skip(t, "promtool not on PATH")
	}
	spec := App("srv-one", []string{"r@kubernetes"}, false)[0]
	const steps = 481 // 0..7200 s
	// values renders one sample per step; at returns "" for no sample.
	values := func(at func(step int) string) string {
		out := make([]string, steps)
		for i := range out {
			if out[i] = at(i); out[i] == "" {
				out[i] = "_"
			}
		}
		return strings.Join(out, " ")
	}
	always := func(v string) string { return values(func(int) string { return v }) }
	// counter is a series absent before `from`, at base+per*(step-from) from
	// then on, until `to` (exclusive), where it goes stale.
	counter := func(from, to, base, per int) string {
		return values(func(i int) string {
			switch {
			case i < from || i > to:
				return ""
			case i == to:
				return "stale"
			default:
				return fmt.Sprint(base + per*(i-from))
			}
		})
	}
	// The m333 probe (w6/123): five 2 MB downloads, three of them served before
	// the series' first scrape at 6,000,570, the last sample 105 s later at
	// 10,000,570, then the service is deleted and the series goes stale.
	probe := values(func(i int) string {
		const first = 310
		switch {
		case i < first || i > first+8:
			return ""
		case i == first+8:
			return "stale"
		case i < first+2:
			return "6000570"
		case i < first+4:
			return "8000570"
		default:
			return "10000570"
		}
	})
	billed, increase := Billed(spec, 3600), Increase(spec, 3600)
	diff := fmt.Sprintf("(%s) - (%s)", billed, increase)
	type expect struct {
		expr string
		want string
	}
	groups := []struct {
		name   string
		up     string
		series map[string]string // code -> values
		checks []expect
	}{
		{"a new series counts from zero, through its last sample after it went stale (the m333 probe)", always("1"),
			map[string]string{"200": probe},
			[]expect{{billed, "10000570"}, {"round(" + increase + ")", "4571429"}, {Health(spec, 3600), "1"}}},
		{"a new series still live at the hour's end", always("1"),
			map[string]string{"200": counter(300, steps, 500, 10)},
			[]expect{{billed, "2300"}}},
		{"a series present at the start bills exactly what increase() bills", always("1"),
			map[string]string{"200": counter(0, steps, 0, 1000)},
			[]expect{{billed, "240000"}, {diff, "0"}}},
		{"an old series and a new status code on the same router", always("1"),
			map[string]string{"200": counter(0, steps, 0, 1000), "503": counter(400, steps, 216, 0)},
			[]expect{{billed, "240216"}}},
		{"a router deleted in the last hour and recreated counts its new life from zero", always("1"),
			map[string]string{"200": values(func(i int) string {
				switch {
				case i < 200:
					return fmt.Sprint(90000 + 100*i)
				case i == 200:
					return "stale"
				case i < 300:
					return ""
				default:
					return fmt.Sprint(700 + 5*(i-300))
				}
			})},
			[]expect{{billed, "1600"}, {Health(spec, 3600), "1"}}},
		{"a series missing at the start because its target was not scraped keeps increase()",
			values(func(i int) string {
				if i > 225 && i < 246 {
					return ""
				}
				return "1"
			}),
			map[string]string{"200": values(func(i int) string {
				if i > 225 && i < 246 {
					return ""
				}
				return fmt.Sprint(50000 + 10*i)
			})},
			[]expect{{diff, "0"}, {"(" + billed + ") < bool 10000", "1"}}},
		{"a series missing at the start because the scrapes failed keeps increase()",
			values(func(i int) string {
				if i >= 238 && i <= 240 {
					return "0"
				}
				return "1"
			}),
			map[string]string{"200": values(func(i int) string {
				if i >= 238 && i <= 240 {
					return ""
				}
				return fmt.Sprint(50000 + 10*i)
			})},
			[]expect{{diff, "0"}, {"(" + billed + ") < bool 10000", "1"}}},
		{"a new series that resets fails the hour's health gate", always("1"),
			map[string]string{"200": values(func(i int) string {
				switch {
				case i < 300:
					return ""
				case i < 400:
					return fmt.Sprint(1000 * (i - 299))
				default:
					return fmt.Sprint(10 * (i - 399))
				}
			})},
			[]expect{{Health(spec, 3600), "0"}}},
		{"the month-to-date read sums the billed hours", always("1"),
			map[string]string{"200": probe},
			[]expect{{CompletedHoursIncrease(spec, 2), "10000570"}}},
	}
	var b strings.Builder
	b.WriteString("rule_files: []\nevaluation_interval: 15s\ntests:\n")
	const target = `instance="10.0.0.1:9100",job="traefik",pod="traefik-a"`
	for _, g := range groups {
		fmt.Fprintf(&b, "  # %s\n  - interval: 15s\n    input_series:\n", g.name)
		fmt.Fprintf(&b, "      - series: 'up{%s}'\n        values: '%s'\n", target, g.up)
		fmt.Fprintf(&b, "      - series: 'process_start_time_seconds{%s}'\n        values: '%s'\n", target, always("0"))
		for code, v := range g.series {
			fmt.Fprintf(&b, "      - series: 'traefik_router_responses_bytes_total{router=\"r@kubernetes\",code=%q,method=\"GET\",protocol=\"http\",%s}'\n        values: '%s'\n", code, target, v)
		}
		b.WriteString("    promql_expr_test:\n")
		for _, c := range g.checks {
			fmt.Fprintf(&b, "      - expr: '%s'\n        eval_time: 7200s\n        exp_samples:\n          - labels: '{}'\n            value: %s\n", c.expr, c.want)
		}
	}
	path := filepath.Join(t.TempDir(), "billed.yml")
	if err := os.WriteFile(path, []byte(b.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command(promtool, "test", "rules", path).CombinedOutput(); err != nil {
		t.Fatalf("promtool: %v\n%s", err, out)
	}
}
