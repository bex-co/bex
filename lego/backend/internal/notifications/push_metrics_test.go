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

package notifications

import (
	"strings"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/store"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/expfmt"
)

func TestPushMetricsExposeOnlyBoundedLabels(t *testing.T) {
	registry := prometheus.NewRegistry()
	metrics := NewPushMetrics(registry)
	metrics.SetEnabled(true)
	metrics.Operation("send", "retry")
	metrics.Operation("receipt", "delivered")
	metrics.Operation("prune", "invalid_token")
	metrics.Transport("expo", "accepted")
	metrics.Transport("webpush", "delivered")
	metrics.SetQueue(store.PushQueueStats{Pending: 2, ReceiptPending: 3, Terminal: 5})
	metrics.Succeeded(time.Unix(123, 0))

	families, err := registry.Gather()
	if err != nil {
		t.Fatal(err)
	}
	var rendered strings.Builder
	for _, family := range families {
		if _, err := expfmt.MetricFamilyToText(&rendered, family); err != nil {
			t.Fatal(err)
		}
		for _, metric := range family.Metric {
			for _, label := range metric.Label {
				switch label.GetName() {
				case "operation", "result", "state", "transport":
				default:
					t.Fatalf("unbounded push metric label %q", label.GetName())
				}
			}
		}
	}
	text := rendered.String()
	for _, secret := range []string{"tenant-identifier", "subject@example.com", "ExponentPushToken", "ticket-one"} {
		if strings.Contains(text, secret) {
			t.Fatalf("metrics leaked %q: %s", secret, text)
		}
	}
	for _, want := range []string{
		`bex_push_enabled 1`,
		`bex_push_operations_total{operation="receipt",result="delivered"} 1`,
		`bex_push_transport_total{result="delivered",transport="webpush"} 1`,
		`bex_push_queue_rows{state="receipt_pending"} 3`,
	} {
		if !strings.Contains(text, want) {
			t.Errorf("metrics missing %q:\n%s", want, text)
		}
	}
}
