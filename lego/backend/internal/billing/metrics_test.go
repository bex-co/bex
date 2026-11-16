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
	"os"
	"regexp"
	"strings"
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

// alertingRulesPath is the rule pack the billing alerts live in; the backend
// workflow lists it among the files its tests read.
const alertingRulesPath = "../../../../deploy/gitops/base/rules/alerting_rules.yml"

// TestAlertedBillingOperationsStartAtZero pins the two halves of one contract:
// every operation an alert reads with result="error" is exported at 0 from the
// first scrape (so increase() counts the first failure on a new pod), and the
// list of those operations is the one the rules file actually names.
func TestAlertedBillingOperationsStartAtZero(t *testing.T) {
	registry := prometheus.NewRegistry()
	NewMetrics(registry)
	families, err := registry.Gather()
	if err != nil {
		t.Fatal(err)
	}
	starts := map[string]float64{}
	for _, family := range families {
		if family.GetName() != "bex_billing_operations_total" {
			continue
		}
		for _, metric := range family.Metric {
			labels := map[string]string{}
			for _, label := range metric.Label {
				labels[label.GetName()] = label.GetValue()
			}
			if labels["result"] == "error" {
				starts[labels["operation"]] = metric.GetCounter().GetValue()
			}
		}
	}
	for _, operation := range alertedErrorOperations {
		value, ok := starts[operation]
		if !ok {
			t.Errorf("operation %q is not exported before its first failure", operation)
		} else if value != 0 {
			t.Errorf("operation %q starts at %v, want 0", operation, value)
		}
	}

	rules, err := os.ReadFile(alertingRulesPath)
	if err != nil {
		t.Fatal(err)
	}
	selector := regexp.MustCompile(`bex_billing_operations_total\{([^}]*)\}`)
	operationMatcher := regexp.MustCompile(`operation=~?"([^"]*)"`)
	registered := map[string]bool{}
	for _, operation := range alertedErrorOperations {
		registered[operation] = true
	}
	var seen int
	for _, match := range selector.FindAllStringSubmatch(string(rules), -1) {
		if !strings.Contains(match[1], `result="error"`) {
			continue
		}
		op := operationMatcher.FindStringSubmatch(match[1])
		if op == nil {
			t.Errorf("an alert reads bex_billing_operations_total{%s} without naming an operation", match[1])
			continue
		}
		for _, operation := range strings.Split(op[1], "|") {
			seen++
			if !registered[operation] {
				t.Errorf("alerting_rules.yml reads operation %q with result=\"error\" but billing.NewMetrics does not register it at 0; add it to alertedErrorOperations", operation)
			}
		}
	}
	if seen == 0 {
		t.Fatal("found no bex_billing_operations_total error selector in alerting_rules.yml; the rule pack moved or its shape changed")
	}
}
