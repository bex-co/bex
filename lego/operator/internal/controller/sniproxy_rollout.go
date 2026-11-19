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

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	extensionsv1beta1 "k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// NewPodsReadyAnnotation is what Argo CD's DaemonSet health check reads
// (deploy/argocd/daemonset-health.lua, w6/m518). The value "<generation>/<nodes>"
// says that every one of <nodes> nodes runs a Ready pod of that generation's
// template that is not terminating. The check reads a DaemonSet Healthy while
// the value matches its generation and desiredNumberScheduled, so an old pod
// that still drains does not keep it Progressing. A stale value never matches
// a later generation or node count.
const NewPodsReadyAnnotation = "app.bex.co/new-pods-ready"

// sniProxyDaemonSets are the DaemonSets whose old pods relay their open
// external streams after SIGTERM (w6/m515), and so stay on their node after
// the new pod is Ready.
var sniProxyDaemonSets = []string{"bex-kv-sni-proxy", "bex-pg-sni-proxy"}

// SNIProxyRolloutReconciler keeps NewPodsReadyAnnotation on the SNI proxy
// DaemonSets (w6/m518). The DaemonSet controller's status reads each node's
// oldest pod, terminating ones included, so it reports a roll unfinished
// until every old pod has drained. Argo CD's built-in health check reads that
// status, so a long drain would hold the bex-operator Application Progressing.
// The annotation records the same per-node rule scripts/operator-pins.sh
// waits for, which the custom health check can then read.
type SNIProxyRolloutReconciler struct {
	client.Client
	// Namespace is where the SNI proxy DaemonSets run: the operator's own.
	Namespace string
}

// +kubebuilder:rbac:groups=apps,resources=daemonsets,verbs=get;list;watch
// +kubebuilder:rbac:groups=apps,resources=daemonsets,verbs=patch,resourceNames=bex-kv-sni-proxy;bex-pg-sni-proxy

func (r *SNIProxyRolloutReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var ds appsv1.DaemonSet
	if err := r.Get(ctx, req.NamespacedName, &ds); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	selector, err := metav1.LabelSelectorAsSelector(ds.Spec.Selector)
	if err != nil {
		return ctrl.Result{}, reconcile.TerminalError(fmt.Errorf("daemonset %s selector: %w", req.NamespacedName, err))
	}
	var pods corev1.PodList
	if err := r.List(ctx, &pods, client.InNamespace(ds.Namespace), client.MatchingLabelsSelector{Selector: selector}); err != nil {
		return ctrl.Result{}, err
	}
	want := ""
	if newPodsReady(&ds, pods.Items) {
		want = fmt.Sprintf("%d/%d", ds.Generation, ds.Status.DesiredNumberScheduled)
	}
	got, has := ds.Annotations[NewPodsReadyAnnotation]
	if (want == "" && !has) || (want != "" && got == want) {
		return ctrl.Result{}, nil
	}
	patch := client.MergeFrom(ds.DeepCopy())
	if want == "" {
		delete(ds.Annotations, NewPodsReadyAnnotation)
	} else {
		if ds.Annotations == nil {
			ds.Annotations = map[string]string{}
		}
		ds.Annotations[NewPodsReadyAnnotation] = want
	}
	return ctrl.Result{}, r.Patch(ctx, &ds, patch)
}

// newPodsReady reports whether the DaemonSet controller has observed ds's
// generation and each node it should run on (desiredNumberScheduled) has a
// Ready pod of the current template that ds controls and that is not
// terminating. Pods that still drain count for nothing, either way.
func newPodsReady(ds *appsv1.DaemonSet, pods []corev1.Pod) bool {
	// The API server counts template changes in this annotation, and the
	// DaemonSet controller labels each pod it creates with the count.
	template := ds.Annotations[appsv1.DeprecatedTemplateGeneration]
	if ds.Status.ObservedGeneration < ds.Generation || template == "" {
		return false
	}
	nodes := map[string]bool{}
	for i := range pods {
		pod := &pods[i]
		owner := metav1.GetControllerOf(pod)
		if owner == nil || owner.UID != ds.UID || pod.DeletionTimestamp != nil ||
			pod.Labels[extensionsv1beta1.DaemonSetTemplateGenerationKey] != template || pod.Spec.NodeName == "" || !podReady(pod) {
			continue
		}
		nodes[pod.Spec.NodeName] = true
	}
	return int32(len(nodes)) >= ds.Status.DesiredNumberScheduled
}

// watches reports whether namespace/name is one of the SNI proxy DaemonSets.
func (r *SNIProxyRolloutReconciler) watches(namespace, name string) bool {
	return namespace == r.Namespace && slices.Contains(sniProxyDaemonSets, name)
}

// SetupWithManager watches the two SNI proxy DaemonSets and the pods they
// control, so a pod turning Ready, a pod starting to terminate and a node
// joining each re-evaluate the annotation.
func (r *SNIProxyRolloutReconciler) SetupWithManager(mgr ctrl.Manager) error {
	daemonSets := predicate.NewPredicateFuncs(func(o client.Object) bool {
		return r.watches(o.GetNamespace(), o.GetName())
	})
	pods := predicate.NewPredicateFuncs(func(o client.Object) bool {
		owner := metav1.GetControllerOfNoCopy(o)
		return owner != nil && owner.Kind == "DaemonSet" && r.watches(o.GetNamespace(), owner.Name)
	})
	return ctrl.NewControllerManagedBy(mgr).
		Named("sniproxyrollout").
		For(&appsv1.DaemonSet{}, builder.WithPredicates(daemonSets)).
		Watches(&corev1.Pod{},
			handler.EnqueueRequestForOwner(mgr.GetScheme(), mgr.GetRESTMapper(), &appsv1.DaemonSet{}, handler.OnlyControllerOwner()),
			builder.WithPredicates(pods)).
		Complete(r)
}
