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
	"time"

	"github.com/prometheus/client_golang/prometheus"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrl "sigs.k8s.io/controller-runtime/pkg/log"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"

	"github.com/bex-co/bex/lego/operator/internal/build"
)

// ClusterBuilder Ready / presence values. No labels: one platform builder.
const (
	clusterBuilderReadyTrue    = 1
	clusterBuilderReadyFalse   = 0
	clusterBuilderReadyUnknown = -1
	clusterBuilderPresentYes   = 1
	clusterBuilderPresentNo    = 0
	clusterBuilderPresentErr   = -1
	clusterBuilderObserveFor   = 2 * time.Second
	kpackReadyType             = "Ready"
)

var (
	clusterBuilderReadyDesc = prometheus.NewDesc(
		"bex_build_clusterbuilder_ready",
		"kpack ClusterBuilder Ready: 1 true, 0 false, -1 unknown or observation error.",
		nil, nil,
	)
	clusterBuilderPresentDesc = prometheus.NewDesc(
		"bex_build_clusterbuilder_present",
		"1 when the configured kpack ClusterBuilder exists, 0 when absent, -1 when observation failed.",
		nil, nil,
	)
	clusterBuilderResolvedDesc = prometheus.NewDesc(
		"bex_build_clusterbuilder_image_resolved_timestamp_seconds",
		"Unix timestamp of the last reviewed ClusterBuilder image digest from committed freshness metadata. Zero if that metadata is missing or malformed.",
		nil, nil,
	)
)

type clusterBuilderGetter interface {
	Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error
}

type clusterBuilderCollector struct {
	get        clusterBuilderGetter
	name       string
	resolvedAt time.Time
}

// NewClusterBuilderCollector observes the live ClusterBuilder and the
// committed resolution timestamp. Collection is scrape-time only so it cannot
// block App reconciliation.
func NewClusterBuilderCollector(get clusterBuilderGetter, resolvedAt time.Time) prometheus.Collector {
	return &clusterBuilderCollector{get: get, name: build.ClusterBuilderName, resolvedAt: resolvedAt}
}

func (c *clusterBuilderCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- clusterBuilderReadyDesc
	ch <- clusterBuilderPresentDesc
	ch <- clusterBuilderResolvedDesc
}

func (c *clusterBuilderCollector) Collect(ch chan<- prometheus.Metric) {
	present, ready := c.observe()
	ch <- prometheus.MustNewConstMetric(clusterBuilderPresentDesc, prometheus.GaugeValue, present)
	ch <- prometheus.MustNewConstMetric(clusterBuilderReadyDesc, prometheus.GaugeValue, ready)
	ts := float64(0)
	if !c.resolvedAt.IsZero() {
		ts = float64(c.resolvedAt.Unix())
	}
	ch <- prometheus.MustNewConstMetric(clusterBuilderResolvedDesc, prometheus.GaugeValue, ts)
}

func (c *clusterBuilderCollector) observe() (present, ready float64) {
	if c.get == nil {
		return clusterBuilderPresentErr, clusterBuilderReadyUnknown
	}
	ctx, cancel := context.WithTimeout(context.Background(), clusterBuilderObserveFor)
	defer cancel()
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(build.ClusterBuilderGVK)
	err := c.get.Get(ctx, client.ObjectKey{Name: c.name}, obj)
	if apierrors.IsNotFound(err) {
		return clusterBuilderPresentNo, clusterBuilderReadyUnknown
	}
	if err != nil {
		return clusterBuilderPresentErr, clusterBuilderReadyUnknown
	}
	condition, found := kpackReady(obj)
	if !found {
		return clusterBuilderPresentYes, clusterBuilderReadyUnknown
	}
	switch condition {
	case corev1.ConditionTrue:
		return clusterBuilderPresentYes, clusterBuilderReadyTrue
	case corev1.ConditionFalse:
		return clusterBuilderPresentYes, clusterBuilderReadyFalse
	default:
		return clusterBuilderPresentYes, clusterBuilderReadyUnknown
	}
}

func kpackReady(obj *unstructured.Unstructured) (corev1.ConditionStatus, bool) {
	conditions, found, _ := unstructured.NestedSlice(obj.Object, "status", "conditions")
	if !found {
		return "", false
	}
	for _, raw := range conditions {
		m, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if str(m["type"]) != kpackReadyType {
			continue
		}
		return corev1.ConditionStatus(str(m["status"])), true
	}
	return "", false
}

func str(v any) string {
	s, _ := v.(string)
	return s
}

// RegisterClusterBuilderMetrics adds scrape-time ClusterBuilder collectors to
// the controller-runtime registry. Inventory parse errors are logged and the
// resolved timestamp is exported as zero so the stale-image alert can fire.
func RegisterClusterBuilderMetrics(get clusterBuilderGetter) {
	resolved := time.Time{}
	if inv, err := build.LoadToolchainInventory(); err != nil {
		ctrl.Log.WithName("clusterbuilder-metrics").Error(err, "freshness metadata missing or malformed; image age will read as zero")
	} else if t, err := inv.ClusterBuilderResolvedAt(); err != nil {
		ctrl.Log.WithName("clusterbuilder-metrics").Error(err, "ClusterBuilder resolved_at missing or malformed; image age will read as zero")
	} else {
		resolved = t
	}
	ctrlmetrics.Registry.MustRegister(NewClusterBuilderCollector(get, resolved))
}
