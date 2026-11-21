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
	"slices"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/operator/internal/drainguard"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// reconcilePlatformRedeploy mirrors a drain-guard handover on dep into
// app.Status.PlatformRedeploy so the backend can project Render's
// zero_downtime_redeploy_started and zero_downtime_redeploy_ended facts
// (w4/m504). A Recreate Deployment (persistent disk) cannot overlap pods and
// never records a zero-downtime redeploy.
func (r *AppReconciler) reconcilePlatformRedeploy(ctx context.Context, app *appv1alpha1.App, dep *appsv1.Deployment, now time.Time) {
	if dep == nil || dep.Spec.Strategy.Type == appsv1.RecreateDeploymentStrategyType {
		return
	}
	evacuatedFrom := dep.Spec.Template.Annotations[drainguard.EvacuatedFromAnnotation]
	if evacuatedFrom != "" && specReplicas(dep) > 0 &&
		(app.Status.PlatformRedeploy == nil || app.Status.PlatformRedeploy.Node != evacuatedFrom) {
		app.Status.PlatformRedeploy = &appv1alpha1.PlatformRedeployStatus{
			TransitionID: fmt.Sprintf("%s-%d", evacuatedFrom, dep.Generation),
			Node:         evacuatedFrom,
			Generation:   dep.Generation,
			Trigger:      r.platformRedeployTrigger(ctx, evacuatedFrom),
			State:        appv1alpha1.PlatformRedeployStateStarted,
			StartedAt:    now.UTC().Format(time.RFC3339Nano),
		}
	}
	tr := app.Status.PlatformRedeploy
	if tr == nil || tr.State != appv1alpha1.PlatformRedeployStateStarted {
		return
	}
	if r.platformRedeployComplete(ctx, dep, tr) {
		tr.State = appv1alpha1.PlatformRedeployStateEnded
		tr.FinishedAt = now.UTC().Format(time.RFC3339Nano)
	}
}

// platformRedeployTrigger reads nodeName's taints to distinguish a Cluster
// Autoscaler scale-down from a CAPI Machine node replacement. Any read error
// (including NotFound when the node is already gone) falls back to
// node_replacement.
func (r *AppReconciler) platformRedeployTrigger(ctx context.Context, nodeName string) string {
	var node corev1.Node
	if err := r.Get(ctx, client.ObjectKey{Name: nodeName}, &node); err == nil {
		if slices.ContainsFunc(node.Spec.Taints, func(t corev1.Taint) bool { return t.Key == drainguard.ScaleDownTaint }) {
			return appv1alpha1.PlatformRedeployTriggerNodeConsolidation
		}
	}
	return appv1alpha1.PlatformRedeployTriggerNodeReplacement
}

// platformRedeployComplete reports when the handover transition has finished:
// either the rollout of tr.Generation completed with all desired replicas
// updated and available (no old ReplicaSet pods), or the old pod left tr.Node
// because drain-guard's hold expired.
func (r *AppReconciler) platformRedeployComplete(ctx context.Context, dep *appsv1.Deployment, tr *appv1alpha1.PlatformRedeployStatus) bool {
	if dep.Status.ObservedGeneration < tr.Generation {
		return false
	}
	want := specReplicas(dep)
	if deploymentRolloutReady(dep, want) && r.deploymentPodsReady(ctx, dep, want) {
		return true
	}
	return r.evacuatedNodeClear(ctx, dep, tr.Node)
}

// evacuatedNodeClear reports whether no non-terminal ReplicaSet pod of dep
// remains on nodeName. In a pod-less envtest where dep.Status.Replicas > 0 and
// no Pod objects exist, it returns false so the rollout-status check decides.
func (r *AppReconciler) evacuatedNodeClear(ctx context.Context, dep *appsv1.Deployment, nodeName string) bool {
	if dep.Spec.Selector == nil || len(dep.Spec.Selector.MatchLabels) == 0 {
		return false
	}
	var pods corev1.PodList
	if err := r.List(ctx, &pods, client.InNamespace(dep.Namespace),
		client.MatchingLabels(dep.Spec.Selector.MatchLabels)); err != nil {
		return false
	}
	rsPods := 0
	for i := range pods.Items {
		pod := &pods.Items[i]
		owner := metav1.GetControllerOf(pod)
		if owner == nil || owner.Kind != kindReplicaSet {
			continue
		}
		rsPods++
		if pod.Spec.NodeName == nodeName && pod.Status.Phase != corev1.PodSucceeded && pod.Status.Phase != corev1.PodFailed {
			return false
		}
	}
	if rsPods == 0 {
		return dep.Status.Replicas == 0
	}
	return true
}

func platformRedeployInProgress(app *appv1alpha1.App) bool {
	return app.Status.PlatformRedeploy != nil && app.Status.PlatformRedeploy.State == appv1alpha1.PlatformRedeployStateStarted
}
