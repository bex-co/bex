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
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// A disk service's Recreate deploy runs a disk holder's pod beside its
// instance (w6/m559), and a code deploy a pre-pull's pod too (w6/m595). Each
// carries the App's label, so the metrics API's snapshot reports its usage
// too; neither is an instance, and neither is a series.
func TestMetricsServerSnapshotShowsOnlyTheInstances(t *testing.T) {
	holder := podFor("web", "dskhld-web-x2k9p")
	holder.Labels[core.PodLabelComponent] = appv1alpha1.ComponentDiskHolder
	prePull := podFor("web", "dskpul-web-7fq2m")
	prePull.Labels[core.PodLabelComponent] = appv1alpha1.ComponentDiskPrePull
	svc := newService(staticResourceMetrics(map[string]PodResourceUsage{
		webInst:      {CPUCores: 0.5, MemoryBytes: 512 << 20},
		holder.Name:  {CPUCores: 0.0001, MemoryBytes: 300 << 10},
		prePull.Name: {CPUCores: 0.0001, MemoryBytes: 200 << 10},
	}), nil, sampleApp("web"), podWithLimits(webInst), holder, prePull)

	for _, metric := range []string{MetricCPU, MetricMemory} {
		series, err := svc.Metrics(context.Background(), MetricQuery{App: "web", Metric: metric})
		if err != nil {
			t.Fatal(err)
		}
		if len(series) != 1 || series[0].Labels["instance"] != ids.ServiceInstanceID("web", webInst) {
			t.Errorf("%s series = %+v, want the one instance's", metric, series)
		}
	}
}
