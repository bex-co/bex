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
	"fmt"
	"os/exec"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/bex-co/bex/lego/backend/internal/testenv"
	"github.com/bex-co/bex/lego/types/podmetrics"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// Turning on edge caching changes no Metrics chart (w3/m487). The edge-cache
// sidecar shares an edge-cached service's pods, but Render's cache runs on its
// CDN, outside the instance, so the service's CPU and memory usage and the
// limit its percentage view divides by count only its own containers. The pod
// below runs its app at 0.3 cores / 300Mi under a starter plan's 0.5 / 512Mi,
// beside a sidecar at 0.08 cores / 150Mi under its own 0.1 / 160Mi. Every read
// must report the app's numbers alone, evaluated in Prometheus' own engine, and
// Total Instances still counts the one pod.
func TestServiceResourceQueriesLeaveTheEdgeCacheOutOnPrometheus(t *testing.T) {
	promtool, err := exec.LookPath("promtool")
	if err != nil {
		testenv.Skip(t, "promtool not on PATH")
	}
	const pod = "web-6d5896f9c9-abcde"
	const mi = 1 << 20
	series := func(metric, container, extra string) string {
		return fmt.Sprintf(`%s{namespace="ns",pod=%q,container=%q%s}`, metric, pod, container, extra)
	}
	// Counters grow per 15s sample by cores × 15.
	counter := func(cores float64) string { return fmt.Sprintf("0+%gx60", cores*15) }
	gauge := func(v float64) string { return fmt.Sprintf("%gx60", v) }
	input := []struct{ series, values string }{
		{series("container_cpu_usage_seconds_total", "app", ""), counter(0.3)},
		{series("container_cpu_usage_seconds_total", "edge-cache", ""), counter(0.08)},
		{series("container_memory_working_set_bytes", "app", ""), gauge(300 * mi)},
		{series("container_memory_working_set_bytes", "edge-cache", ""), gauge(150 * mi)},
		// cAdvisor's pod-level aggregate row, which no read may add either.
		{series("container_memory_working_set_bytes", "", ""), gauge(450 * mi)},
		{series("kube_pod_container_resource_limits", "app", `,resource="cpu"`), gauge(0.5)},
		{series("kube_pod_container_resource_limits", "edge-cache", `,resource="cpu"`), gauge(0.1)},
		{series("kube_pod_container_resource_limits", "app", `,resource="memory"`), gauge(512 * mi)},
		{series("kube_pod_container_resource_limits", "edge-cache", `,resource="memory"`), gauge(160 * mi)},
	}
	usage := func(metric string) string {
		return promResourceQueryFor(ResourceMetricsRangeRequest{Namespace: "ns", App: "web", Metric: metric, Resolution: time.Minute})
	}
	limit := func(metric string) string {
		return promLimitQueryFor(ResourceLimitRangeRequest{Namespace: "ns", App: "web", Metric: metric, Resolution: time.Minute})
	}
	onePod := fmt.Sprintf(`{pod=%q}`, pod)
	checks := []struct {
		name, expr, labels string
		value              float64
	}{
		{"cpu usage", usage(MetricCPU), onePod, 0.3},
		{"memory usage", usage(MetricMemory), onePod, 300 * mi},
		{"cpu limit history", limit(MetricCPU), onePod, 0.5},
		{"memory limit history", limit(MetricMemory), onePod, 512 * mi},
		{"instance count", usage(MetricInstanceCount), `{}`, 1},
	}
	var b strings.Builder
	b.WriteString("rule_files: []\nevaluation_interval: 15s\ntests:\n  - interval: 15s\n    input_series:\n")
	for _, s := range input {
		fmt.Fprintf(&b, "      - series: '%s'\n        values: '%s'\n", s.series, s.values)
	}
	b.WriteString("    promql_expr_test:\n")
	for _, c := range checks {
		// Rounded: a rate is exact only up to float error.
		fmt.Fprintf(&b, "      # %s\n      - expr: 'round(%s, 0.001)'\n        eval_time: 10m\n        exp_samples:\n          - labels: '%s'\n            value: %g\n",
			c.name, strings.ReplaceAll(c.expr, "'", "''"), c.labels, c.value)
	}
	runPromtoolTests(t, promtool, b.String())
}

