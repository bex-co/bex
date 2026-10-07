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
	"net/url"
	"sort"
	"strconv"
	"time"

	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	boundedhttp "github.com/bex-co/bex/lego/operator/internal/httpclient"
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

// annotAutoscaleScaleDownTo records the highest replica count the autoscaler
// asked for while the scale-down window runs, which is what the window commits
// (w5/120): like HPA's stabilization, one deep reading at the end of a shallow
// dip does not scale past what the whole window supports.
const annotAutoscaleScaleDownTo = "app.bex.co/autoscale-scale-down-to"

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
	requestCtx, cancel := boundedhttp.WithTimeout(ctx)
	defer cancel()
	req, err := http.NewRequestWithContext(requestCtx, http.MethodGet,
		fmt.Sprintf("%s/api/v1/query?query=%s", base, url.QueryEscape(query)), nil)
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
	if err := boundedhttp.DecodeJSON(resp.Body, &pr); err != nil {
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
		f, err := strconv.ParseFloat(str, 64)
		if err != nil || math.IsNaN(f) {
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

	minR, maxR := autoscaleBounds(as)

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
	cpuStr, memStr, _, ok := tiers.Compute.Resources(tier)
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

// autoscaleBounds is the replica range autoscaling may choose from: the spec's
// min and max, at least one and at most appv1alpha1.MaxReplicas.
func autoscaleBounds(as *appv1alpha1.AutoscalingSpec) (minR, maxR int32) {
	minR = min(max(as.MinReplicas, 1), appv1alpha1.MaxReplicas)
	maxR = min(max(as.MaxReplicas, minR), appv1alpha1.MaxReplicas)
	return minR, maxR
}

// applyAutoscaling runs the autoscaling decision and, if action is needed,
// records the desired count in annotAutoscaleReplicas. It respects the
// scale-down stabilization window: a first downward decision stamps
// annotAutoscaleScaleDown, annotAutoscaleScaleDownTo tracks the highest count
// asked for, and that count is committed once the window elapses. Any other
// decision clears both (w5/109, w5/120). Returns the replica count to use and
// whether to requeue.
func (r *AppReconciler) applyAutoscaling(ctx context.Context, app *appv1alpha1.App, current int32) (desired int32, requeue bool) {
	if r.MetricsReader == nil {
		r.skipAutoscaling(ctx, app)
		return current, false
	}
	if !autoscalingEnabled(app) {
		return current, false
	}
	as := app.Spec.Autoscaling
	// Hold one from/to edge stable until the Deployment reaches its recorded
	// target. This prevents a metrics wobble from overwriting a Started status
	// before the backend can persist its matching Ended fact.
	if transition := app.Status.Autoscaling; transition != nil && transition.State == appv1alpha1.AutoscalingTransitionStarted {
		return transition.ToReplicas, true
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
		// Scale-down: respect the stabilization window. It holds a dip for its
		// whole length from the first low reading, then commits the highest
		// count any reading in it asked for (w5/120).
		started, err := time.Parse(time.RFC3339, app.Annotations[annotAutoscaleScaleDown])
		if err != nil {
			// First downward signal, or a stamp that cannot be read: stamp now
			// and hold.
			_ = r.patchAppMeta(ctx, app, func(meta *metav1.ObjectMeta) {
				metav1.SetMetaDataAnnotation(meta, annotAutoscaleScaleDown, now.Format(time.RFC3339))
				metav1.SetMetaDataAnnotation(meta, annotAutoscaleScaleDownTo, strconv.Itoa(int(want)))
			})
			return current, true
		}
		ceiling := want
		if n, err := strconv.ParseInt(app.Annotations[annotAutoscaleScaleDownTo], 10, 32); err == nil {
			ceiling = max(ceiling, int32(n))
		}
		if now.Sub(started) < scaleDownStabilizationWindow {
			// A failed write loses only this reading's rise, as a failed stamp
			// loses only this pass's hold.
			if recorded := strconv.Itoa(int(ceiling)); recorded != app.Annotations[annotAutoscaleScaleDownTo] {
				_ = r.patchAppMeta(ctx, app, func(meta *metav1.ObjectMeta) {
					metav1.SetMetaDataAnnotation(meta, annotAutoscaleScaleDownTo, recorded)
				})
			}
			return current, true // window not yet elapsed — hold
		}
		// The record predates any change to the bounds; this pass's reading
		// already honors them, so only a lowered max can bind it.
		_, maxR := autoscaleBounds(as)
		want = min(ceiling, current, maxR)
	}

	// Persist the desired count in an annotation, not spec.replicas.
	// Writing spec.replicas would bump metadata.generation, which causes
	// git-backed Apps to rebuild on every autoscaler tick (annotAutoscaleReplicas
	// explains the incident). The caller reads this annotation to seed `current`
	// on the next reconcile pass so a metrics-failure pass doesn't revert to
	// spec.replicas (the user's static count).
	if err := r.settleAutoscaler(ctx, app, strconv.Itoa(int(want))); err != nil {
		return current, true
	}
	if want != current {
		app.Status.Autoscaling = &appv1alpha1.AutoscalingStatus{
			TransitionID: fmt.Sprintf("%s-%d-%d", now.Format(time.RFC3339Nano), current, want),
			FromReplicas: current,
			ToReplicas:   want,
			State:        appv1alpha1.AutoscalingTransitionStarted,
			StartedAt:    now.Format(time.RFC3339Nano),
		}
	}

	return want, true // always requeue while autoscaling is on
}

// scaleDownPending reports whether a scale-down window is recorded on app.
func scaleDownPending(app *appv1alpha1.App) bool {
	return app.Annotations[annotAutoscaleScaleDown] != "" || app.Annotations[annotAutoscaleScaleDownTo] != ""
}

// skipAutoscaling settles the autoscaler's state on a pass it does not run
// (autoscaling turned off, metrics unwired, or the service suspended). A
// pending scale-down window always goes: it must hold a dip for its whole
// length on readings it saw, so a stamp from before a pause must not let one
// low reading after it commit (w5/120). Only autoscaling turned off also
// forgets the count the autoscaler last chose and ends a transition it started,
// so turning it back on starts from the service's own count (w5/130). A
// suspended or unwired service keeps both and resumes where it was.
func (r *AppReconciler) skipAutoscaling(ctx context.Context, app *appv1alpha1.App) {
	count := app.Annotations[annotAutoscaleReplicas]
	if !autoscalingEnabled(app) {
		count = ""
		endAutoscalingTransition(app, time.Now())
	}
	_ = r.settleAutoscaler(ctx, app, count)
}

// settleAutoscaler records count as the autoscaler's last decision ("" for
// none) and clears any scale-down window in the same write, so a failure cannot
// clear the window yet keep the old count, which would restart it. A pass with
// nothing to change writes nothing.
func (r *AppReconciler) settleAutoscaler(ctx context.Context, app *appv1alpha1.App, count string) error {
	if !scaleDownPending(app) && app.Annotations[annotAutoscaleReplicas] == count {
		return nil
	}
	return r.patchAppMeta(ctx, app, func(meta *metav1.ObjectMeta) {
		clearScaleDownWindow(meta)
		if count == "" {
			delete(meta.Annotations, annotAutoscaleReplicas)
		} else {
			metav1.SetMetaDataAnnotation(meta, annotAutoscaleReplicas, count)
		}
	})
}

// autoscalingEnabled reports whether app's autoscaling is turned on, whether
// or not it runs this pass.
func autoscalingEnabled(app *appv1alpha1.App) bool {
	return app.Spec.Autoscaling != nil && app.Spec.Autoscaling.Enabled
}

// clearScaleDownWindow removes a scale-down window's stamp and its record.
func clearScaleDownWindow(meta *metav1.ObjectMeta) {
	delete(meta.Annotations, annotAutoscaleScaleDown)
	delete(meta.Annotations, annotAutoscaleScaleDownTo)
}

func completeAutoscalingTransition(app *appv1alpha1.App, replicas, ready int32, now time.Time) {
	if transition := app.Status.Autoscaling; transition != nil && replicas == transition.ToReplicas && ready >= transition.ToReplicas {
		endAutoscalingTransition(app, now)
	}
}

// endAutoscalingTransition ends app's autoscaling transition if one is under
// way.
func endAutoscalingTransition(app *appv1alpha1.App, now time.Time) {
	transition := app.Status.Autoscaling
	if transition == nil || transition.State != appv1alpha1.AutoscalingTransitionStarted {
		return
	}
	transition.State = appv1alpha1.AutoscalingTransitionEnded
	transition.FinishedAt = now.UTC().Format(time.RFC3339Nano)
}
