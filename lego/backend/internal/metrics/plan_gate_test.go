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

package metrics

import (
	"context"
	"errors"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// w8/m138: Render's response latency metrics are Pro and higher. On Hobby an
// http_latency read is refused with PLAN_FEATURE_REQUIRED before any source is
// read; request counts stay open on every plan.
func TestLatencyMetricsFollowThePlan(t *testing.T) {
	app := sampleApp("web")
	app.Labels = map[string]string{core.LabelTenant: "tea-a"}
	for plan, refused := range map[string]bool{"hobby": true, "pro": false} {
		t.Run(plan, func(t *testing.T) {
			read := 0
			svc := newService(nil, func(context.Context, RequestMetricsRequest) ([]MetricSeries, error) {
				read++
				return nil, nil
			}, app, podFor("web", webInst))
			svc.Plans = core.WorkspacePlanFunc(func(context.Context, string) (string, error) { return plan, nil })
			ctx := context.Background()

			_, err := svc.Metrics(ctx, MetricQuery{App: "web", Metric: MetricHTTPLatency})
			var coded *core.CodedError
			if refused {
				if !errors.As(err, &coded) || coded.Code != core.PlanFeatureRequiredCode || coded.Params["feature"] != "latencyMetrics" ||
					err.Error() != "bad request: response latency metrics require the Pro plan or higher; this workspace is on Hobby" {
					t.Fatalf("hobby latency: got %v", err)
				}
				if read != 0 {
					t.Fatalf("a refused read reached the source %d times", read)
				}
			} else if err != nil {
				t.Fatalf("pro latency: %v", err)
			}
			if _, err := svc.Metrics(ctx, MetricQuery{App: "web", Metric: MetricHTTPRequests}); err != nil {
				t.Fatalf("%s: request counts stay open: %v", plan, err)
			}
		})
	}
}
