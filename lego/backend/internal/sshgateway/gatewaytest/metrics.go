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

package gatewaytest

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

// MetricValue reads one sample from the gatherer: the metric carrying every
// name/value pair in labels, or — when labels is empty — the family's single
// label-less sample (gauges). A family or label set never incremented reads as
// 0, so a suite can assert "still zero" without seeding samples. Shared by the
// transport suites and cmd/ssh-gateway for the same reason the fakes are: one
// registry walk cannot drift between them.
func MetricValue(t *testing.T, g prometheus.Gatherer, name string, labels map[string]string) float64 {
	t.Helper()
	families, err := g.Gather()
	if err != nil {
		t.Fatal(err)
	}
	for _, family := range families {
		if family.GetName() != name {
			continue
		}
	samples:
		for _, metric := range family.Metric {
			if len(labels) == 0 && len(metric.Label) != 0 {
				continue
			}
			for wantName, wantValue := range labels {
				matched := false
				for _, label := range metric.Label {
					if label.GetName() == wantName && label.GetValue() == wantValue {
						matched = true
						break
					}
				}
				if !matched {
					continue samples
				}
			}
			// A metric is only ever one of the two, so the other reads as 0.
			return metric.GetGauge().GetValue() + metric.GetCounter().GetValue()
		}
	}
	return 0
}
