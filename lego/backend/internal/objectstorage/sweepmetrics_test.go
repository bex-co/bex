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

package objectstorage_test

import (
	"maps"
	"slices"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"

	"github.com/bex-co/bex/lego/backend/internal/objectstorage"
)

// Only the replica that holds the sweep's lock records a pass, and a vector
// creates a child at its first use. A series that first appears with its first
// event is one increase() cannot count (a counter that is born at 1 has no
// earlier sample to be compared with), so the ObjectStorageSweepCutOff alert
// would miss the first cut-off pass of a replica that took the lock over, and the
// staleness alert reads a series that must exist, at 0, before any pass has
// completed. Every replica therefore exports every series, at zero, from its
// start (w21/096).
func TestEveryReplicaExportsEverySweepSeriesAtZero(t *testing.T) {
	reg := prometheus.NewRegistry()
	objectstorage.NewSweepMetrics(reg)
	families, err := reg.Gather()
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]*dto.MetricFamily{}
	for _, f := range families {
		byName[f.GetName()] = f
	}

	// value is a series' sample: a histogram's observation count, a counter's or
	// gauge's value.
	value := func(m *dto.Metric) float64 {
		switch {
		case m.GetHistogram() != nil:
			return float64(m.GetHistogram().GetSampleCount())
		case m.GetCounter() != nil:
			return m.GetCounter().GetValue()
		default:
			return m.GetGauge().GetValue()
		}
	}
	// seriesOf maps a family's label value (or "" for a family with no label) to
	// its sample.
	seriesOf := func(name, label string) map[string]float64 {
		t.Helper()
		f := byName[name]
		if f == nil {
			t.Fatalf("a replica that never swept exports no %s at all", name)
		}
		got := map[string]float64{}
		for _, m := range f.GetMetric() {
			key := ""
			for _, l := range m.GetLabel() {
				if l.GetName() == label {
					key = l.GetValue()
				}
			}
			got[key] = value(m)
		}
		return got
	}

	for _, c := range []struct {
		family, label string
		want          []string
	}{
		{"bex_usage_object_storage_sweep_pass_duration_seconds", "result", []string{"complete", "failures", "cut_off"}},
		{"bex_usage_object_storage_sweep_workspaces_total", "result", []string{"swept", "fresh", "failed", "not_reached"}},
		{"bex_usage_object_storage_sweep_last_complete_timestamp_seconds", "", []string{""}},
		{"bex_usage_object_storage_sweep_workspace_duration_seconds", "", []string{""}},
	} {
		got := seriesOf(c.family, c.label)
		if want := slices.Sorted(slices.Values(c.want)); !slices.Equal(slices.Sorted(maps.Keys(got)), want) {
			t.Errorf("%s has series %v before any pass, want one at 0 for each of %v", c.family, slices.Sorted(maps.Keys(got)), want)
		}
		for k, v := range got {
			if v != 0 {
				t.Errorf("%s{%s=%q} = %v before any pass, want 0", c.family, c.label, k, v)
			}
		}
	}
}
