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
	"strings"
	"testing"
)

// TestInstanceSecondsQueryMeasuresRunningTime pins w4/m173's shape: running
// time per POD (containers collapsed), at the 15 s step, never the old
// count(avg_over_time)×window that billed every container series seen in the
// hour a full hour.
func TestInstanceSecondsQueryMeasuresRunningTime(t *testing.T) {
	q := instanceSecondsQuery(`namespace="tea-a",pod=~"web-.+",container!=""`, 3600)
	for _, want := range []string{"max by (pod)", "count_over_time(", "[3599s:15s]", `container!=""`} {
		if !strings.Contains(q, want) {
			t.Errorf("query %q missing %q", q, want)
		}
	}
	if strings.Contains(q, "avg_over_time") || strings.HasPrefix(q, "count(") {
		t.Errorf("query %q regressed to the series-count shape", q)
	}
}

// TestInstanceSecondsQueryOnPrometheus evaluates the production expression in
// Prometheus' own engine via `promtool test rules` (skipped when promtool is
// not on PATH; verified against v2.54.1, the deployed chart's line). The
// series are 15 s scrapes over one hour, ending in staleness markers the way
// cAdvisor series end when a pod is deleted.
func TestInstanceSecondsQueryOnPrometheus(t *testing.T) {
	promtool, err := exec.LookPath("promtool")
	if err != nil {
		t.Skip("promtool not on PATH")
	}
	presence := func(from, to int, endStale bool) string {
		out := make([]string, 0, 241)
		for i := 0; i <= 240; i++ {
			switch {
			case i >= from && i < to:
				out = append(out, "1")
			case endStale && i == to:
				out = append(out, "stale")
			default:
				out = append(out, "_")
			}
		}
		return strings.Join(out, " ")
	}
	cases := []struct {
		namespace string
		series    map[string]string // pod/container -> values
		want      int
	}{
		// 5 minutes of life: 285, within one scrape (its t=0 sample belongs to the prior hour).
		{"short", map[string]string{"a-1/app": presence(0, 20, true)}, 285},
		// One pod, two containers (Valkey + exporter): one instance, not two.
		{"sidecar", map[string]string{"kv-0/valkey": presence(0, 241, false), "kv-0/exporter": presence(0, 241, false)}, 3600},
		// Two replicas all hour.
		{"replicas", map[string]string{"web-1/app": presence(0, 241, false), "web-2/app": presence(0, 241, false)}, 7200},
		// Rollout: old pod 0–40 min, new pod 38–60 min — only the 2-minute overlap is extra.
		{"rollout", map[string]string{"web-old/app": presence(0, 160, true), "web-new/app": presence(152, 241, false)}, 3720},
	}
	var b strings.Builder
	b.WriteString("rule_files: []\nevaluation_interval: 15s\ntests:\n  - interval: 15s\n    input_series:\n")
	for _, c := range cases {
		for key, values := range c.series {
			pod, container, _ := strings.Cut(key, "/")
			fmt.Fprintf(&b, "      - series: 'container_memory_working_set_bytes{namespace=%q,pod=%q,container=%q}'\n        values: '%s'\n",
				c.namespace, pod, container, values)
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
