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
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"

	boundedhttp "github.com/bex-co/bex/lego/operator/internal/httpclient"
	"github.com/bex-co/bex/lego/types/podmetrics"
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

// metricsFailureLogInterval is how often a still-failing metrics read is logged
// again for one App after its first failure is logged (w4/m364).
const metricsFailureLogInterval = 10 * time.Minute

// autoscalingMetricsUnavailableMessage is the tenant-safe message of the
// AutoscalingMetricsAvailable condition. The read error itself is logged.
const autoscalingMetricsUnavailableMessage = "Autoscaling is paused: utilization metrics are unavailable, so the instance count is held"

// autoscalingMetricsReadFailures counts failed utilization reads per App
// (w4/m364). The series exist only for autoscaled Apps whose read failed, and
// go with the App (forgetMetricsFailures). It feeds the
// AutoscalingMetricsUnavailable alert.
var autoscalingMetricsReadFailures = prometheus.NewCounterVec(prometheus.CounterOpts{
	Name: "bex_autoscaling_metrics_read_failures_total",
	Help: "Failed metrics-server utilization reads by the autoscaler, by App. While an App's reads fail its autoscaler holds the instance count.",
}, []string{"namespace", "app"})

func init() {
	ctrlmetrics.Registry.MustRegister(autoscalingMetricsReadFailures)
}

// PodUsage is one pod's current resource consumption, as reported by metrics-server.
// It is the shared decoder's record so autoscaling and bex-api's reported usage
// read the same payload identically.
type PodUsage = podmetrics.Usage

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
		return podmetrics.Parse(raw)
	}
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
	if _, recorded := recordedAutoscaleCount(app); !recorded {
		current = r.seedAutoscaler(ctx, app, current)
	}
	// Hold one from/to edge stable until the Deployment reaches its recorded
	// target. This prevents a metrics wobble from overwriting a Started status
	// before the backend can persist its matching Ended fact.
	if transition := app.Status.Autoscaling; transition != nil && transition.State == appv1alpha1.AutoscalingTransitionStarted {
		return transition.ToReplicas, true
	}

	// A held release's new pods are all up beside the serving ones and take
	// no traffic; counting their start-up usage would scale the service in
	// the middle of its deploy (w4/571).
	if webImageReleaseHeld(app) {
		return current, true
	}
	usage, err := r.MetricsReader(ctx, app.Namespace, app.Name)
	if err != nil {
		// Metrics unavailable — hold steady, requeue to retry, and say so.
		r.metricsReadFailed(ctx, app, err)
		return current, true
	}
	r.metricsReadRecovered(ctx, app)

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
		startAutoscalingTransition(app, current, want, now)
	}

	return want, true // always requeue while autoscaling is on
}

// startAutoscalingTransition records the autoscaler moving app from one
// replica count to another, which the backend reports as autoscaling_started
// and, once the Deployment is there, autoscaling_ended.
func startAutoscalingTransition(app *appv1alpha1.App, from, to int32, now time.Time) {
	app.Status.Autoscaling = &appv1alpha1.AutoscalingStatus{
		TransitionID: fmt.Sprintf("%s-%d-%d", now.Format(time.RFC3339Nano), from, to),
		FromReplicas: from,
		ToReplicas:   to,
		State:        appv1alpha1.AutoscalingTransitionStarted,
		StartedAt:    now.Format(time.RFC3339Nano),
	}
}

// recordedAutoscaleCount is the replica count the autoscaler last recorded on
// app, if it recorded a usable one.
func recordedAutoscaleCount(app *appv1alpha1.App) (int32, bool) {
	n, err := strconv.ParseInt(app.Annotations[annotAutoscaleReplicas], 10, 32)
	if err != nil || n <= 0 {
		return 0, false
	}
	return int32(n), true
}

// autoscaleStart is the replica count an autoscaled pass starts from: the
// count the autoscaler last recorded, else the count the service runs now (its
// Deployment's), else its own configured count.
func autoscaleStart(app *appv1alpha1.App, configured int32, dep *appsv1.Deployment) int32 {
	if n, ok := recordedAutoscaleCount(app); ok {
		return n
	}
	if dep != nil && dep.Spec.Replicas != nil && *dep.Spec.Replicas > 0 {
		return *dep.Spec.Replicas
	}
	return configured
}

