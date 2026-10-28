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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

const labelKeyValueInternalProxy = "app.bex.co/keyvalue-internal-proxy"

// The front must not carry labelKeyValue: the credentialed backend Service uses
// that selector, and selecting the front too would route requests into a loop.
func keyValueInternalProxyPodLabels(kv *appv1alpha1.KeyValue) map[string]string {
	labels := map[string]string{labelKeyValueInternalProxy: kv.Name}
	for _, key := range []string{labelWorkspace, labelEnvironment, labelNetworkIsolation} {
		if value := kv.Labels[key]; value != "" {
			labels[key] = value
		}
	}
	return labels
}

// The internal front needs both halves of an isolated connection: ingress from
// the owning environment and egress to exactly its own credentialed backend.
// Datastores' ordinary environment policy is ingress-only, and the namespace's
// allow-same-namespace policy excludes isolated pods entirely. Keep these exact
// front/backend allows even while isolation is off so a metadata transition
// cannot strand an old front or backend pod before its labels converge.
func (r *KeyValueReconciler) reconcileKeyValueInternalProxyPolicy(ctx context.Context, kv *appv1alpha1.KeyValue) error {
	proxy := &metav1.LabelSelector{MatchLabels: map[string]string{labelKeyValueInternalProxy: kv.Name}}
	backend := &metav1.LabelSelector{MatchLabels: map[string]string{labelKeyValue: kv.Name}}
	clients := &metav1.LabelSelector{MatchExpressions: []metav1.LabelSelectorRequirement{{
		Key: labelNetworkIsolation, Operator: metav1.LabelSelectorOpDoesNotExist,
	}}}
	if env := kv.Labels[labelNetworkIsolation]; env != "" {
		clients = &metav1.LabelSelector{MatchLabels: map[string]string{labelNetworkIsolation: env}}
	}
	udp, tcp := corev1.ProtocolUDP, corev1.ProtocolTCP
	ports := []networkingv1.NetworkPolicyPort{
		{Protocol: &tcp, Port: new(intstr.FromInt(kvPort))},
		{Protocol: &tcp, Port: new(intstr.FromInt(kvTLSPort))},
	}
	policies := []struct {
		suffix string
		spec   networkingv1.NetworkPolicySpec
	}{
		{"-internal-proxy-environment-ingress", networkingv1.NetworkPolicySpec{
			PodSelector: *proxy,
			PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeIngress, networkingv1.PolicyTypeEgress},
			Ingress: []networkingv1.NetworkPolicyIngressRule{{
				From:  []networkingv1.NetworkPolicyPeer{{PodSelector: clients}, {PodSelector: keyValueJobPeer(kv)}},
				Ports: ports,
			}},
			Egress: []networkingv1.NetworkPolicyEgressRule{
				{To: []networkingv1.NetworkPolicyPeer{{PodSelector: backend}}, Ports: ports},
				// Legacy bootstrap namespaces need this as well: the policy's
				// Egress type isolates the front even without namespace-wide DNS allows.
				{To: []networkingv1.NetworkPolicyPeer{{NamespaceSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"kubernetes.io/metadata.name": "kube-system"}}}}, Ports: []networkingv1.NetworkPolicyPort{
					{Protocol: &udp, Port: new(intstr.FromInt(53))},
					{Protocol: &tcp, Port: new(intstr.FromInt(53))},
				}},
			},
		}},
		{"-internal-proxy-backend-ingress", networkingv1.NetworkPolicySpec{
			PodSelector: *backend,
			PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeIngress},
			Ingress:     []networkingv1.NetworkPolicyIngressRule{{From: []networkingv1.NetworkPolicyPeer{{PodSelector: proxy}}, Ports: ports}},
		}},
	}
	for _, policy := range policies {
		np := &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: kv.Name + policy.suffix, Namespace: kv.Namespace}}
		if _, err := controllerutil.CreateOrUpdate(ctx, r.Client, np, func() error {
			np.Labels = map[string]string{"app.bex.co/managed-by": "bex-operator"}
			np.Spec = policy.spec
			return controllerutil.SetControllerReference(kv, np, r.Scheme)
		}); err != nil {
			return err
		}
	}
	return nil
}
