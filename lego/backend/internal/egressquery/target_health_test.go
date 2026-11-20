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
	"regexp"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/testenv"
)

// rollSteps is two hours of 15 s scrapes, 0..7200 s. The billed hour is
// (3600 s, 7200 s], steps 241..480, evaluated at its end.
const rollSteps = 481

// rollPod is one SNI proxy pod as Prometheus scrapes it: one target, one
// series per metric, every exported metric present only in a successful
// scrape.
type rollPod struct {
	name string
	// scrape is whether the target was scraped at a step, and whether that
	// scrape succeeded.
	scrape func(step int) (scraped, ok bool)
	// start is the process start time a successful scrape reads.
	start func(step int) int
	// bytes is the store's counter, negative while the process holds no
	// series for it.
	bytes func(step int) int
	// drained is the drained gauge, 0 or 1.
	drained func(step int) int
}

// steadyPod runs one process from before the first step through the last,
// scraped successfully at every step, its counter rising by per a step.
func steadyPod(name string, per int) rollPod {
	return rollPod{
		name:    name,
		scrape:  func(int) (bool, bool) { return true, true },
		start:   func(int) int { return 0 },
		bytes:   func(i int) int { return per * i },
		drained: func(int) int { return 0 },
	}
}

// series renders a pod's input series for promtool: up while the target is
// scraped, each exported metric at every successful scrape, and a stale
// marker where a series stops, as Prometheus writes one when a scrape fails or
// the target goes.
func (p rollPod) series(ip string, spec Spec) map[string]string {
	target := fmt.Sprintf(`job=%q,instance="%s:9093",pod=%q`, spec.Job, ip, p.name)
	selector := func(metric, extra string) string {
		name, labels, _ := strings.Cut(strings.TrimSuffix(metric, "}"), "{")
		parts := []string{target}
		for _, l := range []string{labels, extra} {
			if l != "" {
				parts = append(parts, l)
			}
		}
		return name + "{" + strings.Join(parts, ",") + "}"
	}
	render := func(at func(step int) (string, bool)) string {
		out := make([]string, rollSteps)
		was := false
		for i := range out {
			v, has := at(i)
			switch {
			case has:
				out[i] = v
			case was:
				out[i] = "stale"
			default:
				out[i] = "_"
			}
			was = has
		}
		return strings.Join(out, " ")
	}
	exported := func(value func(int) int, skipNegative bool) string {
		return render(func(i int) (string, bool) {
			if _, ok := p.scrape(i); !ok {
				return "", false
			}
			v := value(i)
			if skipNegative && v < 0 {
				return "", false
			}
			return fmt.Sprint(v), true
		})
	}
	return map[string]string{
		selector("up", ""): render(func(i int) (string, bool) {
			scraped, ok := p.scrape(i)
			if !scraped {
				return "", false
			}
			if ok {
				return "1", true
			}
			return "0", true
		}),
		selector(spec.HealthMetric, ""):            exported(func(int) int { return 1 }, false),
		selector(spec.ProcessMetric, ""):           exported(p.start, false),
		selector(spec.DrainedMetric, ""):           exported(p.drained, false),
		selector(spec.Counter+"{}", spec.Matchers): exported(p.bytes, true),
	}
}

