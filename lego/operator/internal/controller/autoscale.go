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
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"sort"
	"strconv"
	"time"

	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/types/tiers"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// autoscaleInterval is how often the autoscaling loop re-evaluates utilization
// when a service has autoscaling enabled. Chosen to match the metrics-server's
// default scrape interval (60 s) so we never ask faster than data refreshes.
const autoscaleInterval = 30 * time.Second

// scaleDownStabilizationWindow is the minimum time that must elapse above the
// desired scale-down replica count before the reconciler actually writes a lower
// replica count. This prevents oscillation during brief load drops (HPA-style).
const scaleDownStabilizationWindow = 5 * time.Minute

// annotAutoscaleScaleDown records the RFC3339 timestamp when the reconciler
// first decided to scale down; the decision is only committed after the
// stabilization window has passed.
const annotAutoscaleScaleDown = "app.bex.co/autoscale-scale-down-at"

// annotAutoscaleReplicas persists the autoscaler's last decided replica count
// between reconcile passes. Stored as an annotation (not spec.replicas) so it
// doesn't bump metadata.generation — a spec change bumps generation, which
// causes the reconciler to treat git-backed Apps as needing a rebuild even
// though only the replica count changed. This was the root cause of the
// 51-generation / 9.7 Gi registry incident (eden-cms-v2-git, 2026-07-12).
const annotAutoscaleReplicas = "app.bex.co/autoscale-replicas"

// PodUsage is one pod's current resource consumption, as reported by metrics-server.
type PodUsage struct {
	Pod         string
	CPUCores    float64
	MemoryBytes float64
}

// MetricsReader reads the current resource usage of an App's pods from
// metrics-server. Returns one entry per running pod. A nil MetricsReader
// disables metrics-based autoscaling (the reconciler skips it).
type MetricsReader func(ctx context.Context, namespace, app string) ([]PodUsage, error)

// NewMetricsServerReader returns a MetricsReader backed by the metrics.k8s.io
// aggregated API (metrics-server). Uses the kubernetes clientset's REST client
// to avoid a direct dependency on the metrics-server client library.
func NewMetricsServerReader(cs kubernetes.Interface) MetricsReader {
	return func(ctx context.Context, namespace, app string) ([]PodUsage, error) {
		raw, err := cs.Discovery().RESTClient().Get().
			AbsPath("/apis/metrics.k8s.io/v1beta1/namespaces", namespace, "pods").
			Param("labelSelector", labelApp+"="+app).
			DoRaw(ctx)
		if err != nil {
			return nil, err
		}
		return parsePodUsage(raw)
	}
}

// NewPrometheusMetricsReader returns a MetricsReader that reads current CPU/memory
// from Prometheus using an instant query over cAdvisor metrics. Used when
// metrics-server is unavailable (e.g. BEX_PROM_URL is set but metrics-server is not).
func NewPrometheusMetricsReader(promURL string, hc *http.Client) MetricsReader {
	if hc == nil {
		hc = http.DefaultClient
	}
	return func(ctx context.Context, namespace, app string) ([]PodUsage, error) {
		// instant query: current per-pod CPU rate + memory
		cpuQ := fmt.Sprintf(
			`sum by (pod) (rate(container_cpu_usage_seconds_total{namespace=%q,pod=~"%s-[a-z0-9]+-[a-z0-9]{5}",container!=""}[2m]))`,
			namespace, app)
		memQ := fmt.Sprintf(
			`sum by (pod) (container_memory_working_set_bytes{namespace=%q,pod=~"%s-[a-z0-9]+-[a-z0-9]{5}",container!=""})`,
			namespace, app)
		cpuSeries, err := promInstantQuery(ctx, hc, promURL, cpuQ)
		if err != nil {
			return nil, fmt.Errorf("prometheus cpu query: %w", err)
		}
		memSeries, err := promInstantQuery(ctx, hc, promURL, memQ)
		if err != nil {
			return nil, fmt.Errorf("prometheus memory query: %w", err)
		}
		byPod := map[string]*PodUsage{}
		for _, s := range cpuSeries {
			pod := s.labels["pod"]
			if pod == "" {
				continue
			}
			if byPod[pod] == nil {
				byPod[pod] = &PodUsage{Pod: pod}
			}
			byPod[pod].CPUCores = s.value
		}
		for _, s := range memSeries {
			pod := s.labels["pod"]
			if pod == "" {
				continue
			}
			if byPod[pod] == nil {
				byPod[pod] = &PodUsage{Pod: pod}
			}
			byPod[pod].MemoryBytes = s.value
		}
		out := make([]PodUsage, 0, len(byPod))
		for _, u := range byPod {
			out = append(out, *u)
		}
		sort.Slice(out, func(i, j int) bool { return out[i].Pod < out[j].Pod })
		return out, nil
	}
}

