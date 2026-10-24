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

	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

const labelEnvironment = "app.bex.co/environment-id"

// reconcileEnvironmentPeerPolicy gives a datastore in an isolated environment
// (env = its labelNetworkIsolation, "" otherwise) the only private ingress it
// accepts: protected App pods of the same immutable environment, plus the
// store's own Jobs (jobs: backup, purge, logical export), which carry no
// isolation label. The namespace-wide allow-same-namespace policy excludes pods
// carrying labelNetworkIsolation, so an isolated store admits nothing else from
// the workspace (w8/m136); platform namespaces (SNI proxies, CNPG, Prometheus)
// keep their own namespace-scoped allows. Not isolated: the policy is removed.
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
		peers := []networkingv1.NetworkPolicyPeer{{
			PodSelector: &metav1.LabelSelector{MatchLabels: map[string]string{labelNetworkIsolation: env}},
		}}
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

// reconcileProtectedAppNetworkPolicy narrows a protected App's private
// connectivity to peers carrying the same immutable environment identity.
// Two peer selectors are intentionally used: Apps carry network-isolation,
// while Database/KeyValue pods carry the unconditional environment-id label.
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
		peers := []networkingv1.NetworkPolicyPeer{
			{PodSelector: &metav1.LabelSelector{MatchLabels: map[string]string{labelNetworkIsolation: env}}},
			{PodSelector: &metav1.LabelSelector{MatchLabels: map[string]string{labelEnvironment: env}}},
		}
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
