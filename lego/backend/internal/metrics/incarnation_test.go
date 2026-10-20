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
	"strings"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// TestPromResourceQueryExcludesPodsBeforeTheApp (w8/m124): cAdvisor pod
// names cannot tell an App from a deleted one of the same name, so every
// pod-scoped query drops the pods kube-state-metrics says predate the App.
func TestPromResourceQueryExcludesPodsBeforeTheApp(t *testing.T) {
	created := time.Unix(1_760_950_000, 0)
	const join = `unless on(pod) (kube_pod_created{namespace="tea-x"} < 1760950000)`
	for _, metric := range []string{MetricCPU, MetricMemory, MetricInstanceCount} {
		q := promResourceQueryFor(ResourceMetricsRangeRequest{Namespace: "tea-x", App: "web", Metric: metric, NotBefore: created, Resolution: time.Minute})
		if !strings.Contains(q, join) {
			t.Errorf("%s query = %s, want it to drop pods created before the App", metric, q)
		}
		if metric == MetricInstanceCount && !strings.HasPrefix(q, "count((") {
			t.Errorf("instance_count = %s, want the exclusion inside the count", q)
		}
		if unscoped := promResourceQueryFor(ResourceMetricsRangeRequest{Namespace: "tea-x", App: "web", Metric: metric, Resolution: time.Minute}); strings.Contains(unscoped, "kube_pod_created") {
			t.Errorf("%s without NotBefore = %s, want no exclusion", metric, unscoped)
		}
	}
}

// TestMetricsStartAtTheAppsCreation: a read reaching back before the App was
// created starts at its creation; one entirely before it is empty.
func TestMetricsStartAtTheAppsCreation(t *testing.T) {
	now := fixedClock()
	created := now.Add(-10 * time.Minute)
	app := sampleApp("web")
	app.Labels = map[string]string{core.LabelAppID: "srv-new"}
	app.CreationTimestamp = metav1.NewTime(created)
	svc := newService(nil, nil, app)
	var got []ResourceMetricsRangeRequest
	svc.ResourceMetricsRange = func(_ context.Context, req ResourceMetricsRangeRequest) ([]MetricSeries, error) {
		got = append(got, req)
		return nil, nil
	}
	if _, err := svc.Metrics(context.Background(), MetricQuery{App: "web", Metric: MetricCPU, Start: now.Add(-time.Hour), End: now, Resolution: time.Minute}); err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || !got[0].Start.Equal(created) || !got[0].NotBefore.Equal(created) {
		t.Fatalf("range requests = %+v, want start and NotBefore at the App's creation %s", got, created)
	}
	series, err := svc.Metrics(context.Background(), MetricQuery{App: "web", Metric: MetricCPU, Start: now.Add(-time.Hour), End: created.Add(-time.Minute), Resolution: time.Minute})
	if err != nil || len(series) != 0 || len(got) != 1 {
		t.Fatalf("a window before the App = %+v (err %v, %d source calls), want empty without a query", series, err, len(got))
	}
}
