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

package webhooks

import (
	"errors"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/expfmt"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

func TestWebhookMetricsDistinguishAutomaticAndManualWithoutResourceLabels(t *testing.T) {
	registry := prometheus.NewRegistry()
	metrics := NewMetrics(registry)
	metrics.ObserveWebhookAttempt("automatic", "failed")
	metrics.ObserveWebhookAttempt("manual", "delivered")
	metrics.ObserveWebhookAttempt("whk-secret", "evt-secret")
	metrics.ObserveWebhookAdmission(store.WebhookEnqueueResult{
		Admitted: 4, Capped: 12, Deduplicated: 1,
	})
	metrics.observeResend(nil)
	metrics.observeResend(core.NewConflictError(WebhookEndpointDisabledCode, "disabled", nil))
	metrics.observeResend(errors.New("https://secret.example/hook"))

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
				case "origin", "result":
				default:
					t.Fatalf("unbounded webhook metric label %q", label.GetName())
				}
			}
		}
	}
	text := rendered.String()
	for _, leaked := range []string{"whk-secret", "evt-secret", "secret.example"} {
		if strings.Contains(text, leaked) {
			t.Fatalf("metrics leaked %q:\n%s", leaked, text)
		}
	}
	for _, want := range []string{
		`bex_webhooks_delivery_attempts_total{origin="automatic",result="failed"} 1`,
		`bex_webhooks_delivery_attempts_total{origin="manual",result="delivered"} 1`,
		`bex_webhooks_delivery_attempts_total{origin="unknown",result="unknown"} 1`,
		`bex_webhooks_delivery_admissions_total{result="admitted"} 4`,
		`bex_webhooks_delivery_admissions_total{result="capped"} 12`,
		`bex_webhooks_delivery_admissions_total{result="deduplicated"} 1`,
		`bex_webhooks_delivery_capped_batch_size_count 1`,
		`bex_webhooks_delivery_capped_batch_size_sum 12`,
		`bex_webhooks_resend_requests_total{result="disabled"} 1`,
		`bex_webhooks_resend_requests_total{result="queued"} 1`,
	} {
		if !strings.Contains(text, want) {
			t.Errorf("metrics missing %q:\n%s", want, text)
		}
	}
}