// TestAnSNIRollOnPrometheus evaluates the Postgres and Key Value sources'
// per-pod health and billed bytes in Prometheus's own engine (w6/155), as
// TestBilledOnPrometheus does for one target: a pod a roll starts inside the
// hour counts from zero, an old pod that drained bills until its exit, and
// neither fails the hour for the others, while a restarted container, a pod
// that vanished without a drained scrape and a steady pod's failed scrape
// still do.
func TestAnSNIRollOnPrometheus(t *testing.T) {
	promtool, err := exec.LookPath("promtool")
	if err != nil {
		testenv.Skip(t, "promtool not on PATH")
	}
	// live is a pod scraped from step `from` to step `to`, both included, with
	// the failed scrapes `failed` names; drained marks the scrapes from
	// drainedFrom on.
	live := func(from, to int, failed func(int) bool) func(int) (bool, bool) {
		return func(i int) (bool, bool) {
			if i < from || i > to {
				return false, false
			}
			return true, !failed(i)
		}
	}
	never := func(int) bool { return false }
	drainedFrom := func(step int) func(int) int {
		return func(i int) int {
			if i >= step {
				return 1
			}
			return 0
		}
	}
	// oldDrained is a pod that ran from step 0, stopped counting at step 310
	// with 310,000 bytes, and read drained at its last successful scrape, step
	// 312, after which Prometheus scraped the gone target once more.
	oldDrained := rollPod{
		name:    "px-old",
		scrape:  live(0, 313, func(i int) bool { return i == 313 }),
		start:   func(int) int { return 0 },
		bytes:   func(i int) int { return 1000 * min(i, 310) },
		drained: drainedFrom(312),
	}
	// fresh is a pod a roll started at 4,490 s, first scraped at step 300. Its
	// store series appears at step 302 already at 700 bytes, counted before
	// that scrape, and ends the hour at 2,480.
	fresh := rollPod{
		name:   "px-new",
		scrape: live(300, rollSteps-1, never),
		start:  func(int) int { return 4490 },
		bytes: func(i int) int {
			if i < 302 {
				return -1
			}
			return 700 + 10*(i-302)
		},
		drained: func(int) int { return 0 },
	}
	vanished := oldDrained
	vanished.drained = func(int) int { return 0 }
	// restarted's container restarts at 5,020 s: four failed scrapes, then a
	// new process whose store series has not appeared by the hour's end, so
	// the counter shows no reset.
	restarted := rollPod{
		name:   "px-a",
		scrape: live(0, rollSteps-1, func(i int) bool { return i >= 331 && i <= 334 }),
		start: func(i int) int {
			if i > 334 {
				return 5020
			}
			return 0
		},
		bytes: func(i int) int {
			if i > 334 {
				return -1
			}
			return 500 * i
		},
		drained: func(int) int { return 0 },
	}
	// freshRestarted is fresh whose container restarts at 5,520 s, both of its
	// processes started inside the window.
	freshRestarted := fresh
	freshRestarted.scrape = live(300, rollSteps-1, func(i int) bool { return i >= 366 && i <= 368 })
	freshRestarted.start = func(i int) int {
		if i > 368 {
			return 5520
		}
		return 4490
	}
	freshRestarted.bytes = func(i int) int {
		if i < 302 || i > 368 {
			return -1
		}
		return 700 + 10*(i-302)
	}
	// restartedAtTheEdge's container restarts just before the window: its
	// scrapes fail from 3,555 s and its new process, started at 3,640 s, is
	// the only one the window reads.
	restartedAtTheEdge := rollPod{
		name:   "px-a",
		scrape: live(0, rollSteps-1, func(i int) bool { return i >= 237 && i <= 242 }),
		start: func(i int) int {
			if i > 242 {
				return 3640
			}
			return 0
		},
		bytes: func(i int) int {
			if i > 242 {
				return 30 * (i - 242)
			}
			return 500 * i
		},
		drained: func(int) int { return 0 },
	}
	// unscraped runs all along, but Prometheus did not scrape it from 3,555 s
	// to 3,630 s, around the window's start.
	unscraped := steadyPod("px-old", 1000)
	unscraped.scrape = func(i int) (bool, bool) { return i < 237 || i > 242, true }
	freshAfterTheGap := fresh
	freshAfterTheGap.scrape = live(247, rollSteps-1, never)
	freshAfterTheGap.start = func(int) int { return 3690 }
	// late is an old pod not scraped for the window's first quarter, which
	// drained at step 330, then was scraped once more after its exit: 31
	// samples where its life in the window asks 72.
	late := oldDrained
	late.scrape = func(i int) (bool, bool) {
		if i > 240 && i <= 300 || i > 331 {
			return false, false
		}
		return true, i != 331
	}
	late.drained = drainedFrom(330)
	// short started at 4,000 s and drained at step 400, inside the hour, with
	// 18,000 bytes, the first 5,000 counted before its series' first scrape.
	short := rollPod{
		name:   "px-short",
		scrape: live(267, 400, never),
		start:  func(int) int { return 4000 },
		bytes: func(i int) int {
			if i < 268 {
				return -1
			}
			return 5000 + 100*min(i-268, 130)
		},
		drained: drainedFrom(400),
	}
	// stopped ran from before the window with 230 samples in it, then its
	// scrapes stopped at step 470 with no drained scrape: the Postgres hour
	// ending 2026-11-20T10:00Z, which the fleet-wide gate passed.
	stopped := steadyPod("px-a", 500)
	stopped.scrape = live(0, 470, never)
	blip := steadyPod("px-a", 500)
	blip.scrape = live(0, rollSteps-1, func(i int) bool { return i == 400 })

	for _, spec := range []Spec{Datastore("dpg-one", "postgres", true)[0], Datastore("kv-one", "key_value", true)[0]} {
		t.Run(string(spec.Source), func(t *testing.T) {
			counter := func(pod string) string {
				return fmt.Sprintf(`sum(increase(%s{%s,pod=%q}[3600s]))`, spec.Counter, spec.Matchers, pod)
			}
			health, billed := Health(spec, 3600), Billed(spec, 3600)
			type check struct{ expr, want string }
			groups := []struct {
				name   string
				pods   []rollPod
				checks []check
			}{
				{"a clean roll bills the old pod until its drained exit and the new pod from zero",
					[]rollPod{oldDrained, fresh, steadyPod("px-other", 0)},
					[]check{{health, "1"}, {fmt.Sprintf("(%s) - %s", billed, counter("px-old")), "2480"}}},
				{"a pod that started and drained inside the hour bills from zero",
					[]rollPod{steadyPod("px-old", 1000), short},
					[]check{{health, "1"}, {fmt.Sprintf("(%s) - %s", billed, counter("px-old")), "18000"}}},
				{"a steady fleet passes, as before", []rollPod{steadyPod("px-a", 500), steadyPod("px-b", 0)},
					[]check{{health, "1"}}},
				{"an old pod that vanished without a drained scrape fails the hour", []rollPod{vanished, fresh},
					[]check{{health, "0"}}},
				{"a restarted container fails the hour though its counter shows no reset",
					[]rollPod{restarted, steadyPod("px-b", 1000)},
					[]check{{health, "0"}, {fmt.Sprintf("sum(resets(%s{%s}[3600s]))", spec.Counter, spec.Matchers), "0"}}},
				{"a new pod whose container restarted in the hour fails it", []rollPod{oldDrained, freshRestarted},
					[]check{{health, "0"}}},
				{"a container restarted just before the hour is no new pod", []rollPod{restartedAtTheEdge, steadyPod("px-b", 1000)},
					[]check{{health, "0"}}},
				{"a new pod is not new when its job was not scraped just before the hour",
					[]rollPod{unscraped, freshAfterTheGap}, []check{{health, "0"}}},
				{"an old pod that drained is still held to 80% of its samples", []rollPod{late, steadyPod("px-b", 0)},
					[]check{{health, "0"}}},
				{"a steady pod whose scrapes stopped before the hour's end fails it", []rollPod{stopped, steadyPod("px-b", 0)},
					[]check{{health, "0"}}},
				{"a steady pod's one failed scrape still fails the hour, as before", []rollPod{blip, steadyPod("px-b", 0)},
					[]check{{health, "0"}}},
			}
			var b strings.Builder
			b.WriteString("rule_files: []\nevaluation_interval: 15s\ntests:\n")
			for _, g := range groups {
				fmt.Fprintf(&b, "  # %s\n  - interval: 15s\n    input_series:\n", g.name)
				for i, pod := range g.pods {
					for series, values := range pod.series(fmt.Sprintf("10.0.0.%d", i+1), spec) {
						fmt.Fprintf(&b, "      - series: '%s'\n        values: '%s'\n", series, values)
					}
				}
				b.WriteString("    promql_expr_test:\n")
				for _, c := range g.checks {
					fmt.Fprintf(&b, "      - expr: '%s'\n        eval_time: 7200s\n        exp_samples:\n          - labels: '{}'\n            value: %s\n", c.expr, c.want)
				}
			}
			path := filepath.Join(t.TempDir(), "roll.yml")
			if err := os.WriteFile(path, []byte(b.String()), 0o600); err != nil {
				t.Fatal(err)
			}
			if out, err := exec.Command(promtool, "test", "rules", path).CombinedOutput(); err != nil {
				t.Fatalf("promtool: %v\n%s", err, out)
			}
		})
	}
}