// Every per-container read of a service leaves the sidecar out, whatever its
// resolution or CPU aggregation: usage, the limit history, and disk activity
// on a service's persistent disk. Total Instances counts pods, not
// containers, so it keeps the matcher the usage meter bills instance-seconds
// by (egressquery.PodNameMatcher), which must select the same pods.
func TestEveryServiceContainerReadLeavesTheEdgeCacheOut(t *testing.T) {
	const sidecar = `container!="edge-cache"`
	usage := func(metric string, res time.Duration, agg string) string {
		return promResourceQueryFor(ResourceMetricsRangeRequest{Namespace: "ns", App: "web", Metric: metric, Resolution: res, CPUAggregation: agg})
	}
	disk := func(metric string) string {
		q, _ := promDatastoreActivityQuery(DatastoreActivityRequest{Kind: DatastoreService, Namespace: "ns", Resource: "web", Metric: metric, Resolution: time.Minute})
		return q
	}
	reads := map[string]string{
		"cpu":               usage(MetricCPU, time.Minute, ""),
		"cpu MAX":           usage(MetricCPU, 5*time.Minute, cpuAggregationMax),
		"cpu MIN":           usage(MetricCPU, 5*time.Minute, cpuAggregationMin),
		"memory":            usage(MetricMemory, time.Minute, ""),
		"memory, coarse":    usage(MetricMemory, time.Hour, ""),
		"cpu limit":         promLimitQueryFor(ResourceLimitRangeRequest{Namespace: "ns", App: "web", Metric: MetricCPU}),
		"memory limit":      promLimitQueryFor(ResourceLimitRangeRequest{Namespace: "ns", App: "web", Metric: MetricMemory}),
		"disk activity":     disk(MetricDiskActivity),
		"disk operations":   disk(MetricDiskOperations),
		"usage, not before": promResourceQueryFor(ResourceMetricsRangeRequest{Namespace: "ns", App: "web", Metric: MetricMemory, Resolution: time.Minute, NotBefore: time.Unix(1_700_000_000, 0)}),
	}
	for name, q := range reads {
		// Each container selector in the expression carries the exclusion.
		if n, m := strings.Count(q, `container!=""`), strings.Count(q, sidecar); n == 0 || n != m {
			t.Errorf("%s: %d of %d container selectors leave the edge-cache sidecar out:\n%s", name, m, n, q)
		}
	}
	for _, res := range []time.Duration{time.Minute, time.Hour} {
		if q := usage(MetricInstanceCount, res, ""); strings.Contains(q, sidecar) {
			t.Errorf("instance count at %s counts pods by the billing meter's matcher, got %s", res, q)
		}
	}
}

// Without Prometheus, the metrics-server snapshot and the pods' spec limits
// serve the same reads, and they leave the sidecar out alike: a cached starter
// instance reads 300Mi of its plan's 512Mi (58.59375%), never 450Mi of 672Mi,
// and its limit line is the plan's on /cpu-limit and /memory-limit.
func TestTheSnapshotPathLeavesTheEdgeCacheOut(t *testing.T) {
	pod := podWithCPUAndMemoryLimits("web", webInst, "500m", "512Mi")
	pod.Spec.Containers = append(pod.Spec.Containers, corev1.Container{
		Name: appv1alpha1.EdgeCacheContainer,
		Resources: corev1.ResourceRequirements{Limits: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("100m"),
			corev1.ResourceMemory: resource.MustParse("160Mi"),
		}},
	})
	snapshot := func(context.Context, string, string) ([]PodResourceUsage, error) {
		return podmetrics.Parse([]byte(`{"items":[{"metadata":{"name":"` + webInst + `"},"containers":[
		  {"name":"app","usage":{"cpu":"300m","memory":"300Mi"}},
		  {"name":"edge-cache","usage":{"cpu":"80m","memory":"150Mi"}}]}]}`))
	}
	svc := newService(snapshot, nil, sampleApp("web"), pod)
	const mi = 1 << 20
	for _, tc := range []struct {
		name      string
		q         MetricQuery
		unit      string
		wantValue float64
	}{
		{"cpu", MetricQuery{Metric: MetricCPU}, unitCores, 0.3},
		{"memory", MetricQuery{Metric: MetricMemory}, unitBytes, 300 * mi},
		{"cpu %", MetricQuery{Metric: MetricCPU, Percentage: true}, unitPercentage, 60},
		{"memory %", MetricQuery{Metric: MetricMemory, Percentage: true}, unitPercentage, 58.59375},
		{"cpu limit", MetricQuery{Metric: MetricCPULimit}, unitCores, 0.5},
		{"memory limit", MetricQuery{Metric: MetricMemoryLimit}, unitBytes, 512 * mi},
	} {
		tc.q.App = "web"
		series := mustMetrics(t, svc, tc.q)
		if len(series) != 1 || len(series[0].Points) != 1 {
			t.Errorf("%s: want one instance's one point, got %+v", tc.name, series)
			continue
		}
		if got := series[0].Points[0].Value; series[0].Unit != tc.unit || !closeTo(got, tc.wantValue) {
			t.Errorf("%s: got %v %s, want %v %s", tc.name, got, series[0].Unit, tc.wantValue, tc.unit)
		}
	}
}