// podMetricsList is the subset of metrics.k8s.io/v1beta1 PodMetricsList bex reads.
type podMetricsList struct {
	Items []struct {
		Metadata struct {
			Name string `json:"name"`
		} `json:"metadata"`
		Containers []struct {
			Usage struct {
				CPU    string `json:"cpu"`
				Memory string `json:"memory"`
			} `json:"usage"`
		} `json:"containers"`
	} `json:"items"`
}

func parsePodUsage(raw []byte) ([]PodUsage, error) {
	var list podMetricsList
	if err := json.Unmarshal(raw, &list); err != nil {
		return nil, fmt.Errorf("decode pod metrics: %w", err)
	}
	out := make([]PodUsage, 0, len(list.Items))
	for _, it := range list.Items {
		u := PodUsage{Pod: it.Metadata.Name}
		for _, ctr := range it.Containers {
			if q, err := resource.ParseQuantity(ctr.Usage.CPU); err == nil {
				u.CPUCores += q.AsApproximateFloat64()
			}
			if q, err := resource.ParseQuantity(ctr.Usage.Memory); err == nil {
				u.MemoryBytes += float64(q.Value())
			}
		}
		out = append(out, u)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Pod < out[j].Pod })
	return out, nil
}

// promInstantSample is one labeled scalar from a Prometheus instant query.
type promInstantSample struct {
	labels map[string]string
	value  float64
}