// TestBillingReadsOnlyWhatTheScrapeJobsKeep: each datastore source's job
// keeps only the families its keep-list names (deploy/gitops/base/
// prometheus.yaml). A family the billing reads and the list drops is never
// stored, and its terms fail every hour: a drained gauge it dropped would fail
// every hour a pod exits in (w6/155).
func TestBillingReadsOnlyWhatTheScrapeJobsKeep(t *testing.T) {
	raw, err := os.ReadFile("../../../../deploy/gitops/base/prometheus.yaml")
	if err != nil {
		t.Fatal(err)
	}
	for _, spec := range []Spec{Datastore("dpg-one", "postgres", true)[0], Datastore("kv-one", "key_value", true)[0]} {
		job := regexp.MustCompile(`(?s)- job_name: ` + regexp.QuoteMeta(spec.Job) + `\n(.*?)\n\s*(?:#[^\n]*\n\s*)*- job_name:`).FindSubmatch(raw)
		if job == nil {
			t.Fatalf("there is no %s scrape job", spec.Job)
		}
		found := regexp.MustCompile(`(?m)^\s*regex:\s*(bex_\S+)\s*$`).FindSubmatch(job[1])
		if found == nil {
			t.Fatalf("the %s job has no metric keep-list: %s", spec.Job, job[1])
		}
		keep := regexp.MustCompile(`^(?:` + string(found[1]) + `)$`) // Prometheus anchors relabel regexes
		for _, metric := range []string{spec.Counter, spec.HealthMetric, spec.ProcessMetric, spec.DrainedMetric} {
			name, _, _ := strings.Cut(metric, "{")
			if !keep.MatchString(name) {
				t.Errorf("%s reads %s, which the %s job's keep-list %q drops", spec.Source, name, spec.Job, found[1])
			}
		}
	}
}
