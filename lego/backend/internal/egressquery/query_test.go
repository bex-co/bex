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

package egressquery

import (
	"strings"
	"testing"
)

func TestAppSourceComposition(t *testing.T) {
	specs := App("srv-one", []string{"z", "a"}, true)
	if len(specs) != 3 || specs[0].Source != HTTP || specs[1].Source != WebSocket || specs[2].Source != Direct {
		t.Fatalf("unexpected sources: %+v", specs)
	}
	query := SumRates(specs, 60)
	for _, metric := range []string{"traefik_router_responses_bytes_total", "bex_websocket_egress_bytes_total", "bex_app_direct_egress_bytes_total"} {
		if strings.Count(query, metric) != 1 {
			t.Errorf("%s occurs %d times in %q", metric, strings.Count(query, metric), query)
		}
	}
}

func TestNonApplicableSourcesAreAbsent(t *testing.T) {
	if got := App("srv-private", nil, false); len(got) != 0 {
		t.Fatalf("want no applicable sources, got %+v", got)
	}
	if got := Datastore("db-one", "postgres", false); len(got) != 0 {
		t.Fatalf("private datastore should have no public proxy source: %+v", got)
	}
	static := App("srv-static", []string{"static@kubernetes"}, false)
	if len(static) != 1 || static[0].Source != HTTP {
		t.Fatalf("static site should require HTTP only, got %+v", static)
	}
}

func TestHealthDetectsZerosAndMissingSamplesAcrossWholeWindow(t *testing.T) {
	spec := App("srv-one", []string{"router@kubernetescrd"}, true)[2]
	query := Health(spec, 3600)
	for _, want := range []string{
		`min_over_time(up{job="bex-egress-meter"}[3600s])`,
		`count_over_time(up{job="bex-egress-meter"}[3600s])`,
		`min_over_time(bex_egress_meter_healthy[3600s])`,
		`count_over_time(bex_egress_meter_healthy[3600s])`,
		`(sum(resets(bex_app_direct_egress_bytes_total{app_id="srv-one"}[3600s])) or vector(0)) == bool 0`,
		`(max(bex_egress_meter_last_counter_loss_time_seconds) or vector(0)) < bool (time() - 3600)`,
		`>= bool 192`,
	} {
		if !strings.Contains(query, want) {
			t.Errorf("health query missing %q: %s", want, query)
		}
	}
}

func TestEverySourceRejectsUnknowableProcessCounterLoss(t *testing.T) {
	specs := App("srv-one", []string{"router@kubernetescrd"}, true)
	specs = append(specs,
		Datastore("dpg-one", "postgres", true)[0],
		Datastore("kv-one", "key_value", true)[0],
	)
	for _, spec := range specs {
		query := Health(spec, 3600)
		if !strings.Contains(query, "resets("+spec.Counter) {
			t.Errorf("%s health does not reject counter resets: %s", spec.Source, query)
		}
		if spec.ProcessMetric != "" && !strings.Contains(query, "max_over_time("+spec.ProcessMetric) {
			t.Errorf("%s health does not reject a boundary restart: %s", spec.Source, query)
		}
	}
	direct := Health(specs[2], 3600)
	if !strings.Contains(direct, "last_counter_loss_time_seconds") {
		t.Fatalf("direct health must reject a durable map-loss event: %s", direct)
	}
}