// promInstantQuery fires an instant query against Prometheus and returns the
// result vector as (labels, float64) pairs.
func promInstantQuery(ctx context.Context, hc *http.Client, base, query string) ([]promInstantSample, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		fmt.Sprintf("%s/api/v1/query?query=%s", base, query), nil)
	if err != nil {
		return nil, err
	}
	resp, err := hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("prometheus: status %d", resp.StatusCode)
	}
	var pr struct {
		Data struct {
			Result []struct {
				Metric map[string]string `json:"metric"`
				Value  []any             `json:"value"`
			} `json:"result"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&pr); err != nil {
		return nil, err
	}
	out := make([]promInstantSample, 0, len(pr.Data.Result))
	for _, r := range pr.Data.Result {
		if len(r.Value) < 2 {
			continue
		}
		str, ok := r.Value[1].(string)
		if !ok {
			continue
		}
		val, err := fmt.Sscanf(str, "%f", new(float64))
		_ = val
		if err != nil {
			continue
		}
		var f float64
		if _, err := fmt.Sscanf(str, "%f", &f); err != nil || math.IsNaN(f) {
			continue
		}
		out = append(out, promInstantSample{labels: r.Metric, value: f})
	}
	return out, nil
}

// autoscaleDesired computes the desired replica count for an App based on
// current pod utilization vs the declared targets. Returns (desired, skip):
//   - desired is the clamped replica count [min, max]
//   - skip is true when metrics are unavailable and no action should be taken
//
// The algorithm mirrors HPA: for each enabled metric, desired = ceil(actual /
// target) * current. The maximum across all enabled metrics is used so the
// replica count satisfies all constraints simultaneously.
func autoscaleDesired(as *appv1alpha1.AutoscalingSpec, usage []PodUsage, tier string) (int32, bool) {
	if as == nil || !as.Enabled || len(usage) == 0 {
		return 0, true // no metrics or not enabled — skip
	}

	minR := max(as.MinReplicas, 1)
	maxR := max(as.MaxReplicas, minR)

	current := int32(len(usage))

	// Sum utilization across all pods.
	var totalCPU, totalMem float64
	for _, u := range usage {
		totalCPU += u.CPUCores
		totalMem += u.MemoryBytes
	}
	avgCPU := totalCPU / float64(current)
	avgMem := totalMem / float64(current)

	// Resolve tier resources so we can compute utilization as a %.
	cpuLimit, memLimit := tierLimits(tier)

	// desired is the maximum across all evaluated metrics (HPA algorithm).
	// Starting at 0 lets scale-down be computed; anyEvaluated guards the
	// best-effort-tier "no limits → hold steady" path.
	desired := int32(0)
	anyEvaluated := false
	if as.TargetCPUPercent != nil && cpuLimit > 0 {
		targetCPU := float64(*as.TargetCPUPercent) / 100.0 * cpuLimit
		cpuDesired := int32(math.Ceil(float64(current) * (avgCPU / targetCPU)))
		if cpuDesired > desired {
			desired = cpuDesired
		}
		anyEvaluated = true
	}
	if as.TargetMemoryPercent != nil && memLimit > 0 {
		targetMem := float64(*as.TargetMemoryPercent) / 100.0 * memLimit
		memDesired := int32(math.Ceil(float64(current) * (avgMem / targetMem)))
		if memDesired > desired {
			desired = memDesired
		}
		anyEvaluated = true
	}

	// No metric could be evaluated (best-effort tier has no limits) — hold
	// steady rather than blindly driving replicas to 0.
	if !anyEvaluated {
		return current, true
	}

	// Clamp.
	if desired < minR {
		desired = minR
	}
	if desired > maxR {
		desired = maxR
	}
	return desired, false
}

// tierLimits returns the CPU (cores) and memory (bytes) limits for a tier,
// returning 0 for both on a best-effort tier (no resource constraints).
func tierLimits(tier string) (cpuCores float64, memBytes float64) {
	cpuStr, memStr, ok := tiers.Compute.Resources(tier)
	if !ok {
		return 0, 0
	}
	if q, err := resource.ParseQuantity(cpuStr); err == nil {
		cpuCores = q.AsApproximateFloat64()
	}
	if q, err := resource.ParseQuantity(memStr); err == nil {
		memBytes = float64(q.Value())
	}
	return cpuCores, memBytes
}

// applyAutoscaling runs the autoscaling decision and, if action is needed,
// updates the App's spec.replicas. It respects the scale-down stabilization
// window: the annotation annotAutoscaleScaleDown is stamped when a downward
// decision is first made; the replica count is only lowered after the window
// has elapsed. Returns the replica count to use and whether to requeue.
func (r *AppReconciler) applyAutoscaling(ctx context.Context, app *appv1alpha1.App, current int32) (desired int32, requeue bool) {
	if r.MetricsReader == nil {
		return current, false
	}
	as := app.Spec.Autoscaling
	if as == nil || !as.Enabled {
		return current, false
	}

	usage, err := r.MetricsReader(ctx, app.Namespace, app.Name)
	if err != nil {
		// Metrics unavailable — hold steady, requeue to retry.
		return current, true
	}

	want, skip := autoscaleDesired(as, usage, app.Spec.Tier)
	if skip {
		return current, true
	}

	now := time.Now().UTC()

	if want < current {
		// Scale-down: respect the stabilization window.
		stamped := app.Annotations[annotAutoscaleScaleDown]
		if stamped == "" {
			// First downward signal — stamp now and hold.
			base := app.DeepCopy()
			if app.Annotations == nil {
				app.Annotations = map[string]string{}
			}
			app.Annotations[annotAutoscaleScaleDown] = now.Format(time.RFC3339)
			_ = r.Patch(ctx, app, client.MergeFrom(base))
			return current, true
		}
		t, err := time.Parse(time.RFC3339, stamped)
		if err != nil || now.Sub(t) < scaleDownStabilizationWindow {
			return current, true // window not yet elapsed — hold
		}
		// Clear the annotation now that we're committing the downscale.
		base := app.DeepCopy()
		delete(app.Annotations, annotAutoscaleScaleDown)
		_ = r.Patch(ctx, app, client.MergeFrom(base))
	} else if want > current {
		// Scale-up: clear any pending scale-down annotation.
		if app.Annotations[annotAutoscaleScaleDown] != "" {
			base := app.DeepCopy()
			delete(app.Annotations, annotAutoscaleScaleDown)
			_ = r.Patch(ctx, app, client.MergeFrom(base))
		}
	}

	// Persist the desired count in an annotation, not spec.replicas.
	// Writing spec.replicas would bump metadata.generation, which causes
	// git-backed Apps to rebuild on every autoscaler tick (annotAutoscaleReplicas
	// explains the incident). The caller reads this annotation to seed `current`
	// on the next reconcile pass so a metrics-failure pass doesn't revert to
	// spec.replicas (the user's static count).
	if strconv.Itoa(int(want)) != app.Annotations[annotAutoscaleReplicas] {
		base := app.DeepCopy()
		if app.Annotations == nil {
			app.Annotations = map[string]string{}
		}
		app.Annotations[annotAutoscaleReplicas] = strconv.Itoa(int(want))
		if err := r.Patch(ctx, app, client.MergeFrom(base)); err != nil {
			return current, true
		}
	}

	return want, true // always requeue while autoscaling is on
}
