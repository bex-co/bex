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

func TestExactDeltaDoesNotExtrapolate(t *testing.T) {
	q := ExactDelta(Spec{Counter: "c_total", Matchers: `router="r"`}, 600)
	if strings.Contains(q, "increase(") || strings.Contains(q, "rate(") {
		t.Fatalf("query %q extrapolates", q)
	}
	for _, want := range []string{`c_total{router="r"} offset 600s`, "clamp_min(", "or vector(0)"} {
		if !strings.Contains(q, want) {
			t.Errorf("query %q missing %q", q, want)
		}
	}
}

// w4/477: a flat counter must read the same at every later instant. Promtool
// evaluates the real engine, which the canned-response metrics tests cannot.
func TestExactDeltaOnPrometheus(t *testing.T) {
	promtool, err := exec.LookPath("promtool")
	if err != nil {
		testenv.Skip(t, "promtool not on PATH")
	}
	spec := Spec{Counter: "c_total", Matchers: `router="r"`}
	// 15 s steps; the "hour" starts at step 40 (10m). young appears at step 50
	// with 100 bytes and stays flat; old climbs 1000→1500 at step 60 then stays
	// flat; reset restarts from 0 mid-hour.
	flat := func(first, n int, v string) string {
		out := make([]string, 0, n)
		for i := 0; i < n; i++ {
			if i < first {
				out = append(out, "_")
			} else {
				out = append(out, v)
			}
		}
		return strings.Join(out, " ")
	}
	old := flat(0, 60, "1000") + " " + flat(0, 60, "1500")
	cases := []struct {
		name   string
		values string
		at     int // eval step
		want   float64
	}{
		{"young-early", flat(50, 140, "100"), 80, 100},
		{"young-late", flat(50, 140, "100"), 130, 100},
		{"old-early", old, 80, 500},
		{"old-late", old, 130, 500},
	}
	var b strings.Builder
	b.WriteString("rule_files: []\nevaluation_interval: 15s\ntests:\n")
	for _, c := range cases {
		hourStart := 40
		fmt.Fprintf(&b, "  - interval: 15s\n    input_series:\n      - series: 'c_total{router=\"r\"}'\n        values: '%s'\n    promql_expr_test:\n", c.values)
		fmt.Fprintf(&b, "      - expr: '%s'\n        eval_time: %ds\n        exp_samples:\n          - labels: '{}'\n            value: %v\n",
			ExactDelta(spec, int64((c.at-hourStart)*15)), c.at*15, c.want)
	}
	path := filepath.Join(t.TempDir(), "exact_delta.yml")
	if err := os.WriteFile(path, []byte(b.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command(promtool, "test", "rules", path).CombinedOutput(); err != nil {
		t.Fatalf("promtool: %v\n%s", err, out)
	}
}