// seedAutoscaler gives the autoscaler a recorded count on its first pass:
// running, clamped to the autoscaling range (w4/m364). From then on the
// autoscaler owns the running count, and spec.replicas is only the stored
// manual count the service returns to when autoscaling is turned off, as
// Render ignores a manual scale while autoscaling is on. Before, the first
// pass had no record until the first decision, so a manual scale inside the
// first scale-down window still changed the running count. The seed is also
// held to the plan's instance cap, as the Deployment will be: a transition to a
// count it can never reach would never end, and its hold would stop the
// autoscaler. A count the clamps move is a scaling the autoscaler made, so it
// records a transition, unless one is already under way, whose end the
// backend still has to see. A failed write records nothing; the next pass
// seeds again from what then runs.
func (r *AppReconciler) seedAutoscaler(ctx context.Context, app *appv1alpha1.App, running int32) int32 {
	minR, maxR := autoscaleBounds(app.Spec.Autoscaling)
	seed := clampReplicas(app, min(max(running, minR), maxR))
	if err := r.patchAppMeta(ctx, app, func(meta *metav1.ObjectMeta) {
		metav1.SetMetaDataAnnotation(meta, annotAutoscaleReplicas, strconv.Itoa(int(seed)))
	}); err != nil {
		return running
	}
	if transition := app.Status.Autoscaling; seed != running &&
		(transition == nil || transition.State != appv1alpha1.AutoscalingTransitionStarted) {
		startAutoscalingTransition(app, running, seed, time.Now().UTC())
	}
	return seed
}

// metricsReadFailed surfaces a failed utilization read (w4/m364): it counts
// it, sets the AutoscalingMetricsAvailable condition and logs the error with
// the App, at once on the first failure and then at most once per
// metricsFailureLogInterval while reads keep failing.
func (r *AppReconciler) metricsReadFailed(ctx context.Context, app *appv1alpha1.App, err error) {
	autoscalingMetricsReadFailures.WithLabelValues(app.Namespace, app.Name).Inc()
	changed := meta.SetStatusCondition(&app.Status.Conditions, metav1.Condition{
		Type:               appv1alpha1.ConditionAutoscalingMetricsAvailable,
		Status:             metav1.ConditionFalse,
		Reason:             appv1alpha1.AutoscalingMetricsUnavailableReason,
		Message:            autoscalingMetricsUnavailableMessage,
		ObservedGeneration: app.Generation,
	})
	key := app.Namespace + "/" + app.Name
	now := time.Now()
	if last, ok := r.metricsFailureLogged.Load(key); !changed && ok && now.Sub(last.(time.Time)) < metricsFailureLogInterval {
		return
	}
	r.metricsFailureLogged.Store(key, now)
	logf.FromContext(ctx).Error(err, "autoscaling metrics read failed; holding the instance count",
		"app", app.Name, "namespace", app.Namespace)
}

// forgetMetricsFailures drops a deleted App's failed-read log limit and its
// failure series, so neither outlives the App.
func (r *AppReconciler) forgetMetricsFailures(namespace, name string) {
	r.metricsFailureLogged.Delete(namespace + "/" + name)
	autoscalingMetricsReadFailures.DeleteLabelValues(namespace, name)
}

// metricsReadRecovered clears what metricsReadFailed set once a read succeeds.
func (r *AppReconciler) metricsReadRecovered(ctx context.Context, app *appv1alpha1.App) {
	r.metricsFailureLogged.Delete(app.Namespace + "/" + app.Name)
	if meta.RemoveStatusCondition(&app.Status.Conditions, appv1alpha1.ConditionAutoscalingMetricsAvailable) {
		logf.FromContext(ctx).Info("autoscaling metrics read recovered", "app", app.Name, "namespace", app.Namespace)
	}
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
	// A pass that reads no metrics has no failing read to report.
	meta.RemoveStatusCondition(&app.Status.Conditions, appv1alpha1.ConditionAutoscalingMetricsAvailable)
	r.metricsFailureLogged.Delete(app.Namespace + "/" + app.Name)
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
func clearScaleDownWindow(om *metav1.ObjectMeta) {
	delete(om.Annotations, annotAutoscaleScaleDown)
	delete(om.Annotations, annotAutoscaleScaleDownTo)
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
