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
// oldest unmetered hour a meter can still fill, 0 for a source with none, and
// 0 again once the hole is metered, so the alert clears without a restart.
func TestPublishUnmeteredHoursSetsEverySourcesStart(t *testing.T) {
	now := time.Date(2026, 11, 12, 12, 30, 0, 0, time.UTC)
	hole := now.Truncate(time.Hour).Add(-30 * time.Hour)
	st := newMemUsageStore()
	st.holes = []store.UsageHole{{ResourceKind: store.ResourceKindService, ServiceID: "srv-live", Kind: store.UsageKindEgressBytes, Source: store.UsageSourceHTTP, Window: hole}}
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
	st.holes = nil
	st.mu.Unlock()
	svc.publishUnmeteredHours(context.Background())
	if got := testutil.ToFloat64(metrics.oldestUnmetered.WithLabelValues(store.UsageSourceHTTP)); got != 0 {
		t.Fatalf("http start after the hole was metered = %v, want 0", got)
	}
}

// A deleted App owes what is still metered for it: its egress when its
// identity was recorded, and its disk. A hole no meter will fill is an
// undercount coverage reports, and paging on it would page for two days on
// nothing an operator can fix (w6/m334, found on production 2026-11-12).
func TestPublishUnmeteredHoursSkipsHolesNoMeterWillFill(t *testing.T) {
	now := time.Date(2026, 11, 12, 12, 30, 0, 0, time.UTC)
	old, newer := now.Truncate(time.Hour).Add(-45*time.Hour), now.Truncate(time.Hour).Add(-30*time.Hour)
	st := newMemUsageStore()
	st.retired = []store.RetiredUsageResource{
		{ResourceKind: store.ResourceKindService, ResourceID: "srv-gone", RetiredAt: old},
		{ResourceKind: store.ResourceKindService, ResourceID: "srv-known", RetiredAt: newer, Egress: &store.EgressIdentity{Direct: true}},
	}
	st.holes = []store.UsageHole{
		{ResourceKind: store.ResourceKindService, ServiceID: "srv-gone", Kind: store.UsageKindEgressBytes, Source: store.UsageSourceDirect, Window: old},
		{ResourceKind: store.ResourceKindService, ServiceID: "srv-known", Kind: store.UsageKindEgressBytes, Source: store.UsageSourceDirect, Window: newer},
		{ResourceKind: store.ResourceKindService, ServiceID: "srv-gone", Kind: store.UsageKindDiskGBSeconds, Source: store.UsageSourceDisk, Window: old},
	}
	metrics := NewMetrics(prometheus.NewRegistry())
	svc := &Service{Base: &core.Base{Clock: func() time.Time { return now }}, Store: st, Metrics: metrics}

	svc.publishUnmeteredHours(context.Background())
	if got := testutil.ToFloat64(metrics.oldestUnmetered.WithLabelValues(store.UsageSourceDirect)); got != float64(newer.Unix()) {
		t.Errorf("direct start = %v, want the recorded-identity App's %v, not the unfillable %v", got, newer.Unix(), old.Unix())
	}
	if got := testutil.ToFloat64(metrics.oldestUnmetered.WithLabelValues(store.UsageSourceDisk)); got != float64(old.Unix()) {
		t.Errorf("disk start = %v, want %v: the disk meter still bills a deleted App's last disk hour", got, old.Unix())
	}
}
