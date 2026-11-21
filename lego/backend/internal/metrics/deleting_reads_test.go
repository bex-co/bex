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
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/savedsearch"
)

// deleting stamps obj as a CR whose delete Kubernetes accepted while its
// finalizer still holds it.
func deleting[T client.Object](obj T) T {
	now := metav1.NewTime(time.Unix(1_700_000_000, 0))
	obj.SetDeletionTimestamp(&now)
	obj.SetFinalizers([]string{"app.bex.co/finalizer"})
	return obj
}

// w8/366: for about 10 s after `bex services delete`, GET /v1/metrics/cpu and
// /v1/metrics/bandwidth still answered 200 with the service's samples while
// every other read of it was 404. Every metrics read of a service, Postgres or
// Key Value whose delete was accepted now answers as for one that never
// existed; a live service keeps its series.
func TestDeletingResourcesHaveNoMetrics(t *testing.T) {
	const pgID, kvID = "dpg-d0000000000000000001", "red-d0000000000000000001"
	svc := newService(staticResourceMetrics(map[string]PodResourceUsage{
		"gone-1": {CPUCores: 0.5, MemoryBytes: 512 << 20},
		webInst:  {CPUCores: 0.5, MemoryBytes: 512 << 20},
	}), nil,
		deleting(sampleApp("gone")), podWithLimitsFor("gone", "gone-1"),
		sampleApp("web"), podWithLimits(webInst),
		deleting(sampleDatabase(pgID, false)), deleting(sampleKeyValue(kvID)),
	)
	ctx := context.Background()

	for _, path := range []string{"/v1/metrics/cpu?resource=gone", "/v1/metrics/bandwidth?resource=gone", "/v1/metrics/cpu?resource=" + pgID} {
		if code := serveREST(svc, path).Code; code != 404 {
			t.Errorf("GET %s for a deleting resource = %d, want 404", path, code)
		}
	}
	if _, err := svc.Metrics(ctx, MetricQuery{App: "gone", Metric: MetricCPU}); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("Metrics of a deleting service = %v, want ErrNotFound", err)
	}
	if _, err := svc.MetricsFilters(ctx, MetricsFiltersQuery{App: "gone", OutputFilters: []string{filterFieldStatusCode}}); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("MetricsFilters of a deleting service = %v, want ErrNotFound", err)
	}
	if _, err := svc.MonthToDateBandwidth(ctx, "gone"); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("MonthToDateBandwidth of a deleting service = %v, want ErrNotFound", err)
	}
	if _, err := svc.DatastoreMetrics(ctx, DatastoreMetricQuery{Kind: DatastoreService, Resource: "gone", Metric: MetricDisk}); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("disk metrics of a deleting service = %v, want ErrNotFound", err)
	}
	for _, resource := range []string{"gone", pgID, kvID} {
		if _, err := svc.MetricsRetention(ctx, resource); !errors.Is(err, core.ErrNotFound) {
			t.Errorf("MetricsRetention of deleting %s = %v, want ErrNotFound", resource, err)
		}
	}
	for _, r := range []savedsearch.Resource{{Type: savedsearch.KindPostgres, ID: pgID}, {Type: savedsearch.KindKeyValue, ID: kvID}, {ID: "gone"}} {
		if err := svc.authorizeProjectResource(ctx, r); !errors.Is(err, core.ErrNotFound) {
			t.Errorf("a project's view check of deleting %s = %v, want ErrNotFound", r.ID, err)
		}
	}

	if series, err := svc.Metrics(ctx, MetricQuery{App: "web", Metric: MetricCPU}); err != nil || len(series) == 0 {
		t.Errorf("Metrics of a live service = %v, %v; want its series", series, err)
	}
	if code := serveREST(svc, "/v1/metrics/cpu?resource=web").Code; code != 200 {
		t.Errorf("GET /v1/metrics/cpu for a live service = %d, want 200", code)
	}
}
