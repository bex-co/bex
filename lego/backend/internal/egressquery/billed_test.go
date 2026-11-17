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
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/testenv"
)

// Only a source whose exporter is verified to recreate a series from zero
// gets the new-series rule, each guarded by a scrape of its own target; the
// direct meter restores a checkpointed counter and bills Increase unchanged.
func TestBilledCountsNewSeriesFromZeroForEveryVerifiedSource(t *testing.T) {
	specs := App("srv-one", []string{"r@kubernetes"}, true)
	specs = append(specs, Datastore("dpg-one", "postgres", true)[0], Datastore("kv-one", "key_value", true)[0])
	ruled := map[Source]bool{}
	for _, spec := range specs {
		got := Billed(spec, 3600)
		if spec.Source == Direct {
			if spec.NewSeriesFromZero || got != Increase(spec, 3600) {
				t.Errorf("%s bills %q, want Increase", spec.Source, got)
			}
			continue
		}
		ruled[spec.Source] = spec.NewSeriesFromZero
		series := fmt.Sprintf("%s{%s}", spec.Counter, spec.Matchers)
		for _, want := range []string{
			fmt.Sprintf("last_over_time(%s[3600s])", series),
			fmt.Sprintf("unless last_over_time(%s[30s] offset 3600s)", series),
			fmt.Sprintf("and on(instance) (max_over_time(up{job=%q}[30s] offset 3600s) == 1)", spec.Job),
		} {
			if !strings.Contains(got, want) {
				t.Errorf("%s billed query missing %q: %s", spec.Source, want, got)
			}
		}
	}
	if want := map[Source]bool{HTTP: true, WebSocket: true, Postgres: true, KeyValue: true}; !maps.Equal(ruled, want) {
		t.Errorf("sources counting new series from zero = %v, want %v", ruled, want)
	}
}

