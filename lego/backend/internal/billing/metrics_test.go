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

package billing

import (
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/bex-co/bex/lego/backend/internal/store"
)

func TestBillingMetricsAreBoundedAndSecretSafe(t *testing.T) {
	registry := prometheus.NewRegistry()
	metrics := NewMetrics(registry)
	metrics.SetEnabled(true)
	metrics.Operation("meter_event", "accepted")
	metrics.SetExportStats(store.BillingExportStats{
		PendingRows: 2, OldestPendingAge: 73 * time.Hour, RejectedRows: 1, AmbiguousRows: 1,
	})
	metrics.WebhookSucceeded(time.Unix(100, 0))

	families, err := registry.Gather()
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{
		"bex_billing_enabled": false, "bex_billing_operations_total": false,
		"bex_billing_outbox_pending_rows": false, "bex_billing_outbox_oldest_pending_age_seconds": false,
		"bex_billing_export_rejected_rows": false, "bex_billing_export_ambiguous_rows": false,
		"bex_billing_webhook_last_success_timestamp_seconds": false,
	}
	for _, family := range families {
		if _, ok := want[family.GetName()]; !ok {
			t.Fatalf("unexpected billing metric %q", family.GetName())
		}
		want[family.GetName()] = true
		for _, metric := range family.Metric {
			for _, label := range metric.Label {
				if label.GetName() != "operation" && label.GetName() != "result" {
					t.Fatalf("unbounded label %q on %s", label.GetName(), family.GetName())
				}
			}
		}
	}
	for name, found := range want {
		if !found {
			t.Errorf("metric %s was not gathered", name)
		}
	}
}
