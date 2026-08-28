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

package metrics

// empty_case_test.go is w6/m110/t005's parity guard: REST, GraphQL and MCP all
// funnel through MetricsWithQuantiles, so a populated query is identical across
// them by construction — but the EMPTY case is assembled separately on each
// surface, and a nil slice marshals to `null` while an allocated one marshals
// to `[]`. The dashboard's "No data in range" state reads a list, so the two
// must not disagree.

import (
	"encoding/json"
	"testing"
)

// TestEmptySeriesShapeMatchesAcrossSurfaces pins REST and MCP to the same empty
// shape. Regression for w6/m110/t005: MCP returned {"series":null} where REST
// returned [].
func TestEmptySeriesShapeMatchesAcrossSurfaces(t *testing.T) {
	var none []MetricSeries // what every surface accumulates when nothing matches

	restJSON, err := json.Marshal(toRenderMetrics(none))
	if err != nil {
		t.Fatalf("marshal REST: %v", err)
	}
	if got := string(restJSON); got != "[]" {
		t.Errorf("REST empty series = %s, want []", got)
	}

	mcpJSON, err := json.Marshal(getMetricsResult{Series: metricSeriesOrEmpty(none)})
	if err != nil {
		t.Fatalf("marshal MCP: %v", err)
	}
	if got := string(mcpJSON); got != `{"series":[]}` {
		t.Errorf(`MCP empty series = %s, want {"series":[]} — a nil slice marshals to null and diverges from REST (w6/m110/t005)`, got)
	}
}

// TestNonEmptySeriesRoundTripsUnchanged is the control: the nil-coalescing must
// not disturb a populated result on either surface.
func TestNonEmptySeriesRoundTripsUnchanged(t *testing.T) {
	series := []MetricSeries{{Unit: "bytes", Points: []MetricPoint{}}}

	if got := len(toRenderMetrics(series)); got != 1 {
		t.Errorf("REST populated series = %d entries, want 1", got)
	}
	if got := len(metricSeriesOrEmpty(series)); got != 1 {
		t.Errorf("MCP populated series = %d entries, want 1", got)
	}
}