// TestBilledOnPrometheus evaluates the billing expression in Prometheus's own
// engine via `promtool test rules`, like TestInstanceSecondsQueryOnPrometheus,
// for every source that counts new series from zero. Each group is one
// 15 s-scraped target of that source over two hours; the billed hour is
// (3600 s, 7200 s], evaluated at its end.
func TestBilledOnPrometheus(t *testing.T) {
	promtool, err := exec.LookPath("promtool")
	if err != nil {
		testenv.Skip(t, "promtool not on PATH")
	}
	app := App("srv-one", []string{"r@kubernetes"}, true)
	// Each source's counter labels: base on every series, variant added to a
	// group's default series (a router series is one status code's).
	sources := []struct {
		spec          Spec
		base, variant string
	}{
		{app[0], `router="r@kubernetes",method="GET",protocol="http"`, `code="200"`},
		{app[1], `app_id="srv-one"`, ""},
		{Datastore("dpg-one", "postgres", true)[0], `resource_id="dpg-one",resource_kind="postgres"`, ""},
		{Datastore("kv-one", "key_value", true)[0], `resource_id="kv-one",resource_kind="key_value"`, ""},
	}
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
	// A short public datastore connection or WebSocket session that ended
	// before its series' first scrape: the proxies add a connection's bytes
	// when it closes, so the first sample is already the whole session.
	session := counter(310, steps, 52000, 0)
	for _, src := range sources {
		spec := src.spec
		t.Run(string(spec.Source), func(t *testing.T) {
			billed, increase := Billed(spec, 3600), Increase(spec, 3600)
			diff := fmt.Sprintf("(%s) - (%s)", billed, increase)
			type expect struct {
				expr string
				want string
			}
			groups := []struct {
				name   string
				only   Source // "" for every source
				up     string
				series map[string]string // extra labels ("" for the source's variant) -> values
				checks []expect
			}{
				{"everything counted before the first scrape counts from zero, where increase() bills nothing", "", always("1"),
					map[string]string{"": session},
					[]expect{{billed, "52000"}, {increase, "0"}, {Health(spec, 3600), "1"}}},
				{"a new series counts from zero, through its last sample after it went stale (the m333 probe)", HTTP, always("1"),
					map[string]string{"": probe},
					[]expect{{billed, "10000570"}, {"round(" + increase + ")", "4571429"}, {Health(spec, 3600), "1"}}},
				{"a new series still live at the hour's end", "", always("1"),
					map[string]string{"": counter(300, steps, 500, 10)},
					[]expect{{billed, "2300"}}},
				{"a series present at the start bills exactly what increase() bills", "", always("1"),
					map[string]string{"": counter(0, steps, 0, 1000)},
					[]expect{{billed, "240000"}, {diff, "0"}}},
				{"an old series and a new status code on the same router", HTTP, always("1"),
					map[string]string{`code="200"`: counter(0, steps, 0, 1000), `code="503"`: counter(400, steps, 216, 0)},
					[]expect{{billed, "240216"}}},
				{"a router deleted in the last hour and recreated counts its new life from zero", HTTP, always("1"),
					map[string]string{"": values(func(i int) string {
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
				{"a series missing at the start because its target was not scraped keeps increase()", "",
					values(func(i int) string {
						if i > 225 && i < 246 {
							return ""
						}
						return "1"
					}),
					map[string]string{"": values(func(i int) string {
						if i > 225 && i < 246 {
							return ""
						}
						return fmt.Sprint(50000 + 10*i)
					})},
					[]expect{{diff, "0"}, {"(" + billed + ") < bool 10000", "1"}}},
				{"a series missing at the start because the scrapes failed keeps increase()", "",
					values(func(i int) string {
						if i >= 238 && i <= 240 {
							return "0"
						}
						return "1"
					}),
					map[string]string{"": values(func(i int) string {
						if i >= 238 && i <= 240 {
							return ""
						}
						return fmt.Sprint(50000 + 10*i)
					})},
					[]expect{{diff, "0"}, {"(" + billed + ") < bool 10000", "1"}}},
				{"a new series that resets fails the hour's health gate", "", always("1"),
					map[string]string{"": values(func(i int) string {
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
				{"the month-to-date read sums the billed hours", "", always("1"),
					map[string]string{"": session},
					[]expect{{CompletedHoursIncrease(spec, 2), "52000"}}},
			}
			// onTarget adds the scrape target's labels, and a counter's base
			// labels, to a selector made only of equality matchers, so it can
			// serve as a promtool input series.
			onTarget := func(selector string) string {
				name, labels, _ := strings.Cut(strings.TrimSuffix(selector, "}"), "{")
				parts := []string{`instance="10.0.0.1:9100"`, `pod="exporter-a"`}
				if !strings.Contains(labels, "job=") {
					parts = append(parts, fmt.Sprintf("job=%q", spec.Job))
				}
				if name == spec.Counter {
					parts = append(parts, src.base)
				}
				if labels != "" {
					parts = append(parts, labels)
				}
				return name + "{" + strings.Join(parts, ",") + "}"
			}
			var b strings.Builder
			b.WriteString("rule_files: []\nevaluation_interval: 15s\ntests:\n")
			for _, g := range groups {
				if g.only != "" && g.only != spec.Source {
					continue
				}
				fmt.Fprintf(&b, "  # %s\n  - interval: 15s\n    input_series:\n", g.name)
				fmt.Fprintf(&b, "      - series: '%s'\n        values: '%s'\n", onTarget("up"), g.up)
				if spec.HealthMetric != "" {
					fmt.Fprintf(&b, "      - series: '%s'\n        values: '%s'\n", onTarget(spec.HealthMetric), always("1"))
				}
				fmt.Fprintf(&b, "      - series: '%s'\n        values: '%s'\n", onTarget(spec.ProcessMetric), always("0"))
				for labels, v := range g.series {
					if labels == "" {
						labels = src.variant
					}
					fmt.Fprintf(&b, "      - series: '%s'\n        values: '%s'\n", onTarget(spec.Counter+"{"+labels+"}"), v)
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
		})
	}
}
