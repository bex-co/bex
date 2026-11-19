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

package controller

import (
	"context"

	"github.com/prometheus/client_golang/prometheus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// Apps outside their workspace's namespace (w5/195).
//
// Under ADR043 a workspace is a namespace: bex-api creates every App in the
// namespace named by its app.bex.co/workspace label, and the API, dashboard,
// billing, quotas and the per-workspace isolation policies all find Apps that
// way. An App applied by hand (kubectl into the bootstrap `default` namespace,
// or into any namespace without a matching label) is invisible to all of them
// while it may still serve on the platform domain: production's
// default/hello-static did exactly that for four months. Nothing else lists
// such an App, so this collector names each one.
var appOutsideWorkspaceDesc = prometheus.NewDesc(
	"bex_app_placement_outside_workspace",
	"1 for each App CR whose namespace is not its app.bex.co/workspace label (an unlabeled App included). Absent when every App lives in its workspace's namespace.",
	[]string{metricLabelNamespace, "name", "workspace"}, nil,
)

type appPlacementCollector struct {
	list   datastoreLister
	errors prometheus.Counter
}

// NewAppPlacementCollector reports every App outside its workspace's
// namespace. It lists metadata only, at scrape time, so it never blocks a
// reconcile and never reads App specs or statuses.
func NewAppPlacementCollector(list datastoreLister) prometheus.Collector {
	return &appPlacementCollector{
		list: list,
		errors: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "bex_app_placement_observe_errors_total",
			Help: "Scrapes whose App listing failed. Non-zero means bex_app_placement_outside_workspace is incomplete and its absence must not be read as clean.",
		}),
	}
}

func (c *appPlacementCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- appOutsideWorkspaceDesc
	c.errors.Describe(ch)
}

func (c *appPlacementCollector) Collect(ch chan<- prometheus.Metric) {
	ctx, cancel := context.WithTimeout(context.Background(), datastoreObserveFor)
	defer cancel()

	apps := &metav1.PartialObjectMetadataList{}
	apps.SetGroupVersionKind(appv1alpha1.SchemeGroupVersion.WithKind("AppList"))
	if err := c.list.List(ctx, apps); err != nil {
		c.errors.Inc()
	} else {
		for i := range apps.Items {
			app := &apps.Items[i]
			workspace := app.Labels[labelWorkspace]
			if workspace != "" && workspace == app.Namespace {
				continue
			}
			ch <- prometheus.MustNewConstMetric(appOutsideWorkspaceDesc, prometheus.GaugeValue, 1,
				app.Namespace, app.Name, workspace)
		}
	}
	ch <- c.errors
}

// RegisterAppPlacementMetrics wires the App placement collector into the
// controller metrics registry.
func RegisterAppPlacementMetrics(list datastoreLister) {
	ctrlmetrics.Registry.MustRegister(NewAppPlacementCollector(list))
}
