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
	"context"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// w6/m334: after each pass the gauge holds, per source, the start of the
// oldest unmetered hour, 0 for a source with none, and 0 again once the hole
// is metered, so the alert clears without a restart.
func TestPublishUnmeteredHoursSetsEverySourcesStart(t *testing.T) {
	now := time.Date(2026, 11, 12, 12, 30, 0, 0, time.UTC)
	hole := now.Truncate(time.Hour).Add(-30 * time.Hour)
	st := newMemUsageStore()
	st.oldestUnmetered = map[string]time.Time{store.UsageSourceHTTP: hole}
	metrics := NewMetrics(prometheus.NewRegistry())
	svc := &Service{Base: &core.Base{Clock: func() time.Time { return now }}, Store: st, Metrics: metrics}

	svc.publishUnmeteredHours(context.Background())
	if got := testutil.ToFloat64(metrics.oldestUnmetered.WithLabelValues(store.UsageSourceHTTP)); got != float64(hole.Unix()) {
		t.Fatalf("http start = %v, want the hole's %v", got, hole.Unix())
	}
	for _, source := range store.UsageStreamSources() {
		if source == store.UsageSourceHTTP {
			continue
		}
		if got := testutil.ToFloat64(metrics.oldestUnmetered.WithLabelValues(source)); got != 0 {
			t.Errorf("%s start = %v, want 0 with no hole", source, got)
		}
	}

	st.mu.Lock()
	st.oldestUnmetered = nil
	st.mu.Unlock()
	svc.publishUnmeteredHours(context.Background())
	if got := testutil.ToFloat64(metrics.oldestUnmetered.WithLabelValues(store.UsageSourceHTTP)); got != 0 {
		t.Fatalf("http start after the hole was metered = %v, want 0", got)
	}
}
