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

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/bex-co/bex/lego/operator/internal/execution"
	"github.com/bex-co/bex/lego/operator/internal/predeploy"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

const labelEnvironment = execution.LabelEnvironment

// reconcileEnvironmentPeerPolicy gives a datastore in an isolated environment
// (env = its labelNetworkIsolation, "" otherwise) the only private ingress it
// accepts: pods of the same immutable environment, plus the store's own Jobs
// (jobs: backup, purge, logical export), which carry no environment label.
// The namespace-wide allow-same-namespace policy excludes an isolated
// environment's pods, so an isolated store admits nothing else from the
// workspace (w8/m136); platform namespaces (SNI proxies, CNPG, Prometheus)
// keep their own namespace-scoped allows. Not isolated: the policy is removed.
// The environment's own pods are admitted by environment-id as well as by the
// isolation label: a pod born from a template that took environment-id
// carries the isolation label only once relabelAppPods reaches it (w4/m342).
func reconcileEnvironmentPeerPolicy(ctx context.Context, cl client.Client, scheme *runtime.Scheme, owner client.Object, env, suffix string, selector map[string]string, jobs ...*metav1.LabelSelector) error {
	name := owner.GetName() + suffix
	np := &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: owner.GetNamespace()}}
	if env == "" {
		if err := cl.Get(ctx, client.ObjectKeyFromObject(np), np); err != nil {
			return client.IgnoreNotFound(err)
		}
		legacyForApp := np.Spec.PodSelector.MatchLabels[labelApp] == owner.GetName()
		for _, ref := range np.GetOwnerReferences() {
			if ref.UID == owner.GetUID() && ref.Controller != nil && *ref.Controller {
				legacyForApp = true
				break
			}
		}
		if legacyForApp {
			if err := cl.Delete(ctx, np); err != nil && !apierrors.IsNotFound(err) {
				return err
			}
		}
		return nil
	}
	_, err := controllerutil.CreateOrUpdate(ctx, cl, np, func() error {
		np.Labels = map[string]string{"app.bex.co/managed-by": "bex-operator"}
		peers := environmentPeers(env)
		for _, job := range jobs {
			peers = append(peers, networkingv1.NetworkPolicyPeer{PodSelector: job})
		}
		np.Spec = networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{MatchLabels: selector},
			PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeIngress},
			Ingress:     []networkingv1.NetworkPolicyIngressRule{{From: peers}},
		}
		return controllerutil.SetControllerReference(owner, np, scheme)
	})
	return err
}

// environmentPeers selects env's pods: by environment-id, which every App,
// Postgres and Key Value pod of the environment carries, and by the isolation
// label, which a pod from before w4/m342 may carry alone.
func environmentPeers(env string) []networkingv1.NetworkPolicyPeer {
	return []networkingv1.NetworkPolicyPeer{
		{PodSelector: &metav1.LabelSelector{MatchLabels: map[string]string{labelNetworkIsolation: env}}},
		{PodSelector: &metav1.LabelSelector{MatchLabels: map[string]string{labelEnvironment: env}}},
	}
}

// appTemplateEnvironmentLabels are the in-place labels an App's pod templates
// should carry: its environment id, and never the isolation label, which an
// environment's toggle flips (w4/m342).
func appTemplateEnvironmentLabels(app *appv1alpha1.App) map[string]string {
	return map[string]string{labelEnvironment: app.Labels[labelEnvironment]}
}

// relabelAppPods moves the App's running pods to the environment labels its
// CR carries, in place: environment-id, which the workspace's isolation
// boundary selects by (bex-api's allow-same-namespace and per-environment
// policies), and the isolation label, which the per-store policies and the
// pre-w4/m342 allow-same-namespace still read. Neither ever rolls a pod: the
// Deployment template holds them lazily (holdIsolationLabel), so an isolation
// toggle or a move restarts nothing, and existing pods gain environment-id
// here without a fleet-wide rollout. Serving (ReplicaSet), cron run and
// pre-deploy pods follow; build and backup pods keep their own labels.
func (r *AppReconciler) relabelAppPods(ctx context.Context, app *appv1alpha1.App) error {
	return relabelPodsInPlace(ctx, r.Client, app.Namespace, client.MatchingLabels{labelApp: app.Name}, app.Labels, func(pod *corev1.Pod, owner *metav1.OwnerReference) bool {
		switch owner.Kind {
		case "ReplicaSet":
			return true
		case "Job":
			component := pod.Labels[execution.LabelComponent]
			return component == "" || component == predeploy.ComponentValue
		}
		return false
	})
}

// reconcileProtectedAppNetworkPolicy narrows a protected App's private
// connectivity to peers carrying the same immutable environment identity.
// bex-api's per-environment policies carry the boundary since w4/m342; this
// per-App twin stays while an operator and a bex-api of different releases
// may run side by side.
func reconcileProtectedAppNetworkPolicy(ctx context.Context, cl client.Client, scheme *runtime.Scheme, app *appv1alpha1.App) error {
	np := &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: app.Name, Namespace: app.Namespace}}
	env := app.Labels[labelNetworkIsolation]
	if env == "" {
		if err := cl.Get(ctx, client.ObjectKeyFromObject(np), np); err != nil {
			return client.IgnoreNotFound(err)
		}
		legacyForApp := np.Spec.PodSelector.MatchLabels[labelApp] == app.Name
		for _, ref := range np.GetOwnerReferences() {
			if ref.UID == app.UID && ref.Controller != nil && *ref.Controller {
				legacyForApp = true
				break
			}
		}
		if legacyForApp {
			if err := cl.Delete(ctx, np); err != nil && !apierrors.IsNotFound(err) {
				return err
			}
		}
		return nil
	}
	_, err := controllerutil.CreateOrUpdate(ctx, cl, np, func() error {
		np.Labels = artifactLabels(app, "protected-network-policy")
		peers := environmentPeers(env)
		np.Spec = networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{MatchLabels: map[string]string{labelApp: app.Name}},
			PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeIngress, networkingv1.PolicyTypeEgress},
			Ingress:     []networkingv1.NetworkPolicyIngressRule{{From: peers}},
			Egress:      []networkingv1.NetworkPolicyEgressRule{{To: peers}},
		}
		return controllerutil.SetControllerReference(app, np, scheme)
	})
	return err
}
