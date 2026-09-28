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
		Reason: "ServingUnobserved", Message: "serving revision has not been observed", ObservedGeneration: app.Generation}
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
	}
}
