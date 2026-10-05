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

package usage

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/testenv"
)

// TestInstanceSecondsQueryMeasuresRunningTime pins w4/m173's shape: running
// time per POD (containers collapsed), at the 15 s step, never the old
// count(avg_over_time)×window that billed every container series seen in the
// hour a full hour.
func TestInstanceSecondsQueryMeasuresRunningTime(t *testing.T) {
	q := instanceSecondsQuery(`namespace="tea-a",pod=~"web-.+",container!=""`, 3600)
	for _, want := range []string{"max by (pod)", "count_over_time(", "[3599s:15s]", "}[44s]", `container!=""`} {
		if !strings.Contains(q, want) {
			t.Errorf("query %q missing %q", q, want)
		}
	}
	if strings.Contains(q, "avg_over_time") || strings.HasPrefix(q, "count(") {
		t.Errorf("query %q regressed to the series-count shape", q)
	}
}

// TestInstanceSecondsQueryOnPrometheus evaluates the production expression in
// Prometheus' own engine via `promtool test rules` (skipped when promtool is not
// on PATH). Series end the way a deleted pod's cAdvisor series do: with no
// staleness marker, because cAdvisor samples carry their own timestamps
// (w4/m110), so only the query's liveness window stops a terminated pod from
// counting (w5/m111). Verified on v2.54.1 (closed ranges, the deployed line)
// and v3.14.0 (left-open ranges): the same values on both.
func TestInstanceSecondsQueryOnPrometheus(t *testing.T) {
	promtool, err := exec.LookPath("promtool")
	if err != nil {
		testenv.Skip(t, "promtool not on PATH")
	}
	// presence is a 15 s series over the hour: a sample at steps [from, to)
	// except the missing ones, then nothing — no staleness marker.
	presence := func(from, to int, missing ...int) string {
		out := make([]string, 0, 241)
		for i := 0; i <= 240; i++ {
			if i >= from && i < to && !slices.Contains(missing, i) {
				out = append(out, "1")
			} else {
				out = append(out, "_")
			}
		}
		return strings.Join(out, " ")
	}
	cases := []struct {
		namespace string
		series    map[string]string // pod/container[#id] -> values
		want      int
	}{
		// 5 minutes of life: its t=0 sample belongs to the prior hour, then 19
		// steps plus the two its last sample stays live for (44 s).
		{"short", map[string]string{"a-1/app": presence(0, 20)}, 315},
		// One pod, two containers (Valkey + exporter): one instance, not two.
		{"sidecar", map[string]string{"kv-0/valkey": presence(0, 241), "kv-0/exporter": presence(0, 241)}, 3600},
		// Two replicas all hour.
		{"replicas", map[string]string{"web-1/app": presence(0, 241), "web-2/app": presence(0, 241)}, 7200},
		// Rollout: old pod 0–40 min, new pod 38–60 min — the 2-minute overlap
		// and the old pod's two live steps are extra, not the 5-minute lookback.
		{"rollout", map[string]string{"web-old/app": presence(0, 160), "web-new/app": presence(152, 241)}, 3750},
		// Gaps of up to 44 s (cAdvisor's housekeeping spacing, missed scrapes)
		// lose no running time; a 60 s gap loses the one step nothing covers.
		{"missed", map[string]string{"web-1/app": presence(0, 241, 100)}, 3600},
		{"missed2", map[string]string{"web-1/app": presence(0, 241, 100, 101)}, 3600},
		{"missed3", map[string]string{"web-1/app": presence(0, 241, 100, 101, 102)}, 3585},
		// A restarted container is a new series in the same pod, briefly
		// overlapping the old one: the pod keeps running, once.
		{"restart", map[string]string{"web-1/app#first": presence(0, 125), "web-1/app#second": presence(121, 241)}, 3600},
	}
	var b strings.Builder
	b.WriteString("rule_files: []\nevaluation_interval: 15s\ntests:\n  - interval: 15s\n    input_series:\n")
	for _, c := range cases {
		for key, values := range c.series {
			pod, rest, _ := strings.Cut(key, "/")
			container, id, _ := strings.Cut(rest, "#")
			fmt.Fprintf(&b, "      - series: 'container_memory_working_set_bytes{namespace=%q,pod=%q,container=%q,id=%q}'\n        values: '%s'\n",
				c.namespace, pod, container, id, values)
		}
	}
	b.WriteString("    promql_expr_test:\n")
	for _, c := range cases {
		expr := fmt.Sprintf("%s * %d", instanceSecondsQuery(fmt.Sprintf(`namespace=%q,container!=""`, c.namespace), 3600), instanceSampleStepSeconds)
		fmt.Fprintf(&b, "      - expr: '%s'\n        eval_time: 1h\n        exp_samples:\n          - labels: '{}'\n            value: %d\n", expr, c.want)
	}
	path := filepath.Join(t.TempDir(), "instance_seconds.yml")
	if err := os.WriteFile(path, []byte(b.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command(promtool, "test", "rules", path).CombinedOutput(); err != nil {
		t.Fatalf("promtool: %v\n%s", err, out)
	}
}
