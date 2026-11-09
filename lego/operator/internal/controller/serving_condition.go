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

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// observeServingRevision keeps rollout diagnostics separate from evidence that
// the previously deployed revision is still serving. A failed list is unknown,
// never a healthy observation retained from a previous reconcile.
func (r *AppReconciler) observeServingRevision(ctx context.Context, app *appv1alpha1.App, dep *appsv1.Deployment, replicas int32) {
	condition := metav1.Condition{Type: appv1alpha1.ConditionServing, Status: metav1.ConditionUnknown,
		Reason: appv1alpha1.ReasonServingUnobserved, Message: "serving revision has not been observed", ObservedGeneration: app.Generation}
	defer func() { meta.SetStatusCondition(&app.Status.Conditions, condition) }()
	if app.Status.ActiveRevision == "" || replicas <= 0 || dep.Spec.Selector == nil || len(dep.Spec.Selector.MatchLabels) == 0 {
		return
	}
	var pods corev1.PodList
	if err := r.List(ctx, &pods, client.InNamespace(dep.Namespace), client.MatchingLabels(dep.Spec.Selector.MatchLabels)); err != nil {
		return
	}
	var ready int32
	for i := range pods.Items {
		pod := &pods.Items[i]
		if pod.Labels[labelRevision] == app.Status.ActiveRevision && pod.DeletionTimestamp.IsZero() && podReady(pod) {
			ready++
		}
	}
	if ready > 0 {
		condition.Status = metav1.ConditionTrue
		condition.Reason = appv1alpha1.ReasonPriorReleaseServing
		condition.Message = "the active revision's pods remain ready during the rollout"
		// The Deployment's own revision is the active one: its release serves,
		// not a prior one. A first deploy always ends here (w2/078).
		if dep.Spec.Template.Labels[labelRevision] == app.Status.ActiveRevision {
			condition.Reason = appv1alpha1.ReasonReleaseServing
			condition.Message = "the active revision's pods are ready"
		}
		return
	}
	// Observed, and nothing serves: the transition dates the stop (w5/083).
	condition.Status = metav1.ConditionFalse
	condition.Reason = appv1alpha1.ReasonServingRevisionUnavailable
	condition.Message = "no pod of the active revision is ready"
}

// refreshServingWhileBuilding re-runs observeServingRevision against the App's
// Deployment while a build is in progress, reporting whether the Serving
// condition changed. An App with no active revision, or no Deployment yet, has
// nothing serving to observe.
func (r *AppReconciler) refreshServingWhileBuilding(ctx context.Context, app *appv1alpha1.App) bool {
	if app.Status.ActiveRevision == "" {
		return false
	}
	var dep appsv1.Deployment
	if err := r.Get(ctx, client.ObjectKey{Namespace: app.Namespace, Name: app.Name}, &dep); err != nil {
		return false
	}
	before := meta.FindStatusCondition(app.Status.Conditions, appv1alpha1.ConditionServing)
	var was metav1.Condition
	if before != nil {
		was = *before
	}
	r.observeServingRevision(ctx, app, &dep, specReplicas(&dep))
	now := meta.FindStatusCondition(app.Status.Conditions, appv1alpha1.ConditionServing)
	return before == nil || now.Status != was.Status || now.Reason != was.Reason || now.ObservedGeneration != was.ObservedGeneration
}

// specReplicas is the Deployment's desired replica count, defaulted to 1 as
// the API server does.
func specReplicas(dep *appsv1.Deployment) int32 {
	if dep.Spec.Replicas != nil {
		return *dep.Spec.Replicas
	}
	return 1
}
