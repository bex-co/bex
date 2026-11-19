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

package logs

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// Every page read is timed under its query's shape, and a page a scan budget
// cut is told apart from a whole one and from a failure (w5/m503).
func TestEachPageReadIsTimedByItsShapeAndResult(t *testing.T) {
	reg := prometheus.NewRegistry()
	svc := newService(nil, sampleApp("web"))
	svc.Metrics = NewMetrics(reg)
	var answer func() ([]LogEntry, error)
	svc.History = func(context.Context, string, LogQuery) ([]LogEntry, error) { return answer() }
	read := func(q LogQuery) {
		t.Helper()
		_, _ = svc.queryLogPage(context.Background(), []string{"web"}, q)
	}

	answer = func() ([]LogEntry, error) { return []LogEntry{lineAt(scanEnd.Add(-time.Minute), "one")}, nil }
	read(LogQuery{Since: scanEnd.Add(-30 * time.Minute), End: scanEnd})
	answer = func() ([]LogEntry, error) { return nil, &ScanIncompleteError{ScannedTo: scanEnd.Add(-time.Hour)} }
	read(LogQuery{Search: []string{"x"}, Types: []string{LogTypeRequest}, Since: scanEnd.Add(-48 * time.Hour), End: scanEnd, Direction: DirectionForward})
	answer = func() ([]LogEntry, error) { return nil, errors.New("loki: status 502") }
	read(LogQuery{Path: []string{"/api"}, Types: []string{LogTypeApp, LogTypeRequest}, Since: scanEnd.Add(-20 * 24 * time.Hour), End: scanEnd})

	for _, want := range [][]string{
		{"1h", "default", "none", DirectionBackward, "ok"},
		{"7d", LogTypeRequest, "text", DirectionForward, "partial"},
		{"30d", "mixed", "line", DirectionBackward, "error"},
	} {
		if n := histogramCount(t, reg, want); n != 1 {
			t.Errorf("pages %v observed %d times, want 1", want, n)
		}
	}
}

// histogramCount is the observation count of bex_api_logs_page_duration_seconds
// with exactly labels (window, type, filter, direction, result).
func histogramCount(t *testing.T, reg *prometheus.Registry, labels []string) uint64 {
	t.Helper()
	families, err := reg.Gather()
	if err != nil {
		t.Fatal(err)
	}
	names := []string{"window", "type", "filter", "direction", "result"}
	for _, f := range families {
		if f.GetName() != "bex_api_logs_page_duration_seconds" {
			continue
		}
		for _, m := range f.GetMetric() {
			got := make([]string, len(names))
			for _, l := range m.GetLabel() {
				if i := slices.Index(names, l.GetName()); i >= 0 {
					got[i] = l.GetValue()
				}
			}
			if slices.Equal(got, labels) {
				return m.GetHistogram().GetSampleCount()
			}
		}
	}
	return 0
}

// The shape labels are closed sets: whatever a caller sends folds into one.
func TestPageShapeLabelsAreClosedSets(t *testing.T) {
	for _, tc := range []struct {
		q                   LogQuery
		window, typ, filter string
	}{
		{LogQuery{}, "7d", "default", "none"}, // no start reads back the store's lookback
		{LogQuery{Since: time.Now().Add(-2 * time.Hour)}, "1d", "default", "none"},
		{LogQuery{Types: []string{"nonsense"}, Level: []string{"error"}}, "7d", "other", "label"},
		{LogQuery{Types: []string{LogTypeBuild}, Build: []string{"bld-x"}}, "7d", LogTypeBuild, "label"},
		{LogQuery{Types: []string{LogTypePreDeploy}, Host: []string{"x.example"}}, "7d", LogTypePreDeploy, "line"},
		{LogQuery{Search: []string{"a"}, Path: []string{"/"}}, "7d", "default", "text"},
	} {
		if got := windowClass(tc.q); got != tc.window {
			t.Errorf("windowClass(%+v) = %q, want %q", tc.q, got, tc.window)
		}
		if got := typeClass(tc.q.Types); got != tc.typ {
			t.Errorf("typeClass(%v) = %q, want %q", tc.q.Types, got, tc.typ)
		}
		if got := filterClass(tc.q); got != tc.filter {
			t.Errorf("filterClass(%+v) = %q, want %q", tc.q, got, tc.filter)
		}
	}
	if core.ShortestLogRetention() > 7*24*time.Hour {
		t.Fatalf("the default lookback is %s; the 7d class no longer holds it", core.ShortestLogRetention())
	}
}

// A service with no Metrics wired records nothing and does not fail.
func TestAPageReadWithoutMetricsStillAnswers(t *testing.T) {
	svc := newService(nil, sampleApp("web"))
	svc.History = func(context.Context, string, LogQuery) ([]LogEntry, error) {
		return []LogEntry{lineAt(scanEnd.Add(-time.Minute), "one")}, nil
	}
	page, err := svc.queryLogPage(context.Background(), []string{"web"}, LogQuery{Since: scanStart, End: scanEnd})
	if err != nil || len(page.Entries) != 1 {
		t.Fatalf("page = %+v, err %v; want the one line", page, err)
	}
}
